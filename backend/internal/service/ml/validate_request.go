package ml

import (
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// dateLayout — формат дат контракта YYYY-MM-DD (UTC).
const dateLayout = "2006-01-02"

// validateRequest проверяет запрос до отправки в ML: идентификаторы, режим,
// период, источники и наблюдения по правилам контракта v1. Нарушение —
// ml_invalid_request: некорректный запрос не покидает Go.
func validateRequest(req *domain.AnalysisRequest) error {
	if req == nil {
		return newError(domain.MLErrorInvalidRequest, "analyze request is nil")
	}
	if req.SchemaVersion != domain.SchemaVersionV1 {
		return newError(domain.MLErrorInvalidRequest, "analyze request schema_version must be 1.0")
	}
	if err := validateID("request_id", req.RequestID); err != nil {
		return err
	}
	if err := validateID("area_id", req.AreaID); err != nil {
		return err
	}
	if err := validateID("input_revision", req.InputRevision); err != nil {
		return err
	}
	if req.Mode != domain.ModeRetrospective {
		return newError(domain.MLErrorInvalidRequest, "analyze request mode must be retrospective")
	}
	if req.FeatureProfile != domain.FeatureProfileNDVIWeatherV1 {
		return newError(domain.MLErrorInvalidRequest, "analyze request feature_profile must be ndvi-weather-v1")
	}
	if err := validatePeriod(req.AnalysisPeriod); err != nil {
		return err
	}
	if err := validateSources(req.Sources); err != nil {
		return err
	}
	return validateObservations(req)
}

// validateID проверяет непустой идентификатор с ограничением длины контракта.
func validateID(field, value string) error {
	if value == "" {
		return newError(domain.MLErrorInvalidRequest, fmt.Sprintf("%s must not be empty", field))
	}
	if len(value) > domain.MaxIDLength {
		return newError(domain.MLErrorInvalidRequest, fmt.Sprintf("%s must not exceed %d characters", field, domain.MaxIDLength))
	}
	return nil
}

// parseDate разбирает дату YYYY-MM-DD.
func parseDate(value string) (time.Time, error) {
	return time.Parse(dateLayout, value)
}

// validatePeriod проверяет даты анализируемого периода и порядок границ;
// границы включены.
func validatePeriod(period domain.Period) error {
	from, err := parseDate(period.From)
	if err != nil {
		return newError(domain.MLErrorInvalidRequest, "analysis_period.from must be a YYYY-MM-DD date")
	}
	to, err := parseDate(period.To)
	if err != nil {
		return newError(domain.MLErrorInvalidRequest, "analysis_period.to must be a YYYY-MM-DD date")
	}
	if from.After(to) {
		return newError(domain.MLErrorInvalidRequest, "analysis_period.from must not be after analysis_period.to")
	}
	return nil
}

// validateSources проверяет ссылки, типы и метаданные источников запроса.
func validateSources(sources []domain.Source) error {
	seen := make(map[string]bool, len(sources))
	for _, src := range sources {
		if err := validateID("source.id", src.ID); err != nil {
			return err
		}
		if seen[src.ID] {
			return newError(domain.MLErrorInvalidRequest, fmt.Sprintf("source id %q is duplicated", src.ID))
		}
		seen[src.ID] = true

		switch src.Kind {
		case domain.SourceSatellite, domain.SourceWeather, domain.SourceReference:
		default:
			return newError(domain.MLErrorInvalidRequest, fmt.Sprintf("source %q has an unknown kind", src.ID))
		}
		if src.Provider == "" || src.Dataset == "" {
			return newError(domain.MLErrorInvalidRequest, fmt.Sprintf("source %q requires provider and dataset", src.ID))
		}
		if src.Mapping == "" {
			return newError(domain.MLErrorInvalidRequest, fmt.Sprintf("source %q requires a mapping", src.ID))
		}
		retrievedAt, err := time.Parse(time.RFC3339, src.RetrievedAt)
		if err != nil {
			return newError(domain.MLErrorInvalidRequest, fmt.Sprintf("source %q retrieved_at must be RFC3339", src.ID))
		}
		if _, offset := retrievedAt.Zone(); offset != 0 {
			return newError(domain.MLErrorInvalidRequest, fmt.Sprintf("source %q retrieved_at must be UTC", src.ID))
		}
	}
	return nil
}

// sourceKinds строит карту id → тип источника для проверки ссылок.
func sourceKinds(sources []domain.Source) map[string]domain.SourceKind {
	kinds := make(map[string]domain.SourceKind, len(sources))
	for _, src := range sources {
		kinds[src.ID] = src.Kind
	}
	return kinds
}

// validateObservations проверяет лимит, порядок дат и правила качества точек.
func validateObservations(req *domain.AnalysisRequest) error {
	if len(req.Observations) > domain.MaxObservationsPerRequest {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observations exceed the limit of %d points", domain.MaxObservationsPerRequest))
	}
	kinds := sourceKinds(req.Sources)

	var prev string
	for _, obs := range req.Observations {
		date, err := parseDate(obs.Date)
		if err != nil {
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("observation date %q must be YYYY-MM-DD", obs.Date))
		}
		if obs.Date <= prev {
			return newError(domain.MLErrorInvalidRequest, "observation dates must be unique and strictly ascending")
		}
		prev = obs.Date

		switch obs.Quality {
		case domain.QualityUsable, domain.QualityUnusable:
			if err := validateNumericObservation(obs, kinds, date); err != nil {
				return err
			}
			// Отбракованное наблюдение сохраняет числовое значение, источник
			// и интервал, но требует причину отбраковки.
			if obs.Quality == domain.QualityUnusable && !nonEmptyPtr(obs.MissingReason) {
				return newError(domain.MLErrorInvalidRequest,
					fmt.Sprintf("unusable observation %q requires missing_reason", obs.Date))
			}
		case domain.QualityMissing:
			if obs.PrimaryNDVI != nil || obs.NDVISourceID != nil || obs.Interval != nil {
				return newError(domain.MLErrorInvalidRequest,
					fmt.Sprintf("missing observation %q must not carry ndvi, source or interval", obs.Date))
			}
			if !nonEmptyPtr(obs.MissingReason) {
				return newError(domain.MLErrorInvalidRequest,
					fmt.Sprintf("missing observation %q requires missing_reason", obs.Date))
			}
		default:
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("observation %q has an unknown quality", obs.Date))
		}

		if obs.ValidFraction != nil && (*obs.ValidFraction < 0 || *obs.ValidFraction > 1) {
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("observation %q valid_fraction must be between 0 and 1", obs.Date))
		}
		if err := validateWeather(obs, kinds); err != nil {
			return err
		}
		if err := validateReference(obs, kinds); err != nil {
			return err
		}
	}
	return nil
}

// validateNumericObservation проверяет точку с числовым NDVI: источник
// satellite, интервал, внутри которого лежит дата наблюдения.
func validateNumericObservation(obs domain.Observation, kinds map[string]domain.SourceKind, date time.Time) error {
	if obs.PrimaryNDVI == nil {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q with numeric quality requires primary_ndvi", obs.Date))
	}
	if obs.NDVISourceID == nil || kinds[*obs.NDVISourceID] != domain.SourceSatellite {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q requires a satellite ndvi_source_id", obs.Date))
	}
	if obs.Interval == nil {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q requires an interval", obs.Date))
	}
	from, err := parseDate(obs.Interval.From)
	if err != nil {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q interval.from must be a YYYY-MM-DD date", obs.Date))
	}
	to, err := parseDate(obs.Interval.To)
	if err != nil {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q interval.to must be a YYYY-MM-DD date", obs.Date))
	}
	if from.After(to) {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q interval.from must not be after interval.to", obs.Date))
	}
	if date.Before(from) || date.After(to) {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q date must fall inside its interval", obs.Date))
	}
	return nil
}

// validateWeather проверяет погодный контекст: ссылка на источник weather,
// неотрицательные осадки; отсутствие значений кодируется null.
func validateWeather(obs domain.Observation, kinds map[string]domain.SourceKind) error {
	if obs.Weather == nil {
		return nil
	}
	if kinds[obs.Weather.SourceID] != domain.SourceWeather {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q weather references a non-weather source", obs.Date))
	}
	if obs.Weather.PrecipitationSumMM != nil && *obs.Weather.PrecipitationSumMM < 0 {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q weather precipitation_sum_mm must not be negative", obs.Date))
	}
	return nil
}

// validateReference проверяет сезонный фон: источник reference, неотрицательная
// дисперсия, положительное число лет и исключение целевого года.
func validateReference(obs domain.Observation, kinds map[string]domain.SourceKind) error {
	if obs.Reference == nil {
		return nil
	}
	ref := obs.Reference
	if kinds[ref.SourceID] != domain.SourceReference {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q reference references a non-reference source", obs.Date))
	}
	if ref.Std < 0 {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q reference std must not be negative", obs.Date))
	}
	if ref.NReferenceYears <= 0 {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q reference requires a positive n_reference_years", obs.Date))
	}
	if !ref.TargetYearExcluded {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q reference must exclude the target year", obs.Date))
	}
	return nil
}

func nonEmptyPtr(value *string) bool {
	return value != nil && *value != ""
}
