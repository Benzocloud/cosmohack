package store

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestOpenRestartInterrupted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := sampleArea("area-1")
	if err := st.PutArea(a); err != nil {
		t.Fatal(err)
	}
	res := sampleResult("area-1", "rv-aaaa", "job-old")
	if err := writeRawResult(st, res); err != nil {
		t.Fatal(err)
	}
	a.ShownResultVersion = "rv-aaaa"
	if err := st.PutArea(a); err != nil {
		t.Fatal(err)
	}
	j := Job{
		ID:             "job-queued-1",
		AreaID:         "area-1",
		Status:         JobQueued,
		Period:         a.Period,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		AreaGeneration: 1,
	}
	if err := st.PutJobQueued(j); err != nil {
		t.Fatal(err)
	}

	st2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st2.GetArea("area-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ShownResultVersion != "rv-aaaa" {
		t.Fatalf("shown_result_version=%s", got.ShownResultVersion)
	}
	if got.ActiveJobID != "" {
		t.Fatalf("active_job_id=%s", got.ActiveJobID)
	}
	jq, err := st2.GetJob("job-queued-1")
	if err != nil {
		t.Fatal(err)
	}
	if jq.Status != JobFailed || jq.Error == nil || jq.Error.Code != "interrupted" {
		t.Fatalf("job=%+v", jq)
	}
}

func TestHealCompletedWithoutMeta(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := sampleArea("area-2")
	a.ActiveJobID = "job-done"
	if err := st.PutArea(a); err != nil {
		t.Fatal(err)
	}
	ver := "rv-bbbb"
	j := Job{
		ID:            "job-done",
		AreaID:        "area-2",
		Status:        JobCompleted,
		ResultVersion: &ver,
		Period:        a.Period,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := writeRawJob(st, j); err != nil {
		t.Fatal(err)
	}
	if err := writeRawResult(st, sampleResult("area-2", ver, "job-done")); err != nil {
		t.Fatal(err)
	}

	st2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st2.GetArea("area-2")
	if err != nil {
		t.Fatal(err)
	}
	if got.ShownResultVersion != ver || got.ActiveJobID != "" {
		t.Fatalf("area=%+v", got)
	}
}

func TestPutRace(t *testing.T) {
	t.Parallel()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := sampleArea("area-race")
	if err := st.PutArea(a); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			x := a
			x.Name = "n" + string(rune('a'+i))
			if err := st.PutArea(x); err != nil {
				t.Errorf("put: %v", err)
			}
		}()
	}
	wg.Wait()
	got, err := st.GetArea("area-race")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "na" && got.Name != "nb" {
		t.Fatalf("name=%q", got.Name)
	}
}

func TestPutResultAfterDelete(t *testing.T) {
	t.Parallel()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := sampleArea("area-del")
	if err := st.PutArea(a); err != nil {
		t.Fatal(err)
	}
	j := Job{ID: "job-run", AreaID: "area-del", AreaGeneration: 1, Period: a.Period}
	if err := st.PutJobQueued(j); err != nil {
		t.Fatal(err)
	}
	if err := st.SetJobRunning("job-run", "analyze"); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteArea("area-del"); err != nil {
		t.Fatal(err)
	}
	err = st.PutResult("area-del", 1, "job-run", sampleResult("area-del", "rv-cccc", "job-run"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, err := st.GetJob("job-run"); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceExistingFile(t *testing.T) {
	t.Parallel()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := sampleArea("area-rep")
	if err := st.PutArea(a); err != nil {
		t.Fatal(err)
	}
	a.Name = "updated"
	if err := st.PutArea(a); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetArea("area-rep")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "updated" {
		t.Fatalf("name=%s", got.Name)
	}
}

func TestCorruptMeta(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := sampleArea("area-bad")
	if err := st.PutArea(a); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "areas", "area-bad", "meta.json")
	if err := os.WriteFile(p, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = st.GetArea("area-bad")
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("err=%v", err)
	}
}

func sampleArea(id string) Area {
	return Area{
		ID:         id,
		Name:       "Поле",
		Geometry:   samplePoly(),
		Source:     Source{Kind: "drawn"},
		Period:     Period{From: "2024-04-01", To: "2024-09-30"},
		CreatedAt:  time.Date(2026, 9, 4, 17, 0, 0, 0, time.UTC),
		Generation: 1,
	}
}

func samplePoly() Polygon {
	return Polygon{
		Type: "Polygon",
		Coordinates: [][][]float64{
			{{37.5, 55.7}, {37.6, 55.7}, {37.6, 55.8}, {37.5, 55.8}, {37.5, 55.7}},
		},
	}
}

func sampleResult(areaID, ver, jobID string) Result {
	return Result{
		ResultVersion:  ver,
		JobID:          jobID,
		AreaID:         areaID,
		Period:         Period{From: "2024-04-01", To: "2024-09-30"},
		ComputedAt:     time.Date(2026, 9, 4, 17, 10, 0, 0, time.UTC),
		SchemaVersion:  "1.0",
		FeatureProfile: "ndvi-weather-v1",
		ModelVersion:   "baseline-example-1",
		Method:         "nearest_mean",
		Status:         "insufficient_data",
		Series:         []SeriesPoint{},
		Weather:        []WeatherPoint{},
		Limitations:    []string{},
		Events:         []Event{},
	}
}

func writeRawResult(st *Store, r Result) error {
	b, err := marshal(r)
	if err != nil {
		return err
	}
	return replaceFile(st.resultPath(r.AreaID, r.ResultVersion), b)
}

func TestSeriesStatesRoundtrip(t *testing.T) {
	t.Parallel()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := sampleArea("area-states")
	if err := st.PutArea(a); err != nil {
		t.Fatal(err)
	}
	j := Job{ID: "job-states", AreaID: "area-states", Period: a.Period, AreaGeneration: 1}
	if err := st.PutJobQueued(j); err != nil {
		t.Fatal(err)
	}
	if err := st.SetJobRunning("job-states", "analyze"); err != nil {
		t.Fatal(err)
	}
	obs, imp := 0.42, 0.40
	res := sampleResult("area-states", "rv-states", "job-states")
	res.Series = []SeriesPoint{
		{Date: "2024-06-01", PrimaryNDVI: &obs, Value: &obs, State: "observed"},
		{Date: "2024-06-02", Value: &imp, State: "imputed"},
		{Date: "2024-06-03", State: "missing"},
	}
	if err := st.PutResult("area-states", 1, "job-states", res); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetResult("area-states", "rv-states")
	if err != nil {
		t.Fatal(err)
	}
	if got.Series[0].State != "observed" || got.Series[1].State != "imputed" || got.Series[2].State != "missing" {
		t.Fatalf("%+v", got.Series)
	}
	if got.Series[1].State == "observed" {
		t.Fatal("imputed became observed")
	}
}

func writeRawJob(st *Store, j Job) error {
	b, err := marshal(j)
	if err != nil {
		return err
	}
	return replaceFile(st.jobPath(j.ID), b)
}
