UPDATE areas AS a
SET active_job_id = NULL
WHERE a.active_job_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM jobs AS j
      WHERE j.id = a.active_job_id
        AND j.status IN ('queued', 'running')
  )
