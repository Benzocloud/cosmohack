UPDATE jobs
SET status = 'completed',
    stage = NULL,
    error_code = NULL,
    error_message = NULL,
    error_retryable = NULL,
    result_version = $2,
    updated_at = $3
WHERE id = $1 AND status = 'running'
