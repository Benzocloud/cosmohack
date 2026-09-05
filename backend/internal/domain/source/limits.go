package source

import (
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
)

const LimitsProvider = "collector"

const defaultMaxPeriodDays = 1461

type LimitsSpec struct {
	MinAreaHectares     float64
	MaxAreaHectares     float64
	MaxPolygonVertices  int
	MaxPeriodDays       int
	MaxObservations     int
	MaxSearchAreaSquare float64
}

type Limits struct {
	minAreaHectares     float64
	maxAreaHectares     float64
	maxPolygonVertices  int
	maxPeriodDays       int
	maxObservations     int
	maxSearchAreaSquare float64
}

func DefaultLimits() Limits {
	limits, err := NewLimits(LimitsSpec{
		MinAreaHectares:     0.5,
		MaxAreaHectares:     25000,
		MaxPolygonVertices:  512,
		MaxPeriodDays:       defaultMaxPeriodDays,
		MaxObservations:     4096,
		MaxSearchAreaSquare: 250000,
	})
	if err != nil {
		panic(err)
	}
	return limits
}

func NewLimits(spec LimitsSpec) (Limits, error) {
	if spec.MinAreaHectares <= 0 || spec.MaxAreaHectares <= spec.MinAreaHectares {
		return Limits{}, fmt.Errorf("site area bounds are invalid")
	}
	if spec.MaxPolygonVertices < 4 || spec.MaxPolygonVertices > geom.MaxRingVertices {
		return Limits{}, fmt.Errorf("vertex limit is outside range [4, %d]", geom.MaxRingVertices)
	}
	if spec.MaxPeriodDays <= 0 || spec.MaxObservations <= 0 {
		return Limits{}, fmt.Errorf("period and observation limits must be positive")
	}
	if spec.MaxSearchAreaSquare <= 0 {
		return Limits{}, fmt.Errorf("contour search area limit must be positive")
	}
	return Limits{
		minAreaHectares:     spec.MinAreaHectares,
		maxAreaHectares:     spec.MaxAreaHectares,
		maxPolygonVertices:  spec.MaxPolygonVertices,
		maxPeriodDays:       spec.MaxPeriodDays,
		maxObservations:     spec.MaxObservations,
		maxSearchAreaSquare: spec.MaxSearchAreaSquare,
	}, nil
}

func (l Limits) MinAreaHectares() float64 {
	return l.minAreaHectares
}

func (l Limits) MaxAreaHectares() float64 {
	return l.maxAreaHectares
}

func (l Limits) MaxPolygonVertices() int {
	return l.maxPolygonVertices
}

func (l Limits) MaxPeriodDays() int {
	return l.maxPeriodDays
}

func (l Limits) MaxObservations() int {
	return l.maxObservations
}

func (l Limits) MaxSearchAreaHectares() float64 {
	return l.maxSearchAreaSquare
}

func (l Limits) ValidatePolygon(polygon *geom.Polygon) error {
	if polygon == nil {
		return domain.NewProviderError(domain.FailureInvalidRequest, LimitsProvider, "site geometry is required")
	}
	if polygon.VertexCount() > l.maxPolygonVertices {
		return domain.NewProviderError(domain.FailureLimitExceeded, LimitsProvider,
			"%d vertices exceed limit %d", polygon.VertexCount(), l.maxPolygonVertices)
	}
	area := polygon.AreaHectares()
	if area < l.minAreaHectares {
		return domain.NewProviderError(domain.FailureLimitExceeded, LimitsProvider,
			"area %.2f ha is below minimum %.2f ha", area, l.minAreaHectares)
	}
	if area > l.maxAreaHectares {
		return domain.NewProviderError(domain.FailureLimitExceeded, LimitsProvider,
			"area %.2f ha exceeds maximum %.2f ha", area, l.maxAreaHectares)
	}
	return nil
}

func (l Limits) ValidatePeriod(period DateRange) error {
	if period.IsZero() {
		return domain.NewProviderError(domain.FailureInvalidRequest, LimitsProvider, "analysis period is required")
	}
	if period.Days() > l.maxPeriodDays {
		return domain.NewProviderError(domain.FailureLimitExceeded, LimitsProvider,
			"period has %d days, limit is %d", period.Days(), l.maxPeriodDays)
	}
	if period.Days() > l.maxObservations {
		return domain.NewProviderError(domain.FailureLimitExceeded, LimitsProvider,
			"period yields %d observations, limit is %d", period.Days(), l.maxObservations)
	}
	return nil
}

func (l Limits) ValidateSearchArea(bbox geom.BBox) error {
	if area := bbox.AreaHectares(); area > l.maxSearchAreaSquare {
		return domain.NewProviderError(domain.FailureLimitExceeded, LimitsProvider,
			"search area %.0f ha exceeds maximum %.0f ha", area, l.maxSearchAreaSquare)
	}
	return nil
}
