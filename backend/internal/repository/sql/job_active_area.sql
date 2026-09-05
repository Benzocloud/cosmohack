SELECT id
FROM jobs
WHERE area_id = $1 AND status IN ('queued', 'running')
ORDER BY id
FOR UPDATE
