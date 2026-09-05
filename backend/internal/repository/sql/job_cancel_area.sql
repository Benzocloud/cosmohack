UPDATE jobs
SET status = 'cancelled', stage = NULL, updated_at = $2
WHERE area_id = $1 AND status IN ('queued', 'running')
