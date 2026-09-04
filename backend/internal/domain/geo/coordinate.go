package geom

import "math"

const (
	minLongitude = -180.0
	maxLongitude = 180.0
	minLatitude  = -90.0
	maxLatitude  = 90.0
)

type Coordinate struct {
	lon float64
	lat float64
}

func NewCoordinate(lon, lat float64) (Coordinate, error) {
	if math.IsNaN(lon) || math.IsInf(lon, 0) || math.IsNaN(lat) || math.IsInf(lat, 0) {
		return Coordinate{}, NewValidationError(CodeInvalidCoordinate, "координата не является конечным числом")
	}
	if lon < minLongitude || lon > maxLongitude {
		return Coordinate{}, NewValidationError(CodeInvalidCoordinate, "долгота %g вне диапазона [-180, 180]", lon)
	}
	if lat < minLatitude || lat > maxLatitude {
		return Coordinate{}, NewValidationError(CodeInvalidCoordinate, "широта %g вне диапазона [-90, 90]", lat)
	}
	return Coordinate{lon: lon, lat: lat}, nil
}

func MustCoordinate(lon, lat float64) Coordinate {
	coordinate, err := NewCoordinate(lon, lat)
	if err != nil {
		panic(err)
	}
	return coordinate
}

func (c Coordinate) Lon() float64 {
	return c.lon
}

func (c Coordinate) Lat() float64 {
	return c.lat
}

func (c Coordinate) Equal(other Coordinate) bool {
	return c.lon == other.lon && c.lat == other.lat
}
