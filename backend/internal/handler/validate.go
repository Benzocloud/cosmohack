package handler

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
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
	errInvalidVersion  = errors.New("invalid_version")
	errInvalidID       = errors.New("invalid_id")
	idPattern          = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

type bbox struct {
	MinLon, MinLat, MaxLon, MaxLat float64
}

type Limits struct {
	// Канонические публичные лимиты повторяют конфигурацию сборки источников.
	AreaHaMax     float64
	VerticesMax   int
	PeriodDaysMax int
	MinDate       string

	// Устаревшие поля совместимости сохранены для вызывающих сторон, которые ещё не
	// перевели корень композиции на канонические имена.
	MaxAreaKm2  float64
	MaxVertices int
}

func validatePeriod(p domain.Period, lim Limits) error {
	if !validDate(p.From) || !validDate(p.To) {
		return errInvalidPeriod
	}

	if p.From > p.To {
		return errInvalidPeriod
	}

	if lim.MinDate != "" && validDate(lim.MinDate) && p.From < lim.MinDate {
		return errLimitExceeded
	}

	if lim.PeriodDaysMax > 0 {
		from, _ := time.Parse("2006-01-02", p.From)
		to, _ := time.Parse("2006-01-02", p.To)

		days := int(to.Sub(from).Hours()/24) + 1
		if days > lim.PeriodDaysMax {
			return errLimitExceeded
		}
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
	if !idPattern.MatchString(id) {
		return errInvalidID
	}

	return nil
}

func validateVersion(version string) error {
	return validateID(version)
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

	if err := validatePeriod(req.Period, lim); err != nil {
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
		return errorCodeInvalidJSON, publicErrorMessage(errorCodeInvalidJSON), false
	case errors.Is(err, errInvalidGeometry):
		return errorCodeInvalidGeometry, publicErrorMessage(errorCodeInvalidGeometry), false
	case errors.Is(err, errInvalidBBox):
		return errorCodeInvalidBBox, publicErrorMessage(errorCodeInvalidBBox), false
	case errors.Is(err, errInvalidPeriod):
		return errorCodeInvalidPeriod, publicErrorMessage(errorCodeInvalidPeriod), false
	case errors.Is(err, errInvalidName):
		return errorCodeInvalidName, publicErrorMessage(errorCodeInvalidName), false
	case errors.Is(err, errInvalidSource):
		return errorCodeInvalidSource, publicErrorMessage(errorCodeInvalidSource), false
	case errors.Is(err, errLimitExceeded):
		return errorCodeLimitExceeded, publicErrorMessage(errorCodeLimitExceeded), false
	case errors.Is(err, errInvalidID):
		return errorCodeNotFound, publicErrorMessage(errorCodeNotFound), false
	case errors.Is(err, errInvalidVersion):
		return errorCodeInvalidVersion, publicErrorMessage(errorCodeInvalidVersion), false
	default:
		return errorCodeInternal, publicErrorMessage(errorCodeInternal), true
	}
}
