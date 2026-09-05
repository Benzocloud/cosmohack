UPDATE jobs
SET stage = NULLIF($2, ''), updated_at = $3
WHERE id = $1 AND status = 'running'
