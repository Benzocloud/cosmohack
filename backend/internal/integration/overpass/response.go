package overpass

import (
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
)

type responseDocument struct {
	Version   float64 `json:"version"`
	Generator string  `json:"generator"`
	Osm3s     struct {
		TimestampOsmBase string `json:"timestamp_osm_base"`
		Copyright        string `json:"copyright"`
	} `json:"osm3s"`
	Elements []responseElement `json:"elements"`
}

type responseElement struct {
	Type     string            `json:"type"`
	ID       int64             `json:"id"`
	Geometry []responseVertex  `json:"geometry"`
	Tags     map[string]string `json:"tags"`
}

type responseVertex struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func (e responseElement) isSupported() bool {
	return e.Type == "way" && len(e.Geometry) > 0
}

func (e responseElement) contourID() string {
	return fmt.Sprintf("osm/way/%d", e.ID)
}

func (e responseElement) name() string {
	if name, ok := e.Tags["name"]; ok && name != "" {
		return name
	}
	return fmt.Sprintf("Контур OSM %d", e.ID)
}

func (e responseElement) polygon() (*geom.Polygon, error) {
	ring := make([]geom.Coordinate, 0, len(e.Geometry))
	for _, vertex := range e.Geometry {
		coordinate, err := geom.NewCoordinate(vertex.Lon, vertex.Lat)
		if err != nil {
			return nil, err
		}
		ring = append(ring, coordinate)
	}
	return geom.NewPolygon(ring)
}
