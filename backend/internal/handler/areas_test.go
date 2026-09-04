package handler_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/handler"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

func testdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newEnv(t *testing.T, contours handler.ContourFinder, q *handler.StubQueue) (http.Handler, *store.Store) {
	t.Helper()
	return newEnvDir(t, t.TempDir(), contours, q)
}

func newEnvDir(t *testing.T, dir string, contours handler.ContourFinder, q *handler.StubQueue) (http.Handler, *store.Store) {
	t.Helper()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if contours == nil {
		contours = handler.StubContours{}
	}
	if q == nil {
		q = handler.NewStubQueue(8)
	}
	return handler.NewMux(st, contours, q, handler.Limits{}), st
}

func doReq(t *testing.T, h http.Handler, method, path string, body []byte, ct string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func doJSON(t *testing.T, h http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	return doReq(t, h, method, path, body, "application/json")
}

func decode(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
}

func createArea(t *testing.T, h http.Handler) string {
	t.Helper()
	w := doJSON(t, h, http.MethodPost, "/api/areas", testdata(t, "area-create-valid.json"))
	if w.Code != http.StatusCreated {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var a map[string]any
	decode(t, w, &a)
	id, _ := a["id"].(string)
	if id == "" {
		t.Fatal("no id")
	}
	return id
}

func TestTestdataFilesAreJSON(t *testing.T) {
	t.Parallel()
	matches, err := filepath.Glob("testdata/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) < 11 {
		t.Fatalf("testdata count=%d", len(matches))
	}
	for _, f := range matches {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(b) {
			t.Errorf("invalid json %s", f)
		}
	}
}

func TestCRUDChain(t *testing.T) {
	h, _ := newEnv(t, nil, nil)

	w := doJSON(t, h, http.MethodGet, "/api/areas", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list %d %s", w.Code, w.Body.String())
	}
	var wrap struct {
		Areas []any `json:"areas"`
	}
	decode(t, w, &wrap)
	if len(wrap.Areas) != 0 {
		t.Fatalf("empty list: %s", w.Body.String())
	}

	id := createArea(t, h)

	w = doJSON(t, h, http.MethodGet, "/api/areas", nil)
	var list struct {
		Areas []map[string]any `json:"areas"`
	}
	decode(t, w, &list)
	if len(list.Areas) != 1 || list.Areas[0]["id"] != id {
		t.Fatalf("list=%s", w.Body.String())
	}

	w = doJSON(t, h, http.MethodGet, "/api/areas/"+id+"/series", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("series %d %s", w.Code, w.Body.String())
	}
	var series map[string]any
	decode(t, w, &series)
	if series["result_version"] != nil {
		t.Fatalf("series=%s", w.Body.String())
	}
	if s, ok := series["series"].([]any); !ok || len(s) != 0 {
		t.Fatalf("series rows=%s", w.Body.String())
	}
	if s, ok := series["weather"].([]any); !ok || len(s) != 0 {
		t.Fatalf("weather=%s", w.Body.String())
	}

	w = doJSON(t, h, http.MethodGet, "/api/areas/"+id+"/events", nil)
	var events map[string]any
	decode(t, w, &events)
	if _, ok := events["events"].([]any); !ok {
		t.Fatalf("events=%s", w.Body.String())
	}

	w = doJSON(t, h, http.MethodDelete, "/api/areas/"+id, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", w.Code, w.Body.String())
	}

	w = doJSON(t, h, http.MethodGet, "/api/areas", nil)
	decode(t, w, &list)
	if len(list.Areas) != 0 {
		t.Fatalf("after delete %s", w.Body.String())
	}

	w = doJSON(t, h, http.MethodGet, "/api/areas/"+id+"/series", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("series after delete %d", w.Code)
	}

	w = doJSON(t, h, http.MethodDelete, "/api/areas/"+id, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("repeat delete %d", w.Code)
	}

	id2 := createArea(t, h)
	if id2 == id {
		t.Fatal("same id")
	}
}

func TestCreateValidation(t *testing.T) {
	h, _ := newEnv(t, nil, nil)
	valid := testdata(t, "area-create-valid.json")
	open := testdata(t, "area-create-open-ring.json")

	tests := []struct {
		name    string
		body    []byte
		code    int
		errCode string
	}{
		{name: "geom_ok", body: valid, code: 201},
		{name: "geom_open_ring", body: open, code: 400, errCode: "invalid_geometry"},
		{name: "geom_not_polygon", body: []byte(`{"name":"a","period":{"from":"2024-01-01","to":"2024-01-02"},"geometry":{"type":"Point","coordinates":[1,2]},"source":{"kind":"drawn"}}`), code: 400, errCode: "invalid_geometry"},
		{name: "geom_few_points", body: []byte(`{"name":"a","period":{"from":"2024-01-01","to":"2024-01-02"},"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,0]]]},"source":{"kind":"drawn"}}`), code: 400, errCode: "invalid_geometry"},
		{name: "geom_lat_as_lon", body: []byte(`{"name":"a","period":{"from":"2024-01-01","to":"2024-01-02"},"geometry":{"type":"Polygon","coordinates":[[[200,10],[201,10],[201,11],[200,11],[200,10]]]},"source":{"kind":"drawn"}}`), code: 400, errCode: "invalid_geometry"},
		{name: "geom_self_intersection", body: []byte(`{"name":"a","period":{"from":"2024-01-01","to":"2024-01-02"},"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,1],[0,1],[1,0],[0,0]]]},"source":{"kind":"drawn"}}`), code: 400, errCode: "invalid_geometry"},
		{name: "period_backwards", body: []byte(`{"name":"a","period":{"from":"2024-12-01","to":"2024-01-02"},"geometry":{"type":"Polygon","coordinates":[[[37.5,55.7],[37.6,55.7],[37.6,55.8],[37.5,55.8],[37.5,55.7]]]},"source":{"kind":"drawn"}}`), code: 400, errCode: "invalid_period"},
		{name: "period_bad_date", body: []byte(`{"name":"a","period":{"from":"2024-13-01","to":"2024-01-02"},"geometry":{"type":"Polygon","coordinates":[[[37.5,55.7],[37.6,55.7],[37.6,55.8],[37.5,55.8],[37.5,55.7]]]},"source":{"kind":"drawn"}}`), code: 400, errCode: "invalid_period"},
		{name: "name_empty", body: []byte(`{"name":"   ","period":{"from":"2024-01-01","to":"2024-01-02"},"geometry":{"type":"Polygon","coordinates":[[[37.5,55.7],[37.6,55.7],[37.6,55.8],[37.5,55.8],[37.5,55.7]]]},"source":{"kind":"drawn"}}`), code: 400, errCode: "invalid_name"},
		{name: "source_missing", body: []byte(`{"name":"a","period":{"from":"2024-01-01","to":"2024-01-02"},"geometry":{"type":"Polygon","coordinates":[[[37.5,55.7],[37.6,55.7],[37.6,55.8],[37.5,55.8],[37.5,55.7]]]}}`), code: 400, errCode: "invalid_source"},
		{name: "source_contour_no_id", body: []byte(`{"name":"a","period":{"from":"2024-01-01","to":"2024-01-02"},"geometry":{"type":"Polygon","coordinates":[[[37.5,55.7],[37.6,55.7],[37.6,55.8],[37.5,55.8],[37.5,55.7]]]},"source":{"kind":"contour","contour_id":null}}`), code: 400, errCode: "invalid_source"},
		{name: "source_drawn_with_id", body: []byte(`{"name":"a","period":{"from":"2024-01-01","to":"2024-01-02"},"geometry":{"type":"Polygon","coordinates":[[[37.5,55.7],[37.6,55.7],[37.6,55.8],[37.5,55.8],[37.5,55.7]]]},"source":{"kind":"drawn","contour_id":"x"}}`), code: 400, errCode: "invalid_source"},
		{name: "json_garbage", body: []byte(`{`), code: 400, errCode: "invalid_json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doJSON(t, h, http.MethodPost, "/api/areas", tt.body)
			if w.Code != tt.code {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
			if tt.errCode != "" {
				var env struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				decode(t, w, &env)
				if env.Error.Code != tt.errCode {
					t.Fatalf("err=%s", w.Body.String())
				}
			}
		})
	}
}

func TestIDTraversal(t *testing.T) {
	h, _ := newEnv(t, nil, nil)
	w := doJSON(t, h, http.MethodGet, "/api/areas/../areas/series", nil)
	// ServeMux чистит /../ редиректом; чужой файл при этом не читается.
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest &&
		w.Code != http.StatusMovedPermanently && w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("code=%d", w.Code)
	}
	w = doJSON(t, h, http.MethodGet, "/api/areas/not.a.valid/series", nil)
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Fatalf("dot id code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestEmptyBodyCreate(t *testing.T) {
	h, _ := newEnv(t, nil, nil)
	w := doReq(t, h, http.MethodPost, "/api/areas", nil, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateBodyLimits(t *testing.T) {
	h, _ := newEnv(t, nil, nil)
	w := doReq(t, h, http.MethodPost, "/api/areas", []byte(`{"name":"x"}`), "text/plain")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("ct %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decode(t, w, &env)
	if env.Error.Code != "invalid_json" {
		t.Fatalf("%s", w.Body.String())
	}
	big := make([]byte, (1<<20)+2)
	for i := range big {
		big[i] = 'a'
	}
	w = doReq(t, h, http.MethodPost, "/api/areas", big, "application/json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversize %d %s", w.Code, w.Body.String())
	}
}
