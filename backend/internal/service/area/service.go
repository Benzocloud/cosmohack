// Package area contains area use-case construction and invariants.
package area

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// CreateInput contains validated user input for a new area.
type CreateInput struct {
	Name     string
	Period   domain.Period
	Geometry domain.Polygon
	Source   domain.AreaSource
}

// Create builds a new area aggregate with its initial generation.
func Create(input CreateInput, now time.Time) (domain.Area, error) {
	id, err := newID()
	if err != nil {
		return domain.Area{}, err
	}
	return domain.Area{
		ID:         id,
		Name:       input.Name,
		Geometry:   input.Geometry,
		Source:     input.Source,
		Period:     input.Period,
		CreatedAt:  now.UTC(),
		Generation: 1,
	}, nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
