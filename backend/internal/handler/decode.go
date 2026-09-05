package handler

import "encoding/json"

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
	if err := json.Unmarshal(raw.Geometry, &req.Geometry); err != nil {
		return req, errInvalidGeometry
	}
	return req, nil
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
