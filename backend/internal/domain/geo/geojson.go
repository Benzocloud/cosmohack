package geom

import (
	"bytes"
	"encoding/json"
)

const (
	typePolygon           = "Polygon"
	typeFeature           = "Feature"
	typeFeatureCollection = "FeatureCollection"
)

type geoJSONDocument struct {
	Type        string             `json:"type"`
	Coordinates json.RawMessage    `json:"coordinates"`
	Geometry    *geoJSONDocument   `json:"geometry"`
	Features    []*geoJSONDocument `json:"features"`
}

type PolygonCodec struct {
	maxBytes int
}

func NewPolygonCodec(maxBytes int) *PolygonCodec {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	return &PolygonCodec{maxBytes: maxBytes}
}

func (c *PolygonCodec) Decode(payload []byte) (*Polygon, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, NewValidationError(CodeMalformedGeoJSON, "document is empty")
	}
	if len(payload) > c.maxBytes {
		return nil, NewValidationError(CodeMalformedGeoJSON, "document exceeds %d byte limit", c.maxBytes)
	}
	document := &geoJSONDocument{}
	if err := json.Unmarshal(payload, document); err != nil {
		return nil, NewValidationError(CodeMalformedGeoJSON, "document cannot be parsed")
	}
	geometry, err := c.geometryOf(document)
	if err != nil {
		return nil, err
	}
	return c.polygonOf(geometry)
}

func (c *PolygonCodec) Encode(polygon *Polygon) ([]byte, error) {
	if polygon == nil {
		return nil, NewValidationError(CodeUnsupportedShape, "polygon is required")
	}
	ring := polygon.Ring()
	coordinates := make([][]float64, 0, len(ring))
	for _, coordinate := range ring {
		coordinates = append(coordinates, []float64{coordinate.Lon(), coordinate.Lat()})
	}
	return json.Marshal(struct {
		Type        string        `json:"type"`
		Coordinates [][][]float64 `json:"coordinates"`
	}{Type: typePolygon, Coordinates: [][][]float64{coordinates}})
}

func (c *PolygonCodec) geometryOf(document *geoJSONDocument) (*geoJSONDocument, error) {
	switch document.Type {
	case typePolygon:
		return document, nil
	case typeFeature:
		if document.Geometry == nil {
			return nil, NewValidationError(CodeMalformedGeoJSON, "feature has no geometry")
		}
		return c.geometryOf(document.Geometry)
	case typeFeatureCollection:
		if len(document.Features) != 1 {
			return nil, NewValidationError(CodeUnsupportedShape, "expected exactly one object, got %d", len(document.Features))
		}
		return c.geometryOf(document.Features[0])
	case "":
		return nil, NewValidationError(CodeMalformedGeoJSON, "type field is missing")
	default:
		return nil, NewValidationError(CodeUnsupportedShape, "type %q is unsupported, expected Polygon", document.Type)
	}
}

func (c *PolygonCodec) polygonOf(document *geoJSONDocument) (*Polygon, error) {
	if len(document.Coordinates) == 0 {
		return nil, NewValidationError(CodeMalformedGeoJSON, "coordinates are missing")
	}
	rings := make([][][]float64, 0, 1)
	if err := json.Unmarshal(document.Coordinates, &rings); err != nil {
		return nil, NewValidationError(CodeUnsupportedShape, "coordinates do not represent a single polygon")
	}
	if len(rings) == 0 {
		return nil, NewValidationError(CodeMalformedGeoJSON, "coordinates are empty")
	}
	if len(rings) > 1 {
		return nil, NewValidationError(CodeUnsupportedShape, "polygons with holes are unsupported")
	}
	ring := make([]Coordinate, 0, len(rings[0]))
	for index, pair := range rings[0] {
		if len(pair) < 2 {
			return nil, NewValidationError(CodeMalformedGeoJSON, "point %d does not contain a longitude/latitude pair", index)
		}
		coordinate, err := NewCoordinate(pair[0], pair[1])
		if err != nil {
			return nil, err
		}
		ring = append(ring, coordinate)
	}
	return NewPolygon(ring)
}
