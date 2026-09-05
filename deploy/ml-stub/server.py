import json
import os
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


SCHEMA_VERSION = "1.0"
FEATURE_PROFILE = "ndvi-weather-v1"
MODEL_VERSION = os.getenv("MODEL_VERSION", "dev-stub")


def json_response(handler, status, payload):
    body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    handler.send_response(status)
    handler.send_header("Content-Type", "application/json")
    handler.send_header("Content-Length", str(len(body)))
    handler.end_headers()
    handler.wfile.write(body)


def failure(mode):
    if mode == "timeout":
        time.sleep(float(os.getenv("STUB_TIMEOUT_SECONDS", "30")))
        return None
    if mode == "busy":
        return 429, {"schema_version": SCHEMA_VERSION, "request_id": None, "error": {"code": "busy", "message": "stub is busy"}}
    if mode == "invalid":
        return 200, {"schema_version": "invalid", "request_id": "stub"}
    return None


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("ml-stub: " + (fmt % args), flush=True)

    def do_GET(self):
        if self.path == "/readyz":
            json_response(
                self,
                200,
                {
                    "status": "ready",
                    "schema_version": SCHEMA_VERSION,
                    "feature_profiles": [FEATURE_PROFILE],
                    "model_version": MODEL_VERSION,
                    "reason": None,
                },
            )
            return
        json_response(self, 404, {"error": {"code": "not_found", "message": "route not found"}})

    def do_POST(self):
        if self.path.split("?", 1)[0] != "/v1/analyze":
            json_response(self, 404, {"error": {"code": "not_found", "message": "route not found"}})
            return
        mode = self.headers.get("X-Stub-Mode", os.getenv("STUB_MODE", ""))
        simulated = failure(mode)
        if simulated:
            status, payload = simulated
            json_response(self, status, payload)
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            request = json.loads(self.rfile.read(length))
        except (ValueError, json.JSONDecodeError):
            json_response(self, 400, {"schema_version": SCHEMA_VERSION, "request_id": None, "error": {"code": "invalid_json", "message": "request body is invalid"}})
            return

        observations = request.get("observations", [])
        series = []
        observed = 0
        period = request.get("analysis_period", {})
        for observation in observations:
            date = observation.get("date", "")
            if date < period.get("from", "") or date > period.get("to", ""):
                continue
            primary = observation.get("primary_ndvi")
            usable = observation.get("quality") == "usable" and primary is not None
            if usable:
                observed += 1
            point = {
                "date": date,
                "primary_ndvi": primary,
                "value": primary if usable else None,
                "state": "observed" if usable else "missing",
                "method": None,
                "baseline": None,
                "z_score": None,
                "interval": None,
                "valid_fraction": observation.get("valid_fraction"),
            }
            series.append(point)

        status = "normal" if observed else "insufficient_data"
        result = {
            "schema_version": SCHEMA_VERSION,
            "request_id": request.get("request_id"),
            "area_id": request.get("area_id"),
            "input_revision": request.get("input_revision"),
            "mode": request.get("mode"),
            "feature_profile": request.get("feature_profile"),
            "model_version": MODEL_VERSION,
            "method": "dev-ml-stub",
            "status": status,
            "severity": "none" if status == "normal" else None,
            "series": series,
            "events": [],
            "limitations": ["result produced by development ML stub"],
        }
        json_response(self, 200, result)


if __name__ == "__main__":
    port = int(os.getenv("PORT", "8000"))
    ThreadingHTTPServer(("0.0.0.0", port), Handler).serve_forever()
