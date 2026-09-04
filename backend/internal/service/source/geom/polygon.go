package geom

import "math"

const (
	MaxRingVertices     = 4096
	earthRadiusMeters   = 6371008.8
	squareMetersPerHa   = 10000.0
	minimalAreaHectares = 1e-6
)

type Polygon struct {
	ring         []Coordinate
	bbox         BBox
	areaHectares float64
}

func NewPolygon(ring []Coordinate) (*Polygon, error) {
	normalized, err := normalizeRing(ring)
	if err != nil {
		return nil, err
	}
	if err := ensureSimple(normalized); err != nil {
		return nil, err
	}
	signedArea := ringAreaSquareMeters(normalized)
	if math.Abs(signedArea)/squareMetersPerHa < minimalAreaHectares {
		return nil, NewValidationError(CodeDegenerateArea, "площадь полигона вырождена")
	}
	if signedArea > 0 {
		normalized = reverseRing(normalized)
	}
	bbox, err := ringBBox(normalized)
	if err != nil {
		return nil, err
	}
	return &Polygon{ring: normalized, bbox: bbox, areaHectares: math.Abs(signedArea) / squareMetersPerHa}, nil
}

func (p *Polygon) Ring() []Coordinate {
	ring := make([]Coordinate, len(p.ring))
	copy(ring, p.ring)
	return ring
}

func (p *Polygon) VertexCount() int {
	return len(p.ring) - 1
}

func (p *Polygon) BBox() BBox {
	return p.bbox
}

func (p *Polygon) AreaHectares() float64 {
	return p.areaHectares
}

func (p *Polygon) Contains(coordinate Coordinate) bool {
	inside := false
	for i, j := 0, len(p.ring)-2; i < len(p.ring)-1; j, i = i, i+1 {
		current, previous := p.ring[i], p.ring[j]
		if (current.lat > coordinate.lat) == (previous.lat > coordinate.lat) {
			continue
		}
		crossing := (previous.lon-current.lon)*(coordinate.lat-current.lat)/(previous.lat-current.lat) + current.lon
		if coordinate.lon < crossing {
			inside = !inside
		}
	}
	return inside
}

func (p *Polygon) RepresentativePoint() Coordinate {
	centroid := p.centroid()
	if p.Contains(centroid) {
		return centroid
	}
	if point, ok := p.scanlineMidpoint(centroid.lat); ok {
		return point
	}
	if point, ok := p.scanlineMidpoint((p.bbox.minLat + p.bbox.maxLat) / 2); ok {
		return point
	}
	return p.ring[0]
}

func (p *Polygon) centroid() Coordinate {
	var doubleArea, lon, lat float64
	for i := 0; i < len(p.ring)-1; i++ {
		current, next := p.ring[i], p.ring[i+1]
		cross := current.lon*next.lat - next.lon*current.lat
		doubleArea += cross
		lon += (current.lon + next.lon) * cross
		lat += (current.lat + next.lat) * cross
	}
	if doubleArea == 0 {
		return p.bbox.Center()
	}
	return Coordinate{lon: lon / (3 * doubleArea), lat: lat / (3 * doubleArea)}
}

func (p *Polygon) scanlineMidpoint(latitude float64) (Coordinate, bool) {
	crossings := make([]float64, 0, len(p.ring))
	for i := 0; i < len(p.ring)-1; i++ {
		current, next := p.ring[i], p.ring[i+1]
		if (current.lat > latitude) == (next.lat > latitude) {
			continue
		}
		crossings = append(crossings, (next.lon-current.lon)*(latitude-current.lat)/(next.lat-current.lat)+current.lon)
	}
	if len(crossings) < 2 {
		return Coordinate{}, false
	}
	sortFloats(crossings)
	bestFrom, bestTo, bestWidth := 0.0, 0.0, -1.0
	for i := 0; i+1 < len(crossings); i += 2 {
		if width := crossings[i+1] - crossings[i]; width > bestWidth {
			bestFrom, bestTo, bestWidth = crossings[i], crossings[i+1], width
		}
	}
	if bestWidth <= 0 {
		return Coordinate{}, false
	}
	return Coordinate{lon: (bestFrom + bestTo) / 2, lat: latitude}, true
}

func normalizeRing(ring []Coordinate) ([]Coordinate, error) {
	if len(ring) > MaxRingVertices {
		return nil, NewValidationError(CodeTooManyVertices, "%d вершин превышает предел %d", len(ring), MaxRingVertices)
	}
	if len(ring) < 4 {
		return nil, NewValidationError(CodeTooFewVertices, "кольцо содержит %d точек, нужно не менее 4", len(ring))
	}
	if !ring[0].Equal(ring[len(ring)-1]) {
		return nil, NewValidationError(CodeRingNotClosed, "первая и последняя точки не совпадают")
	}
	deduplicated := make([]Coordinate, 0, len(ring))
	for i, coordinate := range ring {
		if _, err := NewCoordinate(coordinate.lon, coordinate.lat); err != nil {
			return nil, err
		}
		if i > 0 && coordinate.Equal(deduplicated[len(deduplicated)-1]) {
			continue
		}
		deduplicated = append(deduplicated, coordinate)
	}
	if len(deduplicated) < 4 {
		return nil, NewValidationError(CodeTooFewVertices, "после удаления повторов осталось %d точек", len(deduplicated))
	}
	return deduplicated, nil
}

func ensureSimple(ring []Coordinate) error {
	segments := len(ring) - 1
	for i := 0; i < segments; i++ {
		for j := i + 1; j < segments; j++ {
			if j == i+1 || (i == 0 && j == segments-1) {
				continue
			}
			if segmentsIntersect(ring[i], ring[i+1], ring[j], ring[j+1]) {
				return NewValidationError(CodeSelfIntersection, "рёбра %d и %d пересекаются", i, j)
			}
		}
	}
	return nil
}

func segmentsIntersect(a, b, c, d Coordinate) bool {
	first := orientation(c, d, a)
	second := orientation(c, d, b)
	third := orientation(a, b, c)
	fourth := orientation(a, b, d)
	if ((first > 0 && second < 0) || (first < 0 && second > 0)) &&
		((third > 0 && fourth < 0) || (third < 0 && fourth > 0)) {
		return true
	}
	if first == 0 && onSegment(c, d, a) {
		return true
	}
	if second == 0 && onSegment(c, d, b) {
		return true
	}
	if third == 0 && onSegment(a, b, c) {
		return true
	}
	if fourth == 0 && onSegment(a, b, d) {
		return true
	}
	return false
}

func orientation(a, b, c Coordinate) float64 {
	return (b.lon-a.lon)*(c.lat-a.lat) - (b.lat-a.lat)*(c.lon-a.lon)
}

func onSegment(a, b, point Coordinate) bool {
	return math.Min(a.lon, b.lon) <= point.lon && point.lon <= math.Max(a.lon, b.lon) &&
		math.Min(a.lat, b.lat) <= point.lat && point.lat <= math.Max(a.lat, b.lat)
}

func ringAreaSquareMeters(ring []Coordinate) float64 {
	var total float64
	for i := 0; i < len(ring)-1; i++ {
		current, next := ring[i], ring[i+1]
		total += toRadians(next.lon-current.lon) * (2 + math.Sin(toRadians(current.lat)) + math.Sin(toRadians(next.lat)))
	}
	return total * earthRadiusMeters * earthRadiusMeters / 2
}

func ringBBox(ring []Coordinate) (BBox, error) {
	minLon, minLat := ring[0].lon, ring[0].lat
	maxLon, maxLat := ring[0].lon, ring[0].lat
	for _, coordinate := range ring[1:] {
		minLon = math.Min(minLon, coordinate.lon)
		minLat = math.Min(minLat, coordinate.lat)
		maxLon = math.Max(maxLon, coordinate.lon)
		maxLat = math.Max(maxLat, coordinate.lat)
	}
	return NewBBox(minLon, minLat, maxLon, maxLat)
}

func reverseRing(ring []Coordinate) []Coordinate {
	reversed := make([]Coordinate, len(ring))
	for i, coordinate := range ring {
		reversed[len(ring)-1-i] = coordinate
	}
	return reversed
}

func sortFloats(values []float64) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func toRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}
