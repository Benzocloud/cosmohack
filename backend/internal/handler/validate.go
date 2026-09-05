package handler

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
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

func validatePeriod(p domain.Period) error {
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

func validateSource(src domain.AreaSource) error {
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
