package handler

import (
	"math"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

func validateGeometry(g domain.Polygon, lim Limits) error {
	if g.Type != "Polygon" {
		return errInvalidGeometry
	}

	if len(g.Coordinates) != 1 {
		return errInvalidGeometry
	}

	ring := g.Coordinates[0]
	if len(ring) < 4 {
		return errInvalidGeometry
	}

	first, last := ring[0], ring[len(ring)-1]
	if len(first) != 2 || len(last) != 2 || first[0] != last[0] || first[1] != last[1] {
		return errInvalidGeometry
	}

	for _, p := range ring {
		if len(p) != 2 {
			return errInvalidGeometry
		}

		lon, lat := p[0], p[1]
		if lon < -180 || lon > 180 || lat < -90 || lat > 90 {
			return errInvalidGeometry
		}

		if math.IsNaN(lon) || math.IsNaN(lat) || math.IsInf(lon, 0) || math.IsInf(lat, 0) {
			return errInvalidGeometry
		}
	}

	n := len(ring) - 1
	if n < 3 {
		return errInvalidGeometry
	}

	maxVertices := lim.VerticesMax
	if maxVertices <= 0 {
		maxVertices = lim.MaxVertices
	}

	if maxVertices > 0 && n > maxVertices {
		return errLimitExceeded
	}

	for i := range n {
		for j := i + 1; j < n; j++ {
			if adjacentRingEdges(i, j, n) {
				continue
			}

			if segmentsIntersect(ring[i], ring[i+1], ring[j], ring[j+1]) {
				return errInvalidGeometry
			}
		}
	}

	maxAreaKm2 := lim.AreaHaMax / 100
	if maxAreaKm2 <= 0 {
		maxAreaKm2 = lim.MaxAreaKm2
	}

	if maxAreaKm2 > 0 {
		area := ringAreaKm2(ring[:n])
		if area > maxAreaKm2 {
			return errLimitExceeded
		}
	}

	return nil
}

func adjacentRingEdges(i, j, n int) bool {
	if j == i+1 {
		return true
	}

	if i == 0 && j == n-1 {
		return true
	}

	return false
}

func segmentsIntersect(a, b, c, d []float64) bool {
	o1 := orient(a, b, c)
	o2 := orient(a, b, d)
	o3 := orient(c, d, a)

	o4 := orient(c, d, b)
	if o1 != o2 && o3 != o4 {
		return true
	}

	if o1 == 0 && onSeg(a, b, c) {
		return true
	}

	if o2 == 0 && onSeg(a, b, d) {
		return true
	}

	if o3 == 0 && onSeg(c, d, a) {
		return true
	}

	if o4 == 0 && onSeg(c, d, b) {
		return true
	}

	return false
}

func orient(a, b, c []float64) int {
	v := (b[1]-a[1])*(c[0]-b[0]) - (b[0]-a[0])*(c[1]-b[1])

	const eps = 1e-12
	if v > eps {
		return 1
	}

	if v < -eps {
		return -1
	}

	return 0
}

func onSeg(a, b, p []float64) bool {
	return p[0] <= math.Max(a[0], b[0])+1e-12 && p[0] >= math.Min(a[0], b[0])-1e-12 &&
		p[1] <= math.Max(a[1], b[1])+1e-12 && p[1] >= math.Min(a[1], b[1])-1e-12
}

func ringAreaKm2(ring [][]float64) float64 {
	// Площадь по сферической формуле на сфере 6371 км; нужна только если лимит задан.
	const r = 6371.0

	if len(ring) < 3 {
		return 0
	}

	var sum float64

	for i := range ring {
		j := (i + 1) % len(ring)
		lon1 := ring[i][0] * math.Pi / 180
		lon2 := ring[j][0] * math.Pi / 180
		lat1 := ring[i][1] * math.Pi / 180
		lat2 := ring[j][1] * math.Pi / 180
		sum += (lon2 - lon1) * (2 + math.Sin(lat1) + math.Sin(lat2))
	}

	return math.Abs(sum) * r * r / 2
}
