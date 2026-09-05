UPDATE jobs
SET status = 'failed',
    stage = NULL,
    error_code = 'interrupted',
    error_message = 'server restarted; rerun analysis',
    error_retryable = TRUE,
    updated_at = $1
WHERE status IN ('queued', 'running')
