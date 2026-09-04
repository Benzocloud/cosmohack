package analysis

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// NewJob creates a queued analysis job for the current area generation.
func NewJob(area domain.Area, period domain.Period, now time.Time) (domain.Job, error) {
	id, err := newJobID()
	if err != nil {
		return domain.Job{}, err
	}
	now = now.UTC()
	return domain.Job{
		ID: id, AreaID: area.ID, Status: domain.JobQueued, Period: period,
		CreatedAt: now, UpdatedAt: now, AreaGeneration: area.Generation,
	}, nil
}

func newJobID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
