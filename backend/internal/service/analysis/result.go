package analysis

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

// resultVersion строит детерминированную версию результата: одинаковые
// участок, вход, модель и период дают одну и ту же неизменяемую версию.
func resultVersion(req *domain.AnalysisRequest, res *domain.AnalysisResult) string {
	h := sha256.New()
	for _, part := range []string{
		"v1", req.AreaID, req.InputRevision, res.ModelVersion, req.AnalysisPeriod.From, req.AnalysisPeriod.To,
	} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// mapResult переводит проверенный ответ ML в публичный снимок результата:
// интервалы и доли пригодной площади берутся из входных наблюдений, погода
// и происхождение — из собранных метаданных B1.
func mapResult(req *domain.AnalysisRequest, provenance map[string]any, res *domain.AnalysisResult) store.Result {
	inputByDate := make(map[string]domain.Observation, len(req.Observations))
	for _, obs := range req.Observations {
		inputByDate[obs.Date] = obs
	}

	series := make([]store.SeriesPoint, 0, len(res.Series))
	for _, point := range res.Series {
		sp := store.SeriesPoint{
			Date:        point.Date,
			PrimaryNDVI: point.PrimaryNDVI,
			Value:       point.Value,
			State:       string(point.State),
			Method:      point.Method,
			Baseline:    point.Baseline,
			ZScore:      point.ZScore,
		}
		if obs, ok := inputByDate[point.Date]; ok {
			if obs.Interval != nil {
				sp.Interval = &store.Period{From: obs.Interval.From, To: obs.Interval.To}
			}
			sp.ValidFraction = obs.ValidFraction
		}
		series = append(series, sp)
	}

	weather := make([]store.WeatherPoint, 0, len(req.Observations))
	for _, obs := range req.Observations {
		if obs.Weather == nil {
			continue
		}
		weather = append(weather, store.WeatherPoint{
			Date:               obs.Date,
			TemperatureMeanC:   obs.Weather.TemperatureMeanC,
			PrecipitationSumMM: obs.Weather.PrecipitationSumMM,
			SourceID:           &obs.Weather.SourceID,
		})
	}

	events := make([]store.Event, 0, len(res.Events))
	for _, event := range res.Events {
		events = append(events, store.Event{
			StartDate:     event.StartDate,
			EndDate:       event.EndDate,
			Status:        string(event.Status),
			Severity:      string(event.Severity),
			MinZScore:     event.MinZScore,
			EvidenceDates: event.EvidenceDates,
			Facts:         event.Facts,
			Hypothesis:    event.Hypothesis,
			Limitations:   event.Limitations,
		})
	}

	severity := stringPtr(res.Severity)
	return store.Result{
		ResultVersion:  resultVersion(req, res),
		Period:         store.Period{From: req.AnalysisPeriod.From, To: req.AnalysisPeriod.To},
		SchemaVersion:  res.SchemaVersion,
		FeatureProfile: res.FeatureProfile,
		ModelVersion:   res.ModelVersion,
		Method:         res.Method,
		Status:         string(res.Status),
		Severity:       severity,
		Series:         series,
		Weather:        weather,
		Provenance:     provenance,
		Limitations:    res.Limitations,
		Events:         events,
	}
}

func stringPtr(value *domain.Severity) *string {
	if value == nil {
		return nil
	}
	s := string(*value)
	return &s
}
