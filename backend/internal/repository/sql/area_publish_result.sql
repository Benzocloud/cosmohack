UPDATE areas
SET shown_result_version = $2,
    shown_job_id = $3,
    active_job_id = NULL
WHERE id = $1 AND active_job_id = $3
