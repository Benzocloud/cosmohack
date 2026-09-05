UPDATE areas
SET period_from = $2,
    period_to = $3
WHERE id = $1
