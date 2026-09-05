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
WHERE id = $1
FOR UPDATE
