package domain

import "time"

// Clock передаёт текущее время детерминированному доменному и интеграционному коду.
type Clock func() time.Time
