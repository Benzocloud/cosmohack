package source

import (
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source/geom"
)

const limitsProvider = "collector"

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
		MaxPeriodDays:       732,
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
		return Limits{}, fmt.Errorf("границы площади участка заданы неверно")
	}
	if spec.MaxPolygonVertices < 4 || spec.MaxPolygonVertices > geom.MaxRingVertices {
		return Limits{}, fmt.Errorf("предел вершин вне диапазона [4, %d]", geom.MaxRingVertices)
	}
	if spec.MaxPeriodDays <= 0 || spec.MaxObservations <= 0 {
		return Limits{}, fmt.Errorf("пределы периода и числа наблюдений должны быть положительными")
	}
	if spec.MaxSearchAreaSquare <= 0 {
		return Limits{}, fmt.Errorf("предел площади поиска контуров должен быть положительным")
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
		return NewProviderError(FailureInvalidRequest, limitsProvider, "геометрия участка не задана")
	}
	if polygon.VertexCount() > l.maxPolygonVertices {
		return NewProviderError(FailureLimitExceeded, limitsProvider,
			"вершин %d, предел %d", polygon.VertexCount(), l.maxPolygonVertices)
	}
	area := polygon.AreaHectares()
	if area < l.minAreaHectares {
		return NewProviderError(FailureLimitExceeded, limitsProvider,
			"площадь %.2f га меньше минимальной %.2f га", area, l.minAreaHectares)
	}
	if area > l.maxAreaHectares {
		return NewProviderError(FailureLimitExceeded, limitsProvider,
			"площадь %.2f га больше предельной %.2f га", area, l.maxAreaHectares)
	}
	return nil
}

func (l Limits) ValidatePeriod(period DateRange) error {
	if period.IsZero() {
		return NewProviderError(FailureInvalidRequest, limitsProvider, "период анализа не задан")
	}
	if period.Days() > l.maxPeriodDays {
		return NewProviderError(FailureLimitExceeded, limitsProvider,
			"период %d дней, предел %d", period.Days(), l.maxPeriodDays)
	}
	if period.Days() > l.maxObservations {
		return NewProviderError(FailureLimitExceeded, limitsProvider,
			"период даёт %d наблюдений, предел %d", period.Days(), l.maxObservations)
	}
	return nil
}

func (l Limits) ValidateSearchArea(bbox geom.BBox) error {
	if area := bbox.AreaHectares(); area > l.maxSearchAreaSquare {
		return NewProviderError(FailureLimitExceeded, limitsProvider,
			"область поиска %.0f га больше предельной %.0f га", area, l.maxSearchAreaSquare)
	}
	return nil
}
