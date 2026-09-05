SELECT
    id,
    area_id,
    status,
    stage,
    period_from,
    period_to,
    error_code,
    error_message,
    error_retryable,
    result_version,
    area_generation,
    input_revision,
    created_at,
    updated_at
FROM jobs
WHERE area_id = $1
ORDER BY created_at ASC, id ASC
