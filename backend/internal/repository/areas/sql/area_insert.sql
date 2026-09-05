INSERT INTO areas (
    id,
    name,
    geometry,
    source,
    period_from,
    period_to,
    created_at,
    generation,
    shown_result_version,
    shown_job_id,
    active_job_id
)
VALUES ($1, $2, $3::jsonb, $4::jsonb, $5, $6, $7, $8, $9, $10, $11)
