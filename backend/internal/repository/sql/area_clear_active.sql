UPDATE areas
SET active_job_id = NULL
WHERE id = $1 AND active_job_id = $2
