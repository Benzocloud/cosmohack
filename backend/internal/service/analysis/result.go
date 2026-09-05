package analysis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// resultVersion строит детерминированную версию результата: одинаковые
// участок, вход, модель и период дают одну и ту же неизменяемую версию.
func resultVersion(req *domain.AnalysisRequest, res *domain.AnalysisResult) string {
	h := sha256.New()
	for _, part := range []string{
		"v2", req.AreaID, req.InputRevision, res.ModelVersion, req.AnalysisPeriod.From, req.AnalysisPeriod.To,
	} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	// Peers и контекст культуры — входы анализа, но не часть ревизии снимка B1
	// Учитываем ревизию, чтобы повторный запуск с изменённым контекстом
	// получил новую неизменяемую версию результата вместо конфликта записи.
	context, _ := json.Marshal(struct {
		AreaContext *domain.AreaContext `json:"area_context,omitempty"`
		Peers       []domain.PeerSeries `json:"peers,omitempty"`
	}{AreaContext: req.AreaContext, Peers: req.Peers})
	h.Write(context)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// mapResult переводит проверенный ответ ML в доменный снимок результата.
// Интервалы и доли пригодной площади берутся из входных наблюдений, погода и
// происхождение — из собранных метаданных B1.
func mapResult(req *domain.AnalysisRequest, provenance map[string]any, res *domain.AnalysisResult) domain.AnalysisRecord {
	inputByDate := make(map[string]domain.Observation, len(req.Observations))
	for _, obs := range req.Observations {
		inputByDate[obs.Date] = obs
	}

	series := make([]domain.SeriesPoint, 0, len(res.Series))
	for _, point := range res.Series {
		mapped := point
		if obs, ok := inputByDate[point.Date]; ok {
			if obs.Interval != nil {
				mapped.Interval = &domain.Period{From: obs.Interval.From, To: obs.Interval.To}
			}
			mapped.ValidFraction = obs.ValidFraction
		}
		series = append(series, mapped)
	}

	weather := make([]domain.WeatherPoint, 0, len(req.Observations))
	for _, obs := range req.Observations {
		if obs.Weather == nil {
			continue
		}
		weather = append(weather, domain.WeatherPoint{
			Date: obs.Date, TemperatureMeanC: obs.Weather.TemperatureMeanC,
			PrecipitationSumMM: obs.Weather.PrecipitationSumMM, SourceID: &obs.Weather.SourceID,
		})
	}

	return domain.AnalysisRecord{
		ResultVersion: resultVersion(req, res), AreaID: req.AreaID, Period: req.AnalysisPeriod,
		SchemaVersion: res.SchemaVersion, FeatureProfile: res.FeatureProfile,
		ModelVersion: res.ModelVersion, Method: res.Method, Status: res.Status,
		Severity: res.Severity, InputRevision: req.InputRevision, Series: series,
		Weather: weather, Provenance: provenance, Limitations: res.Limitations, Events: res.Events,
	}
}
