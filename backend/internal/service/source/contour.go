package source

import (
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
)

type Origin struct {
	provider        string
	dataset         string
	license         string
	attribution     string
	query           string
	upstreamVersion string
	retrievedAt     time.Time
}

type OriginSpec struct {
	Provider        string
	Dataset         string
	License         string
	Attribution     string
	Query           string
	UpstreamVersion string
	RetrievedAt     time.Time
}

func NewOrigin(spec OriginSpec) (Origin, error) {
	if spec.Provider == "" || spec.Dataset == "" {
		return Origin{}, fmt.Errorf("происхождение контура без provider или dataset")
	}
	if spec.RetrievedAt.IsZero() {
		return Origin{}, fmt.Errorf("происхождение контура без времени получения")
	}
	return Origin{
		provider:        spec.Provider,
		dataset:         spec.Dataset,
		license:         spec.License,
		attribution:     spec.Attribution,
		query:           spec.Query,
		upstreamVersion: spec.UpstreamVersion,
		retrievedAt:     spec.RetrievedAt.UTC(),
	}, nil
}

func (o Origin) Provider() string {
	return o.provider
}

func (o Origin) Dataset() string {
	return o.dataset
}

func (o Origin) License() string {
	return o.license
}

func (o Origin) Attribution() string {
	return o.attribution
}

func (o Origin) Query() string {
	return o.query
}

func (o Origin) UpstreamVersion() string {
	return o.upstreamVersion
}

func (o Origin) RetrievedAt() time.Time {
	return o.retrievedAt
}

func (o Origin) IsZero() bool {
	return o.provider == ""
}

type Contour struct {
	id      string
	name    string
	polygon *geom.Polygon
	origin  Origin
	tags    map[string]string
}

func NewContour(id, name string, polygon *geom.Polygon, origin Origin, tags map[string]string) (Contour, error) {
	if err := requireIdentifier("id контура", id); err != nil {
		return Contour{}, err
	}
	if polygon == nil {
		return Contour{}, fmt.Errorf("контур %s без геометрии", id)
	}
	if origin.IsZero() {
		return Contour{}, fmt.Errorf("контур %s без происхождения", id)
	}
	copied := make(map[string]string, len(tags))
	for key, value := range tags {
		copied[key] = value
	}
	return Contour{id: id, name: name, polygon: polygon, origin: origin, tags: copied}, nil
}

func (c Contour) ID() string {
	return c.id
}

func (c Contour) Name() string {
	return c.name
}

func (c Contour) Polygon() *geom.Polygon {
	return c.polygon
}

func (c Contour) Origin() Origin {
	return c.origin
}

func (c Contour) Tags() map[string]string {
	tags := make(map[string]string, len(c.tags))
	for key, value := range c.tags {
		tags[key] = value
	}
	return tags
}

type ContourSearchResult struct {
	bbox      geom.BBox
	origin    Origin
	contours  []Contour
	truncated bool
	notes     []string
}

func NewContourSearchResult(bbox geom.BBox, origin Origin, contours []Contour, truncated bool, notes []string) ContourSearchResult {
	stored := make([]Contour, len(contours))
	copy(stored, contours)
	storedNotes := make([]string, len(notes))
	copy(storedNotes, notes)
	return ContourSearchResult{bbox: bbox, origin: origin, contours: stored, truncated: truncated, notes: storedNotes}
}

func (r ContourSearchResult) BBox() geom.BBox {
	return r.bbox
}

func (r ContourSearchResult) Origin() Origin {
	return r.origin
}

func (r ContourSearchResult) Contours() []Contour {
	contours := make([]Contour, len(r.contours))
	copy(contours, r.contours)
	return contours
}

func (r ContourSearchResult) Count() int {
	return len(r.contours)
}

func (r ContourSearchResult) IsEmpty() bool {
	return len(r.contours) == 0
}

func (r ContourSearchResult) Truncated() bool {
	return r.truncated
}

func (r ContourSearchResult) Notes() []string {
	notes := make([]string, len(r.notes))
	copy(notes, r.notes)
	return notes
}
