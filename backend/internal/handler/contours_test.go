package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/handler"
)

func TestContoursTable(t *testing.T) {
	found := handler.StubContours{Items: []handler.Contour{{
		ID: "osm-way-123",
		Geometry: domain.Polygon{
			Type: "Polygon",
			Coordinates: [][][]float64{
				{{37.5, 55.7}, {37.6, 55.7}, {37.6, 55.8}, {37.5, 55.8}, {37.5, 55.7}},
			},
		},
		Source: handler.ContourSource{Provider: "osm", Attribution: "© OpenStreetMap"},
	}}}

	t.Run("bbox_ok", func(t *testing.T) {
		h, _ := newEnv(t, found, nil)
		w := doJSON(t, h, http.MethodGet, "/api/regions/contours?bbox=-1,-1,1,1", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s", w.Body.String())
		}
	})
	t.Run("bbox_min_eq_max", func(t *testing.T) {
		h, _ := newEnv(t, found, nil)
		w := doJSON(t, h, http.MethodGet, "/api/regions/contours?bbox=1,1,1,1", nil)
		if w.Code != 400 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	})
	t.Run("bbox_empty", func(t *testing.T) {
		h, _ := newEnv(t, found, nil)
		w := doJSON(t, h, http.MethodGet, "/api/regions/contours?bbox=", nil)
		if w.Code != 400 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	})
	t.Run("bbox_three_nums", func(t *testing.T) {
		h, _ := newEnv(t, found, nil)
		w := doJSON(t, h, http.MethodGet, "/api/regions/contours?bbox=1,2,3", nil)
		if w.Code != 400 {
			t.Fatalf("%d", w.Code)
		}
	})
	t.Run("found", func(t *testing.T) {
		h, _ := newEnv(t, found, nil)
		w := doJSON(t, h, http.MethodGet, "/api/regions/contours?bbox=-1,-1,1,1", nil)
		var wrap struct {
			Contours []json.RawMessage `json:"contours"`
		}
		decode(t, w, &wrap)
		if len(wrap.Contours) != 1 {
			t.Fatalf("n=%d %s", len(wrap.Contours), w.Body.String())
		}
	})
	t.Run("empty", func(t *testing.T) {
		h, _ := newEnv(t, handler.StubContours{}, nil)
		w := doJSON(t, h, http.MethodGet, "/api/regions/contours?bbox=-1,-1,1,1", nil)
		var wrap struct {
			Contours []any `json:"contours"`
		}
		decode(t, w, &wrap)
		if wrap.Contours == nil || len(wrap.Contours) != 0 {
			t.Fatalf("%s", w.Body.String())
		}
	})
	t.Run("error", func(t *testing.T) {
		h, _ := newEnv(t, handler.StubContours{Err: errors.New("overpass down")}, nil)
		w := doJSON(t, h, http.MethodGet, "/api/regions/contours?bbox=-1,-1,1,1", nil)
		if w.Code != 500 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		var env struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		decode(t, w, &env)
		if env.Error.Code != "source_unavailable" {
			t.Fatalf("%s", w.Body.String())
		}
	})
}
