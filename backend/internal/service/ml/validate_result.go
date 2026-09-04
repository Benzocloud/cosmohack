package ml

import (
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// validateResult сверяет ответ ML с запросом и контрактом v1: эхо полей,
// версия модели, статусы, состав series по датам и согласованность events.
// Любое несовпадение — ml_invalid_response, результат не сохраняется.
func validateResult(req *domain.AnalysisRequest, res *domain.AnalysisResult, expectedModelVersion string) error {
	if res == nil {
		return newError(domain.MLErrorInvalidResponse, "analyze result is nil")
	}
	if err := validateEcho(req, res); err != nil {
		return err
	}
	if res.Series == nil {
		return newError(domain.MLErrorInvalidResponse, "analyze result requires a series array")
	}
	if res.ModelVersion == "" {
		return newError(domain.MLErrorInvalidResponse, "analyze result has no model_version")
	}
	if expectedModelVersion != "" && res.ModelVersion != expectedModelVersion {
		return newError(domain.MLErrorInvalidResponse, "analyze result model_version does not match the release manifest")
	}
	if res.Method == "" {
		return newError(domain.MLErrorInvalidResponse, "analyze result has no method")
	}
	if err := validateStatus(res.Status, res.Severity); err != nil {
		return err
	}
	if err := validateSeries(req, res.Series); err != nil {
		return err
	}
	return validateEvents(req, res)
}

// validateEcho проверяет неизменность корреляционных полей запроса.
func validateEcho(req *domain.AnalysisRequest, res *domain.AnalysisResult) error {
	switch {
	case res.SchemaVersion != domain.SchemaVersionV1:
		return newError(domain.MLErrorInvalidResponse, "analyze result has an unexpected schema_version")
	case res.RequestID != req.RequestID:
		return newError(domain.MLErrorInvalidResponse, "analyze result request_id does not match the request")
	case res.AreaID != req.AreaID:
		return newError(domain.MLErrorInvalidResponse, "analyze result area_id does not match the request")
	case res.InputRevision != req.InputRevision:
		return newError(domain.MLErrorInvalidResponse, "analyze result input_revision does not match the request")
	case res.Mode != req.Mode:
		return newError(domain.MLErrorInvalidResponse, "analyze result mode does not match the request")
	case res.FeatureProfile != req.FeatureProfile:
		return newError(domain.MLErrorInvalidResponse, "analyze result feature_profile does not match the request")
	}
	return nil
}

// validateStatus проверяет допустимость пары статус/тяжесть. При
// insufficient_data тяжести нет, при normal — none, иначе moderate/high.
func validateStatus(status domain.ResultStatus, severity *domain.Severity) error {
	switch status {
	case domain.StatusInsufficientData:
		if severity != nil {
			return newError(domain.MLErrorInvalidResponse, "insufficient_data result must not carry severity")
		}
	case domain.StatusNormal:
		if severity == nil || *severity != domain.SeverityNone {
			return newError(domain.MLErrorInvalidResponse, "normal result must carry severity none")
		}
	case domain.StatusCandidate, domain.StatusConfirmed:
		if severity == nil || (*severity != domain.SeverityModerate && *severity != domain.SeverityHigh) {
			return newError(domain.MLErrorInvalidResponse, fmt.Sprintf("%s result must carry moderate or high severity", status))
		}
	default:
		return newError(domain.MLErrorInvalidResponse, fmt.Sprintf("analyze result has an unknown status %q", status))
	}
	return nil
}

// expectedDates возвращает даты наблюдений внутри анализируемого периода;
// только на них ML возвращает точки series.
func expectedDates(req *domain.AnalysisRequest) []string {
	dates := make([]string, 0, len(req.Observations))
	for _, obs := range req.Observations {
		if obs.Date >= req.AnalysisPeriod.From && obs.Date <= req.AnalysisPeriod.To {
			dates = append(dates, obs.Date)
		}
	}
	return dates
}

// validateSeries проверяет состав точек: по одной на каждую входную дату
// периода в том же порядке, неизменный исходный NDVI и допустимые состояния.
func validateSeries(req *domain.AnalysisRequest, series []domain.SeriesPoint) error {
	dates := expectedDates(req)
	if len(series) != len(dates) {
		return newError(domain.MLErrorInvalidResponse, "analyze result series dates do not match the request period")
	}

	inputByDate := make(map[string]domain.Observation, len(req.Observations))
	for _, obs := range req.Observations {
		inputByDate[obs.Date] = obs
	}

	for i, point := range series {
		if point.Date != dates[i] {
			return newError(domain.MLErrorInvalidResponse, "analyze result series dates do not match the request period")
		}
		input := inputByDate[point.Date]
		if input.Quality == domain.QualityUsable && point.State != domain.StateObserved {
			return newError(domain.MLErrorInvalidResponse,
				fmt.Sprintf("usable input %q must stay observed in the result", point.Date))
		}
		if !sameFloat(point.PrimaryNDVI, input.PrimaryNDVI) {
			return newError(domain.MLErrorInvalidResponse,
				fmt.Sprintf("series point %q changed the original primary_ndvi", point.Date))
		}

		switch point.State {
		case domain.StateObserved:
			if input.Quality != domain.QualityUsable {
				return newError(domain.MLErrorInvalidResponse,
					fmt.Sprintf("series point %q is observed without a usable input", point.Date))
			}
			if point.PrimaryNDVI == nil || point.Value == nil || *point.Value != *point.PrimaryNDVI {
				return newError(domain.MLErrorInvalidResponse,
					fmt.Sprintf("observed series point %q must repeat the original value", point.Date))
			}
			if point.Method != nil {
				return newError(domain.MLErrorInvalidResponse,
					fmt.Sprintf("observed series point %q must not carry a method", point.Date))
			}
		case domain.StateImputed:
			if point.Value == nil || !nonEmptyPtr(point.Method) {
				return newError(domain.MLErrorInvalidResponse,
					fmt.Sprintf("imputed series point %q requires a value and a method", point.Date))
			}
		case domain.StateMissing:
			if point.Value != nil || point.Method != nil {
				return newError(domain.MLErrorInvalidResponse,
					fmt.Sprintf("missing series point %q must not carry a value or a method", point.Date))
			}
		default:
			return newError(domain.MLErrorInvalidResponse,
				fmt.Sprintf("series point %q has an unknown state", point.Date))
		}
	}
	return nil
}

// sameFloat сравнивает необязательные числа с учётом отсутствия значения.
func sameFloat(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// validateEvents проверяет события аномалий и сводный статус результата:
// при normal/insufficient_data событий нет, confirmed требует подтверждённого.
func validateEvents(req *domain.AnalysisRequest, res *domain.AnalysisResult) error {
	if res.Events == nil {
		return newError(domain.MLErrorInvalidResponse, "analyze result requires an events array")
	}
	observedDates := make(map[string]bool, len(res.Series))
	for _, point := range res.Series {
		if point.State == domain.StateObserved {
			observedDates[point.Date] = true
		}
	}

	hasConfirmed := false
	highestSeverity := domain.SeverityModerate
	for _, event := range res.Events {
		if event.Status != domain.StatusCandidate && event.Status != domain.StatusConfirmed {
			return newError(domain.MLErrorInvalidResponse, "event has an unknown status")
		}
		if event.Severity != domain.SeverityModerate && event.Severity != domain.SeverityHigh {
			return newError(domain.MLErrorInvalidResponse, "event must carry moderate or high severity")
		}
		if event.Severity == domain.SeverityHigh {
			highestSeverity = domain.SeverityHigh
		}
		if err := validateEventRange(event); err != nil {
			return err
		}
		if event.StartDate < req.AnalysisPeriod.From || event.EndDate > req.AnalysisPeriod.To {
			return newError(domain.MLErrorInvalidResponse, "event dates must fall inside the analysis period")
		}
		if event.EvidenceDates == nil {
			return newError(domain.MLErrorInvalidResponse, "event requires an evidence_dates array")
		}
		for _, evidence := range event.EvidenceDates {
			if !observedDates[evidence] {
				return newError(domain.MLErrorInvalidResponse,
					fmt.Sprintf("event evidence date %q is not an observed input date", evidence))
			}
		}
		if event.Facts == nil || event.Limitations == nil {
			return newError(domain.MLErrorInvalidResponse, "event requires facts and limitations arrays")
		}
		// Подтверждённое отклонение опирается на наблюдаемые основания;
		// собственная импутация ими не заменяется.
		if event.Status == domain.StatusConfirmed {
			if len(event.EvidenceDates) == 0 {
				return newError(domain.MLErrorInvalidResponse, "confirmed event requires observed evidence")
			}
			hasConfirmed = true
		}
	}

	switch res.Status {
	case domain.StatusNormal, domain.StatusInsufficientData:
		if len(res.Events) > 0 {
			return newError(domain.MLErrorInvalidResponse,
				fmt.Sprintf("%s result must not contain events", res.Status))
		}
	case domain.StatusCandidate:
		if len(res.Events) == 0 {
			return newError(domain.MLErrorInvalidResponse, "candidate result requires at least one event")
		}
		if hasConfirmed {
			return newError(domain.MLErrorInvalidResponse,
				"result with a confirmed event must be confirmed, not candidate")
		}
		if res.Severity == nil || *res.Severity != highestSeverity {
			return newError(domain.MLErrorInvalidResponse,
				"result severity must match the maximum event severity")
		}
	case domain.StatusConfirmed:
		if len(res.Events) == 0 || !hasConfirmed {
			return newError(domain.MLErrorInvalidResponse, "confirmed result requires a confirmed event")
		}
		if res.Severity == nil || *res.Severity != highestSeverity {
			return newError(domain.MLErrorInvalidResponse,
				"result severity must match the maximum event severity")
		}
	}

	if res.Limitations == nil {
		return newError(domain.MLErrorInvalidResponse, "analyze result requires a limitations array")
	}
	return nil
}

// validateEventRange проверяет даты события и порядок границ.
func validateEventRange(event domain.AnomalyEvent) error {
	start, err := parseDate(event.StartDate)
	if err != nil {
		return newError(domain.MLErrorInvalidResponse, "event start_date must be a YYYY-MM-DD date")
	}
	end, err := parseDate(event.EndDate)
	if err != nil {
		return newError(domain.MLErrorInvalidResponse, "event end_date must be a YYYY-MM-DD date")
	}
	if start.After(end) {
		return newError(domain.MLErrorInvalidResponse, "event start_date must not be after end_date")
	}
	return nil
}
