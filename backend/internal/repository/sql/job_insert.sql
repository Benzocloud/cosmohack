INSERT INTO jobs (
    id,
    area_id,
    status,
    stage,
    period_from,
    period_to,
    area_generation,
    created_at,
    updated_at
)
VALUES ($1, $2, 'queued', NULL, $3, $4, $5, $6, $6)
