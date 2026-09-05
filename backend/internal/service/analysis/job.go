package analysis

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// ResolvePeriod возвращает запрошенный период или период участка по умолчанию.
func ResolvePeriod(area domain.Area, requested *domain.Period) domain.Period {
	if requested != nil {
		return *requested
	}
	return area.Period
}

// IsActiveStatus сообщает, занимает ли задача активный слот анализа участка.
func IsActiveStatus(status domain.JobStatus) bool {
	return status == domain.JobQueued || status == domain.JobRunning
}

// NewJob создаёт задачу анализа в очереди для текущего поколения участка.
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
