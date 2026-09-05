package handler

import (
	"encoding/json"

	geom "github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
)

func decodeCreateArea(body []byte) (createAreaRequest, error) {
	var raw createAreaRawRequest
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

	codec := geom.NewPolygonCodec(maxBodyBytes)
	polygon, err := codec.Decode(raw.Geometry)
	if err != nil {
		return req, errInvalidGeometry
	}
	canonical, err := codec.Encode(polygon)
	if err != nil {
		return req, errInvalidGeometry
	}
	if err := json.Unmarshal(canonical, &req.Geometry); err != nil {
		return req, errInvalidGeometry
	}

	return req, nil
}

func decodeCreateAnalysis(body []byte) (createAnalysisRequest, error) {
	if len(body) == 0 {
		return createAnalysisRequest{}, nil
	}
	var req createAnalysisRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return req, errInvalidJSON
	}
	return req, nil
}
