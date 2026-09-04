package geom

import (
	"math"
	"strconv"
	"strings"
)

const maxBBoxSpanDegrees = 180.0

type BBox struct {
	minLon float64
	minLat float64
	maxLon float64
	maxLat float64
}

func NewBBox(minLon, minLat, maxLon, maxLat float64) (BBox, error) {
	corners := [][2]float64{{minLon, minLat}, {maxLon, maxLat}}
	for _, corner := range corners {
		if _, err := NewCoordinate(corner[0], corner[1]); err != nil {
			return BBox{}, err
		}
	}
	if minLon >= maxLon || minLat >= maxLat {
		return BBox{}, NewValidationError(CodeInvalidBBox, "нижний угол должен быть строго меньше верхнего")
	}
	if maxLon-minLon >= maxBBoxSpanDegrees {
		return BBox{}, NewValidationError(CodeAntimeridianSpan, "ширина области %g градусов не поддерживается", maxLon-minLon)
	}
	return BBox{minLon: minLon, minLat: minLat, maxLon: maxLon, maxLat: maxLat}, nil
}

func ParseBBox(value string) (BBox, error) {
	parts := strings.Split(strings.TrimSpace(value), ",")
	if len(parts) != 4 {
		return BBox{}, NewValidationError(CodeInvalidBBox, "ожидается minLon,minLat,maxLon,maxLat")
	}
	numbers := make([]float64, 0, 4)
	for _, part := range parts {
		number, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return BBox{}, NewValidationError(CodeInvalidBBox, "значение %q не является числом", part)
		}
		numbers = append(numbers, number)
	}
	return NewBBox(numbers[0], numbers[1], numbers[2], numbers[3])
}

func (b BBox) MinLon() float64 {
	return b.minLon
}

func (b BBox) MinLat() float64 {
	return b.minLat
}

func (b BBox) MaxLon() float64 {
	return b.maxLon
}

func (b BBox) MaxLat() float64 {
	return b.maxLat
}

func (b BBox) Contains(coordinate Coordinate) bool {
	return coordinate.lon >= b.minLon && coordinate.lon <= b.maxLon &&
		coordinate.lat >= b.minLat && coordinate.lat <= b.maxLat
}

func (b BBox) Center() Coordinate {
	return Coordinate{lon: (b.minLon + b.maxLon) / 2, lat: (b.minLat + b.maxLat) / 2}
}

func (b BBox) ring() []Coordinate {
	return []Coordinate{
		{lon: b.minLon, lat: b.minLat},
		{lon: b.maxLon, lat: b.minLat},
		{lon: b.maxLon, lat: b.maxLat},
		{lon: b.minLon, lat: b.maxLat},
		{lon: b.minLon, lat: b.minLat},
	}
}

func (b BBox) AreaHectares() float64 {
	return math.Abs(ringAreaSquareMeters(b.ring())) / squareMetersPerHa
}

func (b BBox) String() string {
	return strings.Join([]string{
		formatDegrees(b.minLon), formatDegrees(b.minLat),
		formatDegrees(b.maxLon), formatDegrees(b.maxLat),
	}, ",")
}

func formatDegrees(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
