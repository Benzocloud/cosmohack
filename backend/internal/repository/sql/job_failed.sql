UPDATE jobs
SET status = 'failed',
    stage = NULL,
    error_code = $2,
    error_message = $3,
    error_retryable = $4,
    updated_at = $5
WHERE id = $1 AND status IN ('queued', 'running')
