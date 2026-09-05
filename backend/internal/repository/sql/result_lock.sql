SELECT
    area_id,
    result_version,
    period_from,
    period_to,
    computed_at,
    schema_version,
    feature_profile,
    model_version,
    method,
    status,
    severity,
    input_revision,
    content_hash,
    series,
    weather,
    provenance,
    limitations,
    events
FROM analysis_results
WHERE area_id = $1 AND result_version = $2
FOR UPDATE
