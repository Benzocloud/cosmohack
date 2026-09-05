package handler_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/handler"
)

func TestAnalyses202And409(t *testing.T) {
	q := handler.NewStubQueue(8)
	h, st := newEnv(t, nil, q)
	id := createArea(t, h)

	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var body struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &body)
	if body.JobID == "" {
		t.Fatal("no job_id")
	}

	w = doJSON(t, h, http.MethodGet, "/api/jobs/"+body.JobID, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var job map[string]any
	decode(t, w, &job)
	if job["status"] != "queued" && job["status"] != "running" {
		t.Fatalf("%s", w.Body.String())
	}
	if job["status"] == "queued" && job["stage"] != nil {
		t.Fatalf("queued stage %s", w.Body.String())
	}
	if err := st.SetJobRunning(body.JobID, "satellite"); err != nil {
		t.Fatal(err)
	}
	w = doJSON(t, h, http.MethodGet, "/api/jobs/"+body.JobID, nil)
	decode(t, w, &job)
	if job["status"] != "running" || job["stage"] != "satellite" {
		t.Fatalf("running %s", w.Body.String())
	}

	w = doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", []byte(`{}`))
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 got %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Error struct {
			Code  string `json:"code"`
			JobID string `json:"job_id"`
		} `json:"error"`
	}
	decode(t, w, &env)
	if env.Error.Code != "conflict" || env.Error.JobID != "" {
		t.Fatalf("%s", w.Body.String())
	}
}

func TestAnalysesEmptyBodyNoContentType(t *testing.T) {
	h, _ := newEnv(t, nil, nil)
	id := createArea(t, h)
	w := doReq(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil, "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var body struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &body)
	w = doJSON(t, h, http.MethodGet, "/api/jobs/"+body.JobID, nil)
	var job map[string]any
	decode(t, w, &job)
	p := job["period"].(map[string]any)
	if p["from"] != "2024-04-01" || p["to"] != "2024-09-30" {
		t.Fatalf("period %s", w.Body.String())
	}
}

func TestAnalysesBadPeriod(t *testing.T) {
	h, _ := newEnv(t, nil, nil)
	id := createArea(t, h)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", []byte(`{"period":{"from":"2024-12-01","to":"2024-01-01"}}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestAnalyses404(t *testing.T) {
	h, st := newEnv(t, nil, nil)
	w := doJSON(t, h, http.MethodPost, "/api/areas/aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee/analyses", nil)
	if w.Code != 404 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	jobs, err := st.ListJobsByArea("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("jobs=%d", len(jobs))
	}
}

func TestQueueFull(t *testing.T) {
	q := handler.NewStubQueue(8)
	h, st := newEnv(t, nil, q)
	var ids []string
	for i := 0; i < 8; i++ {
		ids = append(ids, createShiftedArea(t, h, i))
		w := doJSON(t, h, http.MethodPost, "/api/areas/"+ids[i]+"/analyses", nil)
		if w.Code != http.StatusAccepted {
			t.Fatalf("i=%d %d %s", i, w.Code, w.Body.String())
		}
	}
	ninth := createShiftedArea(t, h, 8)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+ninth+"/analyses", nil)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var env struct {
		Error struct {
			Code      string `json:"code"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	decode(t, w, &env)
	if env.Error.Code != "queue_full" || !env.Error.Retryable {
		t.Fatalf("%s", w.Body.String())
	}
	jobs, err := st.ListJobsByArea(ninth)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("orphan jobs=%d", len(jobs))
	}
	w = doJSON(t, h, http.MethodPost, "/api/areas/"+ids[0]+"/analyses", nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 got %d", w.Code)
	}
}

func TestEnqueueFailAfterPut(t *testing.T) {
	q := handler.NewStubQueue(8)
	q.Fail = errors.New("boom")
	h, st := newEnv(t, nil, q)
	id := createArea(t, h)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	if w.Code != 500 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	jobs, err := st.ListJobsByArea(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("orphan=%d", len(jobs))
	}
	a, err := st.getArea(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.Period.From != "2024-04-01" || a.ActiveJobID != "" {
		t.Fatalf("area after enqueue fail: %+v", a)
	}
}

func TestInsufficientCompleted(t *testing.T) {
	h, st := newEnv(t, nil, nil)
	id := createArea(t, h)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	var body struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &body)
	if err := st.SetJobRunning(body.JobID, "analyze"); err != nil {
		t.Fatal(err)
	}
	a, err := st.getArea(id)
	if err != nil {
		t.Fatal(err)
	}
	res := domain.AnalysisRecord{
		ResultVersion:  newTestID(),
		AreaID:         id,
		Period:         a.Period,
		ComputedAt:     time.Now().UTC(),
		SchemaVersion:  "1.0",
		FeatureProfile: "ndvi-weather-v1",
		ModelVersion:   "baseline-example-1",
		Method:         "no_estimate",
		Status:         "insufficient_data",
		Series:         []domain.SeriesPoint{},
		Weather:        []domain.WeatherPoint{},
		Limitations:    []string{"Нет наблюдений"},
		Events:         []domain.AnomalyEvent{},
	}
	if err := st.PutResult(id, a.Generation, body.JobID, res); err != nil {
		t.Fatal(err)
	}
	w = doJSON(t, h, http.MethodGet, "/api/jobs/"+body.JobID, nil)
	var job map[string]any
	decode(t, w, &job)
	if job["status"] != "completed" {
		t.Fatalf("%s", w.Body.String())
	}
	w = doJSON(t, h, http.MethodGet, "/api/areas/"+id+"/series", nil)
	if w.Code != 200 {
		t.Fatalf("%d", w.Code)
	}
	var series map[string]any
	decode(t, w, &series)
	if series["status"] != "insufficient_data" || series["result_version"] == nil {
		t.Fatalf("%s", w.Body.String())
	}
	if series["severity"] != nil {
		t.Fatalf("severity=%v", series["severity"])
	}
}

func TestMLBusyThenRetry(t *testing.T) {
	h, st := newEnv(t, nil, nil)
	id := createArea(t, h)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	var body struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &body)
	if err := st.SetJobFailed(body.JobID, "ml_busy", "busy", true); err != nil {
		t.Fatal(err)
	}
	w = doJSON(t, h, http.MethodGet, "/api/jobs/"+body.JobID, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202 got %d %s", w.Code, w.Body.String())
	}
}

func TestDeleteCancels(t *testing.T) {
	h, st := newEnv(t, nil, nil)
	id := createArea(t, h)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	var body struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &body)
	if err := st.SetJobRunning(body.JobID, "satellite"); err != nil {
		t.Fatal(err)
	}
	w = doJSON(t, h, http.MethodDelete, "/api/areas/"+id, nil)
	if w.Code != 204 {
		t.Fatal(w.Body.String())
	}
	w = doJSON(t, h, http.MethodGet, "/api/jobs/"+body.JobID, nil)
	var job map[string]any
	decode(t, w, &job)
	if job["status"] != "cancelled" {
		t.Fatalf("%s", w.Body.String())
	}
	if _, err := st.getArea(id); err == nil {
		t.Fatal("area still there")
	}
	err := st.PutResult(id, 1, body.JobID, domain.AnalysisRecord{ResultVersion: newTestID(), AreaID: id})
	if err == nil {
		t.Fatal("late result accepted")
	}
	w = doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("post after delete %d %s", w.Code, w.Body.String())
	}
}

func TestTwoVersions(t *testing.T) {
	h, st := newEnv(t, nil, nil)
	id := createArea(t, h)
	completeJob := func(status string) string {
		w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
		var body struct {
			JobID string `json:"job_id"`
		}
		decode(t, w, &body)
		if err := st.SetJobRunning(body.JobID, "analyze"); err != nil {
			t.Fatal(err)
		}
		a, err := st.getArea(id)
		if err != nil {
			t.Fatal(err)
		}
		ver := newTestID()
		res := domain.AnalysisRecord{
			ResultVersion:  ver,
			AreaID:         id,
			Period:         a.Period,
			ComputedAt:     time.Now().UTC(),
			Status:         domain.ResultStatus(status),
			SchemaVersion:  "1.0",
			FeatureProfile: "ndvi-weather-v1",
			ModelVersion:   "m-" + status,
			Method:         "nearest_mean",
			Series:         []domain.SeriesPoint{},
			Weather:        []domain.WeatherPoint{},
			Events:         []domain.AnomalyEvent{},
			Limitations:    []string{},
		}
		if err := st.PutResult(id, a.Generation, body.JobID, res); err != nil {
			t.Fatal(err)
		}
		return ver
	}
	_ = completeJob("normal")
	v2 := completeJob("candidate")
	w := doJSON(t, h, http.MethodGet, "/api/areas/"+id+"/series", nil)
	var series map[string]any
	decode(t, w, &series)
	if series["result_version"] != v2 {
		t.Fatalf("%s", w.Body.String())
	}
	w = doJSON(t, h, http.MethodGet, "/api/areas", nil)
	var list struct {
		Areas []map[string]any `json:"areas"`
	}
	decode(t, w, &list)
	shown := list.Areas[0]["shown_result"].(map[string]any)
	if shown["result_version"] != v2 {
		t.Fatalf("%v", shown)
	}
	period := shown["period"].(map[string]any)
	if period["from"] == nil || period["to"] == nil {
		t.Fatalf("shown period %v", shown)
	}
	w = doJSON(t, h, http.MethodGet, "/api/areas/"+id+"/events", nil)
	var events map[string]any
	decode(t, w, &events)
	if events["result_version"] != v2 {
		t.Fatalf("events %s", w.Body.String())
	}
}

func TestAnalysesStaleActive(t *testing.T) {
	h, st := newEnv(t, nil, nil)
	id := createArea(t, h)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	var body struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &body)
	if err := st.SetJobFailed(body.JobID, "ml_busy", "busy", true); err != nil {
		t.Fatal(err)
	}
	a, err := st.getArea(id)
	if err != nil {
		t.Fatal(err)
	}
	a.ActiveJobID = body.JobID
	if err := st.updateArea(a); err != nil {
		t.Fatal(err)
	}
	w = doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("stale failed job_id 409? %d %s", w.Code, w.Body.String())
	}

	h2, st2 := newEnv(t, nil, nil)
	id2 := createArea(t, h2)
	a2, err := st2.getArea(id2)
	if err != nil {
		t.Fatal(err)
	}
	a2.ActiveJobID = "missing-job-id"
	if err := st2.updateArea(a2); err != nil {
		t.Fatal(err)
	}
	w = doJSON(t, h2, http.MethodPost, "/api/areas/"+id2+"/analyses", nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("missing job_id 409? %d %s", w.Code, w.Body.String())
	}
}

func TestAnalysesPeriodUpdate(t *testing.T) {
	h, _ := newEnv(t, nil, nil)
	id := createArea(t, h)
	body := []byte(`{"period":{"from":"2024-06-01","to":"2024-08-31"}}`)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var jobBody struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &jobBody)
	w = doJSON(t, h, http.MethodGet, "/api/jobs/"+jobBody.JobID, nil)
	var job map[string]any
	decode(t, w, &job)
	period := job["period"].(map[string]any)
	if period["from"] != "2024-06-01" || period["to"] != "2024-08-31" {
		t.Fatalf("job period %s", w.Body.String())
	}
	w = doJSON(t, h, http.MethodGet, "/api/areas", nil)
	var list struct {
		Areas []map[string]any `json:"areas"`
	}
	decode(t, w, &list)
	ap := list.Areas[0]["period"].(map[string]any)
	if ap["from"] != "2024-06-01" {
		t.Fatalf("area period %v", ap)
	}
}

func TestRestartInterruptedHTTP(t *testing.T) {
	h, st := newEnv(t, nil, nil)
	id := createArea(t, h)
	w := doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	var body struct {
		JobID string `json:"job_id"`
	}
	decode(t, w, &body)
	if err := st.SetJobRunning(body.JobID, "analyze"); err != nil {
		t.Fatal(err)
	}
	a, err := st.getArea(id)
	if err != nil {
		t.Fatal(err)
	}
	ver := newTestID()
	res := domain.AnalysisRecord{
		ResultVersion:  ver,
		AreaID:         id,
		Period:         a.Period,
		ComputedAt:     time.Now().UTC(),
		Status:         "normal",
		SchemaVersion:  "1.0",
		FeatureProfile: "ndvi-weather-v1",
		ModelVersion:   "m1",
		Method:         "nearest_mean",
		Series:         []domain.SeriesPoint{},
		Weather:        []domain.WeatherPoint{},
		Events:         []domain.AnomalyEvent{},
		Limitations:    []string{},
	}
	if err := st.PutResult(id, a.Generation, body.JobID, res); err != nil {
		t.Fatal(err)
	}
	w = doJSON(t, h, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	decode(t, w, &body)
	st.FailInterrupted()
	h2 := newEnvWithStore(t, st, nil, nil)
	w = doJSON(t, h2, http.MethodGet, "/api/jobs/"+body.JobID, nil)
	var job map[string]any
	decode(t, w, &job)
	if job["status"] != "failed" {
		t.Fatalf("%s", w.Body.String())
	}
	errBody, _ := job["error"].(map[string]any)
	if errBody["code"] != "interrupted" {
		t.Fatalf("%s", w.Body.String())
	}
	w = doJSON(t, h2, http.MethodGet, "/api/areas/"+id+"/series", nil)
	var series map[string]any
	decode(t, w, &series)
	if series["result_version"] != ver {
		t.Fatalf("lost shown_result %s", w.Body.String())
	}
	if series["severity"] != "none" {
		t.Fatalf("normal severity %s", w.Body.String())
	}
	w = doJSON(t, h2, http.MethodPost, "/api/areas/"+id+"/analyses", nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("retry after interrupt %d %s", w.Code, w.Body.String())
	}
}

func createShiftedArea(t *testing.T, h http.Handler, i int) string {
	t.Helper()
	base := 37.5 + float64(i)*0.01
	body := fmt.Sprintf(`{"name":"p%d","period":{"from":"2024-04-01","to":"2024-09-30"},"geometry":{"type":"Polygon","coordinates":[[[%f,55.7],[%f,55.7],[%f,55.8],[%f,55.8],[%f,55.7]]]},"source":{"kind":"drawn","contour_id":null,"provider":null}}`,
		i, base, base+0.05, base+0.05, base, base)
	w := doJSON(t, h, http.MethodPost, "/api/areas", []byte(body))
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var a map[string]any
	decode(t, w, &a)
	return a["id"].(string)
}

func newTestID() string {
	return fmt.Sprintf("id-%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000000)
}
