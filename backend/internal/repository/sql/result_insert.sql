INSERT INTO analysis_results (
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
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15::jsonb, $16::jsonb, $17::jsonb, $18::jsonb)
ON CONFLICT (area_id, result_version) DO NOTHING
