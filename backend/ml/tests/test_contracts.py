from __future__ import annotations

from datetime import date, timedelta

import pytest
from pydantic import ValidationError

from vegetation_ml import contracts


def _request() -> dict:
    return {
        "schema_version": "1.0",
        "request_id": "job-contract-1",
        "area_id": "area-contract-1",
        "input_revision": "revision-contract-1",
        "mode": "retrospective",
        "feature_profile": "ndvi-weather-v1",
        "analysis_period": {"from": "2026-06-01", "to": "2026-06-01"},
        "sources": [],
        "observations": [{
            "date": "2026-06-01",
            "primary_ndvi": None,
            "quality": "missing",
            "ndvi_source_id": None,
            "interval": None,
            "valid_fraction": None,
            "missing_reason": "no_usable_observation",
            "weather": None,
            "reference": None,
        }],
    }


def test_production_profile_is_the_only_supported_contract():
    request = contracts.AnalyzeRequest.model_validate(_request())

    assert request.schema_version == contracts.SCHEMA_VERSION
    assert request.feature_profile == contracts.FEATURE_PROFILE
    assert contracts.PRODUCTION_SCHEMA_VERSIONS == (contracts.SCHEMA_VERSION,)
    contracts.check_contract(
        _request(), supported_versions=contracts.PRODUCTION_SCHEMA_VERSIONS)

    extended = _request()
    extended["schema_version"] = contracts.SCHEMA_VERSION_EXTENDED
    with pytest.raises(contracts.UnsupportedContract):
        contracts.check_contract(
            extended, supported_versions=contracts.PRODUCTION_SCHEMA_VERSIONS)


@pytest.mark.parametrize("field", ["request_id", "area_id", "input_revision"])
def test_identifiers_are_nonempty_strict_strings(field):
    request = _request()
    request[field] = 1
    with pytest.raises(ValidationError):
        contracts.AnalyzeRequest.model_validate(request)

    request[field] = ""
    with pytest.raises(ValidationError):
        contracts.AnalyzeRequest.model_validate(request)


def test_unknown_fields_are_rejected_at_each_schema_level():
    request = _request()
    request["unexpected"] = True
    with pytest.raises(ValidationError):
        contracts.AnalyzeRequest.model_validate(request)

    request = _request()
    request["observations"][0]["unexpected"] = True
    with pytest.raises(ValidationError):
        contracts.AnalyzeRequest.model_validate(request)

    request = _request()
    request["analysis_period"]["from_"] = request["analysis_period"].pop("from")
    with pytest.raises(ValidationError):
        contracts.AnalyzeRequest.model_validate(request)


@pytest.mark.parametrize("value", [float("nan"), float("inf"), float("-inf"), "0.5", True])
def test_numeric_fields_reject_nonfinite_and_non_numeric_values(value):
    request = _request()
    request["observations"][0]["weather"] = {
        "source_id": "wx-1",
        "temperature_mean_c": value,
        "precipitation_sum_mm": 0,
    }
    request["sources"] = [{
        "id": "wx-1",
        "kind": "weather",
        "provider": "open-meteo",
        "dataset": "era5",
        "mapping": "daily UTC aggregation",
        "retrieved_at": "2026-09-01T00:00:00Z",
        "license": None,
    }]
    with pytest.raises(ValidationError):
        contracts.AnalyzeRequest.model_validate(request)


def test_source_metadata_is_strict_and_utc():
    request = _request()
    request["sources"] = [{
        "id": "wx-1",
        "kind": "weather",
        "provider": "open-meteo",
        "dataset": "era5",
        "mapping": "daily UTC aggregation",
        "retrieved_at": "2026-09-01T03:00:00+03:00",
        "license": None,
    }]
    request["observations"][0]["weather"] = {
        "source_id": "wx-1",
        "temperature_mean_c": 20,
        "precipitation_sum_mm": 0,
    }
    with pytest.raises(ValidationError, match="UTC"):
        contracts.AnalyzeRequest.model_validate(request)

    request["sources"][0]["retrieved_at"] = "2026-09-01 00:00:00"
    with pytest.raises(ValidationError, match="RFC3339"):
        contracts.AnalyzeRequest.model_validate(request)

    request["sources"][0]["retrieved_at"] = "2026-09-01T00:00:00Z"
    assert contracts.AnalyzeRequest.model_validate(request).sources[0].mapping == (
        "daily UTC aggregation")


@pytest.mark.parametrize("value", ["2026-06-01T00:00:00Z", 20260601, "2026-2-1"])
def test_dates_require_wire_format(value):
    request = _request()
    request["analysis_period"]["from"] = value
    with pytest.raises(ValidationError, match="YYYY-MM-DD"):
        contracts.AnalyzeRequest.model_validate(request)


def test_source_mapping_does_not_accept_an_object():
    request = _request()
    request["sources"] = [{
        "id": "wx-1",
        "kind": "weather",
        "provider": "open-meteo",
        "dataset": "era5",
        "mapping": {"aggregation": "daily"},
        "retrieved_at": "2026-09-01T00:00:00Z",
        "license": None,
    }]
    with pytest.raises(ValidationError):
        contracts.AnalyzeRequest.model_validate(request)


def test_observation_limit_and_empty_observations():
    request = _request()
    request["observations"] = []
    with pytest.raises(ValidationError):
        contracts.AnalyzeRequest.model_validate(request)

    request = _request()
    start = date(2020, 1, 1)
    request["analysis_period"] = {
        "from": start.isoformat(),
        "to": (start + timedelta(days=contracts.MAX_OBSERVATIONS)).isoformat(),
    }
    request["observations"] = []
    for index in range(contracts.MAX_OBSERVATIONS + 1):
        current = start + timedelta(days=index)
        request["observations"].append({
            "date": current.isoformat(),
            "primary_ndvi": None,
            "quality": "missing",
            "ndvi_source_id": None,
            "interval": None,
            "valid_fraction": None,
            "missing_reason": "no_usable_observation",
            "weather": None,
            "reference": None,
        })
    with pytest.raises(ValidationError, match="4096"):
        contracts.AnalyzeRequest.model_validate(request)


def test_null_values_are_allowed_for_missing_observation():
    request = _request()
    request["observations"][0]["quality"] = None
    assert contracts.AnalyzeRequest.model_validate(request).observations[0].effective_quality == (
        "missing")
