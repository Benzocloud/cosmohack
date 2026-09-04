package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/handler"
	"github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

// placeholderCollector — временная заглушка сбора данных до подключения
// реального сборщика B1 (semennejo). Явно помечена: строит валидный вход без
// наблюдений, ML отвечает insufficient_data; не является рабочим сбором.
type placeholderCollector struct{}

func (placeholderCollector) Collect(_ context.Context, job store.Job, area store.Area, report analysis.StageReporter) (analysis.Collected, error) {
	report(domain.StageCollectSatellite)
	report(domain.StageCollectWeather)
	return analysis.Collected{
		Request: domain.AnalysisRequest{
			SchemaVersion:  domain.SchemaVersionV1,
			RequestID:      job.ID,
			AreaID:         area.ID,
			InputRevision:  fmt.Sprintf("input-placeholder-%s-%s-%s", area.ID, area.Period.From, area.Period.To),
			Mode:           domain.ModeRetrospective,
			FeatureProfile: domain.FeatureProfileNDVIWeatherV1,
			AnalysisPeriod: domain.Period{From: area.Period.From, To: area.Period.To},
			Sources:        []domain.Source{},
			Observations:   []domain.Observation{},
		},
		Provenance: map[string]any{"collector": "placeholder_until_b1"},
	}, nil
}

// executorQueue адаптирует исполнителя к интерфейсу Queue обработчика B3:
// переполнение очереди исполнения транслируется в контрактный сентинел
// обработчика, который даёт 429 публичного API.
type executorQueue struct {
	exec *analysis.Executor
}

func (q *executorQueue) Enqueue(ctx context.Context, jobID string) error {
	if err := q.exec.Enqueue(ctx, jobID); err != nil {
		if errors.Is(err, analysis.ErrQueueFull) {
			return handler.ErrQueueFull
		}
		return err
	}
	return nil
}

// placeholderContours — временная заглушка поиска контуров до подключения
// B1: каталог пуст, ошибки нет.
type placeholderContours struct{}

func (placeholderContours) Find(_ context.Context, _, _, _, _ float64) ([]handler.Contour, error) {
	return []handler.Contour{}, nil
}
