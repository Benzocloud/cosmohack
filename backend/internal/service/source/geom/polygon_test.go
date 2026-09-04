package geom_test

import (
	"math"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source/geom"
)

func ring(pairs ...[2]float64) []geom.Coordinate {
	coordinates := make([]geom.Coordinate, 0, len(pairs))
	for _, pair := range pairs {
		coordinates = append(coordinates, geom.MustCoordinate(pair[0], pair[1]))
	}
	return coordinates
}

func squareRing() []geom.Coordinate {
	return ring([2]float64{39.0, 45.0}, [2]float64{39.01, 45.0}, [2]float64{39.01, 45.01}, [2]float64{39.0, 45.01}, [2]float64{39.0, 45.0})
}

func TestNewPolygonRejectsInvalidRings(t *testing.T) {
	cases := map[string]struct {
		ring []geom.Coordinate
		code geom.ErrorCode
	}{
		"незамкнутое кольцо": {
			ring: ring([2]float64{39.0, 45.0}, [2]float64{39.01, 45.0}, [2]float64{39.01, 45.01}, [2]float64{39.0, 45.01}),
			code: geom.CodeRingNotClosed,
		},
		"мало точек": {
			ring: ring([2]float64{39.0, 45.0}, [2]float64{39.01, 45.0}, [2]float64{39.0, 45.0}),
			code: geom.CodeTooFewVertices,
		},
		"самопересечение": {
			ring: ring([2]float64{39.0, 45.0}, [2]float64{39.01, 45.01}, [2]float64{39.01, 45.0}, [2]float64{39.0, 45.01}, [2]float64{39.0, 45.0}),
			code: geom.CodeSelfIntersection,
		},
		"нулевая площадь": {
			ring: ring([2]float64{39.0, 45.0}, [2]float64{39.01, 45.0}, [2]float64{39.02, 45.0}, [2]float64{39.0, 45.0}),
			code: geom.CodeDegenerateArea,
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			polygon, err := geom.NewPolygon(testCase.ring)
			if err == nil {
				t.Fatalf("ожидалась ошибка %s, полигон построен с площадью %g", testCase.code, polygon.AreaHectares())
			}
			if code := geom.CodeOf(err); code != testCase.code {
				t.Fatalf("код ошибки %q, ожидался %q", code, testCase.code)
			}
		})
	}
}

func TestNewPolygonRejectsCoordinatesOutOfRange(t *testing.T) {
	if _, err := geom.NewCoordinate(45.0, 200.0); geom.CodeOf(err) != geom.CodeInvalidCoordinate {
		t.Fatalf("перепутанный порядок longitude/latitude должен отклоняться, получено %v", err)
	}
	if _, err := geom.NewCoordinate(math.NaN(), 45.0); geom.CodeOf(err) != geom.CodeInvalidCoordinate {
		t.Fatalf("NaN должен отклоняться, получено %v", err)
	}
}

func TestPolygonAreaMatchesSphericalEstimate(t *testing.T) {
	polygon, err := geom.NewPolygon(squareRing())
	if err != nil {
		t.Fatalf("полигон не построен: %v", err)
	}
	expected := 87.42
	if math.Abs(polygon.AreaHectares()-expected)/expected > 0.01 {
		t.Fatalf("площадь %g га отличается от ожидаемой %g га больше чем на 1%%", polygon.AreaHectares(), expected)
	}
	if polygon.VertexCount() != 4 {
		t.Fatalf("вершин %d, ожидалось 4", polygon.VertexCount())
	}
}

func TestPolygonNormalizesOrientation(t *testing.T) {
	clockwise := ring([2]float64{39.0, 45.0}, [2]float64{39.0, 45.01}, [2]float64{39.01, 45.01}, [2]float64{39.01, 45.0}, [2]float64{39.0, 45.0})
	polygon, err := geom.NewPolygon(clockwise)
	if err != nil {
		t.Fatalf("полигон не построен: %v", err)
	}
	normalized := polygon.Ring()
	if !normalized[1].Equal(geom.MustCoordinate(39.01, 45.0)) {
		t.Fatalf("кольцо не приведено к обходу против часовой стрелки: %v", normalized[1])
	}
	if !normalized[0].Equal(normalized[len(normalized)-1]) {
		t.Fatal("кольцо после нормализации не замкнуто")
	}
}

func TestPolygonRemovesRepeatedVertices(t *testing.T) {
	withRepeats := ring(
		[2]float64{39.0, 45.0}, [2]float64{39.0, 45.0}, [2]float64{39.01, 45.0},
		[2]float64{39.01, 45.01}, [2]float64{39.0, 45.01}, [2]float64{39.0, 45.0},
	)
	polygon, err := geom.NewPolygon(withRepeats)
	if err != nil {
		t.Fatalf("полигон не построен: %v", err)
	}
	if polygon.VertexCount() != 4 {
		t.Fatalf("вершин %d, ожидалось 4", polygon.VertexCount())
	}
}

func TestRepresentativePointStaysInsideConcavePolygon(t *testing.T) {
	concave := ring(
		[2]float64{39.0, 45.0}, [2]float64{39.03, 45.0}, [2]float64{39.03, 45.005},
		[2]float64{39.005, 45.005}, [2]float64{39.005, 45.025}, [2]float64{39.03, 45.025},
		[2]float64{39.03, 45.03}, [2]float64{39.0, 45.03}, [2]float64{39.0, 45.0},
	)
	polygon, err := geom.NewPolygon(concave)
	if err != nil {
		t.Fatalf("полигон не построен: %v", err)
	}
	point := polygon.RepresentativePoint()
	if !polygon.Contains(point) {
		t.Fatalf("репрезентативная точка %v оказалась вне полигона", point)
	}
	if !polygon.BBox().Contains(point) {
		t.Fatalf("репрезентативная точка %v вне bbox", point)
	}
}

func TestPolygonContains(t *testing.T) {
	polygon, err := geom.NewPolygon(squareRing())
	if err != nil {
		t.Fatalf("полигон не построен: %v", err)
	}
	if !polygon.Contains(geom.MustCoordinate(39.005, 45.005)) {
		t.Fatal("внутренняя точка не распознана")
	}
	if polygon.Contains(geom.MustCoordinate(39.02, 45.005)) {
		t.Fatal("внешняя точка признана внутренней")
	}
}
