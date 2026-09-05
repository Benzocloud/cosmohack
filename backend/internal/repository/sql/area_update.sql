UPDATE areas
SET name = $2,
    geometry = $3::jsonb,
    source = $4::jsonb,
    period_from = $5,
    period_to = $6,
    generation = $7,
    shown_result_version = $8,
    shown_job_id = $9,
    active_job_id = $10
WHERE id = $1
