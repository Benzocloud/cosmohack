package handler

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

const (
	maxNameRunes = 80
	maxBodyBytes = 1 << 20
)

var (
	errInvalidJSON     = errors.New("invalid_json")
	errInvalidGeometry = errors.New("invalid_geometry")
	errInvalidBBox     = errors.New("invalid_bbox")
	errInvalidPeriod   = errors.New("invalid_period")
	errInvalidName     = errors.New("invalid_name")
	errInvalidSource   = errors.New("invalid_source")
	errLimitExceeded   = errors.New("limit_exceeded")
	errInvalidID       = errors.New("invalid_id")
)

type bbox struct {
	MinLon, MinLat, MaxLon, MaxLat float64
}

type Limits struct {
	MaxAreaKm2  float64
	MaxVertices int
}

func parseBBox(s string) (bbox, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return bbox{}, errInvalidBBox
	}
	var nums [4]float64
	for i, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return bbox{}, errInvalidBBox
		}
		nums[i] = v
	}
	b := bbox{MinLon: nums[0], MinLat: nums[1], MaxLon: nums[2], MaxLat: nums[3]}
	if b.MinLon < -180 || b.MaxLon > 180 || b.MinLat < -90 || b.MaxLat > 90 {
		return bbox{}, errInvalidBBox
	}
	if b.MinLon >= b.MaxLon || b.MinLat >= b.MaxLat {
		return bbox{}, errInvalidBBox
	}
	return b, nil
}

func validatePeriod(p store.Period) error {
	if !validDate(p.From) || !validDate(p.To) {
		return errInvalidPeriod
	}
	if p.From > p.To {
		return errInvalidPeriod
	}
	return nil
}

func validDate(s string) bool {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return false
	}
	return t.Format("2006-01-02") == s
}

func validateName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" || utf8.RuneCountInString(n) > maxNameRunes {
		return errInvalidName
	}
	if n != name {
		return errInvalidName
	}
	return nil
}

func validateID(id string) error {
	if err := store.ValidID(id); err != nil {
		return errInvalidID
	}
	return nil
}

func validateSource(src store.Source) error {
	switch src.Kind {
	case "drawn":
		if src.ContourID != nil && *src.ContourID != "" {
			return errInvalidSource
		}
		return nil
	case "contour":
		if src.ContourID == nil || *src.ContourID == "" {
			return errInvalidSource
		}
		return nil
	default:
		return errInvalidSource
	}
}

func validateGeometry(g store.Polygon, lim Limits) error {
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
	if lim.MaxVertices > 0 && n > lim.MaxVertices {
		return errLimitExceeded
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if adjacentRingEdges(i, j, n) {
				continue
			}
			if segmentsIntersect(ring[i], ring[i+1], ring[j], ring[j+1]) {
				return errInvalidGeometry
			}
		}
	}
	if lim.MaxAreaKm2 > 0 {
		area := ringAreaKm2(ring[:n])
		if area > lim.MaxAreaKm2 {
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
	for i := 0; i < len(ring); i++ {
		j := (i + 1) % len(ring)
		lon1 := ring[i][0] * math.Pi / 180
		lon2 := ring[j][0] * math.Pi / 180
		lat1 := ring[i][1] * math.Pi / 180
		lat2 := ring[j][1] * math.Pi / 180
		sum += (lon2 - lon1) * (2 + math.Sin(lat1) + math.Sin(lat2))
	}
	return math.Abs(sum) * r * r / 2
}

type createAreaRequest struct {
	Name     string        `json:"name"`
	Period   store.Period  `json:"period"`
	Geometry store.Polygon `json:"geometry"`
	Source   *store.Source `json:"source"`
}

func decodeCreateArea(body []byte) (createAreaRequest, error) {
	var raw struct {
		Name     string          `json:"name"`
		Period   store.Period    `json:"period"`
		Geometry json.RawMessage `json:"geometry"`
		Source   *store.Source   `json:"source"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return createAreaRequest{}, errInvalidJSON
	}
	req := createAreaRequest{Name: raw.Name, Period: raw.Period, Source: raw.Source}
	if len(raw.Geometry) == 0 {
		return req, nil
	}
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw.Geometry, &peek); err != nil {
		return req, errInvalidGeometry
	}
	req.Geometry.Type = peek.Type
	if peek.Type != "Polygon" {
		return req, nil
	}
	if err := json.Unmarshal(raw.Geometry, &req.Geometry); err != nil {
		return req, errInvalidGeometry
	}
	return req, nil
}

func validateCreate(req createAreaRequest, lim Limits) error {
	if req.Source == nil {
		return errInvalidSource
	}
	if err := validateName(req.Name); err != nil {
		return err
	}
	if err := validatePeriod(req.Period); err != nil {
		return err
	}
	if err := validateGeometry(req.Geometry, lim); err != nil {
		return err
	}
	return validateSource(*req.Source)
}

type analysesRequest struct {
	Period *store.Period `json:"period"`
}

func decodeAnalyses(body []byte) (analysesRequest, error) {
	if len(body) == 0 {
		return analysesRequest{}, nil
	}
	var req analysesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return req, errInvalidJSON
	}
	return req, nil
}

func validationMessage(err error) (code, message string, retryable bool) {
	switch {
	case errors.Is(err, errInvalidJSON):
		return "invalid_json", "Тело запроса должно быть JSON-объектом", false
	case errors.Is(err, errInvalidGeometry):
		return "invalid_geometry", "Полигон должен быть замкнут", false
	case errors.Is(err, errInvalidBBox):
		return "invalid_bbox", "Параметр bbox задан неверно", false
	case errors.Is(err, errInvalidPeriod):
		return "invalid_period", "Период from/to задан неверно", false
	case errors.Is(err, errInvalidName):
		return "invalid_name", "Имя участка задано неверно", false
	case errors.Is(err, errInvalidSource):
		return "invalid_source", "Источник геометрии задан неверно", false
	case errors.Is(err, errLimitExceeded):
		return "limit_exceeded", "Полигон превышает допустимый размер", false
	case errors.Is(err, errInvalidID):
		return "not_found", "Объект не найден", false
	default:
		return "internal_error", "Внутренняя ошибка сервера", true
	}
}
