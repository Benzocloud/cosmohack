package handler

import (
	"encoding/json"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// createAreaRequest is the HTTP DTO for area creation. Domain value types are
// used at the transport boundary; persistence types never cross this layer.
type createAreaRequest struct {
	Name     string             `json:"name"`
	Period   domain.Period      `json:"period"`
	Geometry domain.Polygon     `json:"geometry"`
	Source   *domain.AreaSource `json:"source"`
}

type createAreaRawRequest struct {
	Name     string             `json:"name"`
	Period   domain.Period      `json:"period"`
	Geometry json.RawMessage    `json:"geometry"`
	Source   *domain.AreaSource `json:"source"`
}

type createAnalysisRequest struct {
	Period *domain.Period `json:"period"`
}
