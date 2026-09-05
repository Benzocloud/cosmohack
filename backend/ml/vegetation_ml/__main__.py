"""Запуск HTTP-сервиса: python -m vegetation_ml."""

from __future__ import annotations

import argparse

import uvicorn


def main() -> int:
    ap = argparse.ArgumentParser(prog="vegetation_ml", description="HTTP-сервис ML")
    ap.add_argument("--host", default="0.0.0.0")
    ap.add_argument("--port", type=int, default=8000)
    ap.add_argument("--log-level", default="info")
    args = ap.parse_args()
    uvicorn.run("vegetation_ml.service:app", host=args.host, port=args.port,
                log_level=args.log_level, workers=1)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
