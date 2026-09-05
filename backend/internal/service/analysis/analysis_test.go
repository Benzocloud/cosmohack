package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/service/ml"
)

const (
	testAreaID  = "area-00000000-0000-0000-0000-000000000001"
	testJobID   = "job-00000000-0000-0000-0000-000000000001"
	testArea2ID = "area-00000000-0000-0000-0000-000000000002"
	testJob2ID  = "job-00000000-0000-0000-0000-000000000002"
)

// waitFor крутит условие до успеха или тайм-аута; воркер асинхронный.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not reached in time")
}

// newExecutorPersistence creates an in-memory persistence fake and seeds an area.
func newExecutorPersistence(t *testing.T) *testPersistence {
	t.Helper()
	st := newTestPersistence()
	if err := st.CreateArea(testArea(testAreaID)); err != nil {
		t.Fatalf("put area: %v", err)
	}
	return st
}

func testArea(id string) domain.Area {
	return domain.Area{
		ID:        id,
		Name:      "test area",
		Geometry:  domain.Polygon{Type: "Polygon", Coordinates: [][][]float64{{{30, 50}, {31, 50}, {31, 51}, {30, 50}}}},
		Period:    domain.Period{From: "2026-05-01", To: "2026-05-01"},
		CreatedAt: time.Now().UTC(),
	}
}

// enqueueJob создаёт queued-задачу через публичные операции хранилища.
func enqueueJob(t *testing.T, st *testPersistence, areaID, jobID string) {
	t.Helper()
	area, err := st.getArea(areaID)
	if err != nil {
		t.Fatalf("get area: %v", err)
	}
	now := time.Now().UTC()
	job := domain.Job{
		ID:             jobID,
		AreaID:         areaID,
		Period:         area.Period,
		CreatedAt:      now,
		UpdatedAt:      now,
		AreaGeneration: area.Generation,
	}
	if err := st.putJobQueued(job); err != nil {
		t.Fatalf("put job queued: %v", err)
	}
}

// fixtureRequest — минимальный валидный запрос: одна usable-точка с погодой
// и reference (см. правила validateRequest).
func fixtureRequest(jobID string) domain.AnalysisRequest {
	satellite := "src-satellite-1"
	weatherSrc := "src-weather-1"
	referenceSrc := "src-reference-1"
	ndvi := 0.5
	empty := ""
	return domain.AnalysisRequest{
		SchemaVersion:  domain.SchemaVersionV1,
		RequestID:      jobID,
		AreaID:         testAreaID,
		InputRevision:  "input-" + jobID,
		Mode:           domain.ModeRetrospective,
		FeatureProfile: domain.FeatureProfileNDVIWeatherV1,
		AnalysisPeriod: domain.Period{From: "2026-05-01", To: "2026-05-01"},
		Sources: []domain.Source{
			{ID: satellite, Kind: domain.SourceSatellite, Provider: "cdse", Dataset: "S2L2A", Mapping: json.RawMessage(`{}`), RetrievedAt: "2026-08-01T10:00:00Z"},
			{ID: weatherSrc, Kind: domain.SourceWeather, Provider: "open-meteo", Dataset: "ERA5", Mapping: json.RawMessage(`{}`), RetrievedAt: "2026-08-01T10:00:00Z"},
			{ID: referenceSrc, Kind: domain.SourceReference, Provider: "team", Dataset: "baseline", Mapping: json.RawMessage(`{}`), RetrievedAt: "2026-08-01T10:00:00Z"},
		},
		Observations: []domain.Observation{{
			Date:          "2026-05-01",
			PrimaryNDVI:   &ndvi,
			Quality:       domain.QualityUsable,
			NDVISourceID:  &satellite,
			Interval:      &domain.Interval{From: "2026-04-29", To: "2026-05-03"},
			ValidFraction: ptrFloat(0.9),
			MissingReason: &empty,
			Weather:       &domain.Weather{SourceID: weatherSrc, TemperatureMeanC: ptrFloat(18.0), PrecipitationSumMM: ptrFloat(1.0)},
			Reference:     &domain.Reference{SourceID: referenceSrc, Mean: 0.48, Std: 0.1, NReferenceYears: 7, TargetYearExcluded: true},
		}},
	}
}

func ptrFloat(v float64) *float64 { return &v }

// fixtureResponse — валидный успешный ответ на fixtureRequest: эхо полей
// запроса, одна observed-точка, статус normal.
func fixtureResponse(req *domain.AnalysisRequest) domain.AnalysisResult {
	ndvi := *req.Observations[0].PrimaryNDVI
	baseline := 0.48
	z := 0.2
	return domain.AnalysisResult{
		SchemaVersion:  domain.SchemaVersionV1,
		RequestID:      req.RequestID,
		AreaID:         req.AreaID,
		InputRevision:  req.InputRevision,
		Mode:           req.Mode,
		FeatureProfile: req.FeatureProfile,
		ModelVersion:   "model-fixture-1",
		Method:         "baseline",
		Status:         domain.StatusNormal,
		Severity:       sevPtr(domain.SeverityNone),
		Series: []domain.SeriesPoint{{
			Date:        req.Observations[0].Date,
			PrimaryNDVI: &ndvi,
			Value:       &ndvi,
			State:       domain.StateObserved,
			Baseline:    &baseline,
			ZScore:      &z,
		}},
		Events:      []domain.AnomalyEvent{},
		Limitations: []string{},
	}
}

func sevPtr(s domain.Severity) *domain.Severity { return &s }

// stubCollector отдаёт фикстурный запрос, пишет сообщённые стадии и умеет
// возвращать ошибку источника.
type stubCollector struct {
	mu     sync.Mutex
	stages []string
	err    error
}

func (c *stubCollector) Collect(_ context.Context, job domain.Job, _ domain.Area, report StageReporter) (Collected, error) {
	report(domain.StageCollectSatellite)
	report(domain.StageCollectWeather)
	c.mu.Lock()
	c.stages = append(c.stages, domain.StageCollectSatellite, domain.StageCollectWeather)
	c.mu.Unlock()
	if c.err != nil {
		return Collected{}, c.err
	}
	return Collected{Request: fixtureRequest(job.ID), Provenance: map[string]any{"sources": 3}}, nil
}

func (c *stubCollector) reportedStages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.stages...)
}

// okAnalyzer всегда возвращает валидный ответ на переданный запрос.
type okAnalyzer struct{}

func (okAnalyzer) Analyze(_ context.Context, req *domain.AnalysisRequest) (*domain.AnalysisResult, error) {
	out := fixtureResponse(req)
	return &out, nil
}

// mlErrorAnalyzer возвращает ошибку, собранную настоящим клиентом ML
// против тестового HTTP-сервера: сохраняет точное поведение контракта.
type mlErrorAnalyzer struct {
	err error
}

func (m mlErrorAnalyzer) Analyze(context.Context, *domain.AnalysisRequest) (*domain.AnalysisResult, error) {
	return nil, m.err
}

// analyzeErrorAgainstServer прогоняет фикстурный запрос через настоящий
// клиент ml против тестового сервера, чтобы получить каноническую ошибку.
func analyzeErrorAgainstServer(t *testing.T, status int, body string) error {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	client, err := ml.New(ml.DefaultConfig(server.URL))
	if err != nil {
		t.Fatalf("new ml client: %v", err)
	}
	req := fixtureRequest(testJobID)
	_, err = client.Analyze(context.Background(), &req)
	if err == nil {
		t.Fatal("expected analyze error")
	}
	return err
}

// blockingAnalyzer держит вызов до release, чтобы протестировать отмену.
type blockingAnalyzer struct {
	entered chan struct{}
	release chan struct{}
}

func (b *blockingAnalyzer) Analyze(ctx context.Context, req *domain.AnalysisRequest) (*domain.AnalysisResult, error) {
	close(b.entered)
	select {
	case <-b.release:
		out := fixtureResponse(req)
		return &out, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestExecutor_SuccessPath(t *testing.T) {
	st := newExecutorPersistence(t)

	collector := &stubCollector{}
	exec := New(st, collector, okAnalyzer{}, 8)
	exec.Start(context.Background())
	enqueueJob(t, st, testAreaID, testJobID)
	if err := exec.Enqueue(context.Background(), testJobID); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	var completed domain.Job
	waitFor(t, func() bool {
		job, err := st.getJob(testJobID)
		if err != nil {
			return false
		}
		if job.Status == domain.JobCompleted {
			completed = job
			return true
		}
		return false
	})
	if completed.ResultVersion == nil {
		t.Fatal("completed job must carry result_version")
	}
	result, err := st.getResult(testAreaID, *completed.ResultVersion)
	if err != nil {
		t.Fatalf("get result: %v", err)
	}
	if result.Status != domain.StatusNormal || result.ModelVersion != "model-fixture-1" {
		t.Fatalf("unexpected result: %s/%s", result.Status, result.ModelVersion)
	}
	if len(result.Series) != 1 || result.Series[0].Interval == nil || result.Series[0].ValidFraction == nil {
		t.Fatalf("series mapping lost input metadata: %+v", result.Series)
	}
	if len(result.Weather) != 1 || result.Weather[0].TemperatureMeanC == nil {
		t.Fatalf("weather mapping lost: %+v", result.Weather)
	}
	if got, ok := result.Provenance["sources"].(int); !ok || got != 3 {
		t.Fatalf("provenance lost: %v", result.Provenance)
	}
	if result.AreaID != testAreaID {
		t.Fatalf("result area correlation lost: %s", result.AreaID)
	}

	stages := collector.reportedStages()
	if len(stages) != 2 || stages[0] != domain.StageCollectSatellite || stages[1] != domain.StageCollectWeather {
		t.Fatalf("collector stages lost: %v", stages)
	}
	if completed.Stage != nil {
		t.Fatalf("terminal job must clear stage, got %q", *completed.Stage)
	}
	if jobStillActive(st, testAreaID, testJobID) {
		t.Fatal("completed job must clear active_job_id")
	}
}

func TestExecutor_QueueUsesConfiguredCapacity(t *testing.T) {
	st := newExecutorPersistence(t)
	exec := New(st, &stubCollector{}, okAnalyzer{}, 2)

	for range 2 {
		if err := exec.Enqueue(context.Background(), testJobID); err != nil {
			t.Fatalf("enqueue within limit: %v", err)
		}
	}
	if err := exec.Enqueue(context.Background(), testJob2ID); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("want ErrQueueFull, got %v", err)
	}
}

func TestExecutor_SourceError(t *testing.T) {
	st := newExecutorPersistence(t)

	collector := &stubCollector{err: errors.New("provider unavailable")}
	exec := New(st, collector, okAnalyzer{}, 8)
	exec.Start(context.Background())
	enqueueJob(t, st, testAreaID, testJobID)
	if err := exec.Enqueue(context.Background(), testJobID); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	var failed domain.Job
	waitFor(t, func() bool {
		job, err := st.getJob(testJobID)
		if err != nil {
			return false
		}
		if job.Status == domain.JobFailed {
			failed = job
			return true
		}
		return false
	})
	if failed.ErrorCode == nil || *failed.ErrorCode != "source_failed" || failed.ErrorMessage == nil || *failed.ErrorMessage != "provider unavailable" {
		t.Fatalf("unexpected job error: %s/%s", valueOrEmpty(failed.ErrorCode), valueOrEmpty(failed.ErrorMessage))
	}
	if jobStillActive(st, testAreaID, testJobID) {
		t.Fatal("failed job must clear active_job_id")
	}
}

func TestExecutor_BusyML(t *testing.T) {
	st := newExecutorPersistence(t)

	analyzeErr := analyzeErrorAgainstServer(t, http.StatusTooManyRequests,
		`{"schema_version":"1.0","request_id":"`+testJobID+`","error":{"code":"busy","message":"ML busy","retryable":true}}`)
	exec := New(st, &stubCollector{}, mlErrorAnalyzer{err: analyzeErr}, 8)
	exec.Start(context.Background())
	enqueueJob(t, st, testAreaID, testJobID)
	if err := exec.Enqueue(context.Background(), testJobID); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	var failed domain.Job
	waitFor(t, func() bool {
		job, err := st.getJob(testJobID)
		if err != nil {
			return false
		}
		if job.Status == domain.JobFailed {
			failed = job
			return true
		}
		return false
	})
	if failed.ErrorCode == nil || *failed.ErrorCode != string(domain.MLErrorBusy) || failed.ErrorRetryable == nil || !*failed.ErrorRetryable {
		t.Fatalf("want ml_busy retryable, got %s", valueOrEmpty(failed.ErrorCode))
	}
	if failed.ErrorMessage == nil || *failed.ErrorMessage != "ML busy" {
		t.Fatalf("ml message lost: %s", valueOrEmpty(failed.ErrorMessage))
	}
}

func TestExecutor_CancelDuringAnalyze(t *testing.T) {
	st := newExecutorPersistence(t)

	blocker := &blockingAnalyzer{entered: make(chan struct{}), release: make(chan struct{})}
	exec := New(st, &stubCollector{}, blocker, 8)
	exec.Start(context.Background())
	enqueueJob(t, st, testAreaID, testJobID)
	if err := exec.Enqueue(context.Background(), testJobID); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitFor(t, func() bool {
		job, err := st.getJob(testJobID)
		return err == nil && job.Status == domain.JobRunning && job.Stage != nil && *job.Stage == domain.StageAnalyze
	})

	exec.Cancel(testJobID)

	var cancelled domain.Job
	waitFor(t, func() bool {
		job, err := st.getJob(testJobID)
		if err != nil {
			return false
		}
		if job.Status == domain.JobCancelled {
			cancelled = job
			return true
		}
		return false
	})
	if cancelled.ResultVersion != nil {
		t.Fatal("cancelled job must not carry a result version")
	}

	// Поздний ответ ML не сохраняется.
	close(blocker.release)
	time.Sleep(100 * time.Millisecond)
	job, err := st.getJob(testJobID)
	if err != nil || job.Status != domain.JobCancelled || job.ResultVersion != nil {
		t.Fatalf("late save must not happen: %+v/%v", job, err)
	}
}

func TestExecutor_CancelPending(t *testing.T) {
	st := newExecutorPersistence(t)
	if err := st.CreateArea(testArea(testArea2ID)); err != nil {
		t.Fatalf("put second area: %v", err)
	}
	enqueueJob(t, st, testAreaID, testJobID)
	enqueueJob(t, st, testArea2ID, testJob2ID)

	blocker := &blockingAnalyzer{entered: make(chan struct{}), release: make(chan struct{})}
	exec := New(st, &stubCollector{}, blocker, 8)
	exec.Start(context.Background())
	enqueueJob(t, st, testAreaID, testJobID)
	if err := exec.Enqueue(context.Background(), testJobID); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	// Воркер занят первой задачей: вторая ждёт в очереди.
	waitFor(t, func() bool {
		job, err := st.getJob(testJobID)
		return err == nil && job.Status == domain.JobRunning && job.Stage != nil && *job.Stage == domain.StageAnalyze
	})
	enqueueJob(t, st, testArea2ID, testJob2ID)
	if err := exec.Enqueue(context.Background(), testJob2ID); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}
	exec.Cancel(testJob2ID)

	waitFor(t, func() bool {
		job, err := st.getJob(testJob2ID)
		return err == nil && job.Status == domain.JobCancelled
	})
	close(blocker.release)
	waitFor(t, func() bool {
		job, err := st.getJob(testJobID)
		return err == nil && job.Status == domain.JobCompleted
	})
}

func TestExecutor_RestartMarksInterrupted(t *testing.T) {
	st := newExecutorPersistence(t)
	enqueueJob(t, st, testAreaID, testJobID)

	// A fresh executor recovers unfinished jobs through its persistence port.
	New(st, &stubCollector{}, okAnalyzer{}, 8).Start(context.Background())

	var failed domain.Job
	waitFor(t, func() bool {
		job, err := st.getJob(testJobID)
		if err != nil {
			return false
		}
		if job.Status == domain.JobFailed {
			failed = job
			return true
		}
		return false
	})
	if failed.ErrorCode == nil || *failed.ErrorCode != "interrupted" {
		t.Fatalf("want interrupted code, got %s", valueOrEmpty(failed.ErrorCode))
	}
}

func TestExecutor_ResultVersionDeterministic(t *testing.T) {
	req := fixtureRequest(testJobID)
	res := fixtureResponse(&req)

	version := resultVersion(&req, &res)
	if version != resultVersion(&req, &res) {
		t.Fatal("same input and model must produce the same result version")
	}
	other := res
	other.ModelVersion = "model-other"
	if resultVersion(&req, &res) == resultVersion(&req, &other) {
		t.Fatal("different model versions must produce different result versions")
	}
}

func jobStillActive(st *testPersistence, areaID, jobID string) bool {
	area, err := st.getArea(areaID)
	if err != nil {
		return false
	}
	return area.ActiveJobID == jobID
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
