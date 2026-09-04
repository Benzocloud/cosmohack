package geom_test

import (
	"math"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source/geom"
)

func TestParseBBox(t *testing.T) {
	bbox, err := geom.ParseBBox("38.9, 45.2, 39.1, 45.35")
	if err != nil {
		t.Fatalf("bbox не разобран: %v", err)
	}
	if bbox.String() != "38.9,45.2,39.1,45.35" {
		t.Fatalf("строковое представление %q", bbox.String())
	}
	if !bbox.Contains(geom.MustCoordinate(39.0, 45.3)) {
		t.Fatal("точка внутри области не распознана")
	}
	if bbox.Contains(geom.MustCoordinate(39.2, 45.3)) {
		t.Fatal("точка вне области признана внутренней")
	}
}

func TestParseBBoxRejectsInvalidInput(t *testing.T) {
	cases := map[string]struct {
		value string
		code  geom.ErrorCode
	}{
		"мало значений":       {value: "38.9,45.2,39.1", code: geom.CodeInvalidBBox},
		"не число":            {value: "38.9,45.2,39.1,север", code: geom.CodeInvalidBBox},
		"вырожденная область": {value: "39.1,45.2,39.1,45.35", code: geom.CodeInvalidBBox},
		"перевёрнутая":        {value: "39.1,45.35,38.9,45.2", code: geom.CodeInvalidBBox},
		"широта вне мира":     {value: "38.9,-95.0,39.1,45.35", code: geom.CodeInvalidCoordinate},
		"через антимеридиан":  {value: "-179.0,45.2,179.0,45.35", code: geom.CodeAntimeridianSpan},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := geom.ParseBBox(testCase.value); geom.CodeOf(err) != testCase.code {
				t.Fatalf("код ошибки %q, ожидался %q (ошибка %v)", geom.CodeOf(err), testCase.code, err)
			}
		})
	}
}

func TestBBoxAreaAndCenter(t *testing.T) {
	bbox, err := geom.NewBBox(39.0, 45.0, 39.01, 45.01)
	if err != nil {
		t.Fatalf("bbox не построен: %v", err)
	}
	center := bbox.Center()
	if math.Abs(center.Lon()-39.005) > 1e-9 || math.Abs(center.Lat()-45.005) > 1e-9 {
		t.Fatalf("центр области %v", center)
	}
	if math.Abs(bbox.AreaHectares()-87.42)/87.42 > 0.01 {
		t.Fatalf("площадь области %g га", bbox.AreaHectares())
	}
}

func TestBBoxHandlesVerySmallArea(t *testing.T) {
	bbox, err := geom.NewBBox(38.9, 45.2, 38.9000001, 45.2000001)
	if err != nil {
		t.Fatalf("малая область не принята: %v", err)
	}
	if area := bbox.AreaHectares(); area < 0 || area > 1 {
		t.Fatalf("площадь малой области %g га", area)
	}
}
