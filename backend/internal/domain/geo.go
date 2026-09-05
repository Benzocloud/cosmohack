package domain

// BBox — поисковый прямоугольник WGS84 в порядке долгота/широта.
type BBox struct {
	MinLon float64 `json:"min_lon"`
	MinLat float64 `json:"min_lat"`
	MaxLon float64 `json:"max_lon"`
	MaxLat float64 `json:"max_lat"`
}

type ContourSource struct {
	Provider    string `json:"provider"`
	Attribution string `json:"attribution"`
}

type Contour struct {
	ID       string        `json:"id"`
	Geometry Polygon       `json:"geometry"`
	Source   ContourSource `json:"source"`
}
