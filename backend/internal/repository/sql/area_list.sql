SELECT
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
FROM areas
ORDER BY created_at ASC, id ASC
