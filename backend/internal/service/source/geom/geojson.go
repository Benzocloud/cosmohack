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
		return nil, NewValidationError(CodeMalformedGeoJSON, "пустой документ")
	}
	if len(payload) > c.maxBytes {
		return nil, NewValidationError(CodeMalformedGeoJSON, "документ больше предела %d байт", c.maxBytes)
	}
	document := &geoJSONDocument{}
	if err := json.Unmarshal(payload, document); err != nil {
		return nil, NewValidationError(CodeMalformedGeoJSON, "документ не разбирается")
	}
	geometry, err := c.geometryOf(document)
	if err != nil {
		return nil, err
	}
	return c.polygonOf(geometry)
}

func (c *PolygonCodec) Encode(polygon *Polygon) ([]byte, error) {
	if polygon == nil {
		return nil, NewValidationError(CodeUnsupportedShape, "полигон не задан")
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
			return nil, NewValidationError(CodeMalformedGeoJSON, "Feature без geometry")
		}
		return c.geometryOf(document.Geometry)
	case typeFeatureCollection:
		if len(document.Features) != 1 {
			return nil, NewValidationError(CodeUnsupportedShape, "ожидается ровно один объект, получено %d", len(document.Features))
		}
		return c.geometryOf(document.Features[0])
	case "":
		return nil, NewValidationError(CodeMalformedGeoJSON, "поле type отсутствует")
	default:
		return nil, NewValidationError(CodeUnsupportedShape, "тип %q не поддерживается, ожидается Polygon", document.Type)
	}
}

func (c *PolygonCodec) polygonOf(document *geoJSONDocument) (*Polygon, error) {
	if len(document.Coordinates) == 0 {
		return nil, NewValidationError(CodeMalformedGeoJSON, "coordinates отсутствуют")
	}
	rings := make([][][]float64, 0, 1)
	if err := json.Unmarshal(document.Coordinates, &rings); err != nil {
		return nil, NewValidationError(CodeUnsupportedShape, "coordinates не соответствуют одиночному полигону")
	}
	if len(rings) == 0 {
		return nil, NewValidationError(CodeMalformedGeoJSON, "coordinates пусты")
	}
	if len(rings) > 1 {
		return nil, NewValidationError(CodeUnsupportedShape, "полигон с отверстиями не поддерживается")
	}
	ring := make([]Coordinate, 0, len(rings[0]))
	for index, pair := range rings[0] {
		if len(pair) < 2 {
			return nil, NewValidationError(CodeMalformedGeoJSON, "точка %d не содержит пары longitude/latitude", index)
		}
		coordinate, err := NewCoordinate(pair[0], pair[1])
		if err != nil {
			return nil, err
		}
		ring = append(ring, coordinate)
	}
	return NewPolygon(ring)
}
