package ml

import (
	"fmt"
	"math"
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
	switch req.SchemaVersion {
	case domain.SchemaVersionV1:
		if req.FeatureProfile == domain.FeatureProfileMultisensorV1 {
			return newError(domain.MLErrorContractMismatch,
				"ndvi-multisensor-v1 requires schema_version 1.1")
		}
	case domain.SchemaVersionV11:
	default:
		return newError(domain.MLErrorContractMismatch, "analyze request has an unsupported schema_version")
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
	if req.FeatureProfile != domain.FeatureProfileNDVIWeatherV1 &&
		req.FeatureProfile != domain.FeatureProfileMultisensorV1 {
		return newError(domain.MLErrorInvalidRequest, "analyze request has an unsupported feature_profile")
	}
	if req.FeatureProfile == domain.FeatureProfileNDVIWeatherV1 && hasMultisensorFields(req) {
		return newError(domain.MLErrorInvalidRequest,
			"indices, area_context and peers require ndvi-multisensor-v1")
	}
	if err := validatePeriod(req.AnalysisPeriod); err != nil {
		return err
	}
	if err := validateSources(req.Sources); err != nil {
		return err
	}
	if err := validateObservations(req); err != nil {
		return err
	}
	if req.FeatureProfile != domain.FeatureProfileMultisensorV1 {
		return nil
	}
	return validateMultisensorContext(req)
}

func hasMultisensorFields(req *domain.AnalysisRequest) bool {
	if req.AreaContext != nil || req.Peers != nil {
		return true
	}
	for _, obs := range req.Observations {
		if obs.Indices != nil {
			return true
		}
	}
	return false
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

		if obs.ValidFraction != nil &&
			(!finite(*obs.ValidFraction) || *obs.ValidFraction < 0 || *obs.ValidFraction > 1) {
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("observation %q valid_fraction must be between 0 and 1", obs.Date))
		}
		if err := validateIndices(obs.Indices, obs.Date); err != nil {
			return err
		}
		if err := validatePrimaryIndices(obs.PrimaryNDVI, obs.Indices, obs.Date); err != nil {
			return err
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
	if !finite(*obs.PrimaryNDVI) {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q primary_ndvi must be finite", obs.Date))
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
	if obs.Weather.TemperatureMeanC != nil && !finite(*obs.Weather.TemperatureMeanC) {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q weather temperature_mean_c must be finite", obs.Date))
	}
	if obs.Weather.PrecipitationSumMM != nil && !finite(*obs.Weather.PrecipitationSumMM) {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q weather precipitation_sum_mm must be finite", obs.Date))
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
	if !finite(ref.Mean) || !finite(ref.Std) {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q reference values must be finite", obs.Date))
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

const ndviMatchTolerance = 1e-9

func validateIndices(indices *domain.Indices, date string) error {
	if indices == nil {
		return nil
	}
	values := []*float64{
		indices.S2NDVI,
		indices.S2EVI,
		indices.S2NDWI,
		indices.LandsatNDVI,
		indices.LandsatEVI,
		indices.LandsatNDWI,
		indices.MODISNDVI,
		indices.MODISEVI,
	}
	for _, value := range values {
		if value != nil && !finite(*value) {
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("observation %q indices must contain finite numbers", date))
		}
	}
	return nil
}

func validatePrimaryIndices(primary *float64, indices *domain.Indices, date string) error {
	if indices == nil {
		return nil
	}
	expected := indices.Primary()
	if primary == nil && expected != nil {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q has indices but no primary_ndvi", date))
	}
	if primary != nil && expected == nil {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q has primary_ndvi but no index", date))
	}
	if primary != nil && expected != nil && math.Abs(*primary-*expected) > ndviMatchTolerance {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("observation %q primary_ndvi does not match the first available index", date))
	}
	return nil
}

func validateMultisensorContext(req *domain.AnalysisRequest) error {
	if len(req.Peers) > domain.MaxPeers {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("peers exceed the limit of %d areas", domain.MaxPeers))
	}
	peerIDs := make(map[string]bool, len(req.Peers))
	totalPoints := len(req.Observations)
	for _, peer := range req.Peers {
		if err := validateID("peer.area_id", peer.AreaID); err != nil {
			return err
		}
		if peerIDs[peer.AreaID] || peer.AreaID == req.AreaID {
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("peer area id %q is duplicated or is the analyzed area", peer.AreaID))
		}
		peerIDs[peer.AreaID] = true
		if len(peer.Observations) > domain.MaxObservationsPerRequest {
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("peer %q observations exceed the limit of %d points", peer.AreaID, domain.MaxObservationsPerRequest))
		}
		totalPoints += len(peer.Observations)
		if err := validatePeerObservations(peer); err != nil {
			return err
		}
	}
	if totalPoints > domain.MaxTotalPoints {
		return newError(domain.MLErrorInvalidRequest,
			fmt.Sprintf("total observations exceed the limit of %d points", domain.MaxTotalPoints))
	}
	return nil
}

func validatePeerObservations(peer domain.PeerSeries) error {
	var previous string
	for _, obs := range peer.Observations {
		if _, err := parseDate(obs.Date); err != nil {
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("peer observation date %q must be YYYY-MM-DD", obs.Date))
		}
		if obs.Date <= previous {
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("peer %q observation dates must be unique and strictly ascending", peer.AreaID))
		}
		previous = obs.Date
		switch obs.Quality {
		case domain.QualityUsable, domain.QualityUnusable, domain.QualityMissing, "":
		default:
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("peer observation %q has an unknown quality", obs.Date))
		}
		if obs.PrimaryNDVI != nil && !finite(*obs.PrimaryNDVI) {
			return newError(domain.MLErrorInvalidRequest,
				fmt.Sprintf("peer observation %q primary_ndvi must be finite", obs.Date))
		}
		if err := validateIndices(obs.Indices, obs.Date); err != nil {
			return err
		}
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func nonEmptyPtr(value *string) bool {
	return value != nil && *value != ""
}
