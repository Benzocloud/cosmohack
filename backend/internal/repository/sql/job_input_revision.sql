UPDATE jobs
SET input_revision = NULLIF($2, ''), updated_at = $3
WHERE id = $1
