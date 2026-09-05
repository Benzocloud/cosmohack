package handler

import (
	"encoding/json"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// createAreaRequest — HTTP DTO для создания участка. Доменные типы значений
// используются на транспортной границе; типы хранилища не выходят из этого слоя.
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
