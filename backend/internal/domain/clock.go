package domain

import "time"

// Clock supplies the current time to deterministic domain and integration code.
type Clock func() time.Time
