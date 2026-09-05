package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func TestPostgresRepositoryIntegration(t *testing.T) {
	db, repo := openIntegrationRepository(t)
	ctx := context.Background()

	areaID := integrationID("area")
	cleanupIntegrationArea(t, db, areaID)
	area := integrationArea(areaID)
	if err := repo.CreateArea(ctx, area); err != nil {
		t.Fatalf("create area: %v", err)
	}
	if err := repo.CreateArea(ctx, area); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate area error = %v, want ErrConflict", err)
	}
	got, err := repo.GetArea(ctx, areaID)
	if err != nil {
		t.Fatalf("get area: %v", err)
	}
	if got.ID != area.ID || got.Name != area.Name || got.Geometry.Type != area.Geometry.Type || got.Source.Kind != area.Source.Kind || got.Period != area.Period {
		t.Fatalf("area round-trip = %+v, want %+v", got, area)
	}
	updated := area
	updated.Name = "updated field"
	updated.Period = domain.Period{From: "2026-05-02", To: "2026-05-04"}
	if err := repo.UpdateArea(ctx, updated); err != nil {
		t.Fatalf("update area: %v", err)
	}
	got, err = repo.GetArea(ctx, areaID)
	if err != nil || got.Name != updated.Name || got.Period != updated.Period {
		t.Fatalf("updated area = %+v, err=%v", got, err)
	}

	jobA := integrationJob(integrationID("job-a"), areaID, updated.Period)
	jobB := integrationJob(integrationID("job-b"), areaID, updated.Period)
	type queueResult struct {
		id  string
		err error
	}
	queueResults := make(chan queueResult, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, job := range []domain.Job{jobA, jobB} {
		wg.Add(1)
		go func(job domain.Job) {
			defer wg.Done()
			<-start
			queueResults <- queueResult{id: job.ID, err: repo.PutJobQueued(ctx, job)}
		}(job)
	}
	close(start)
	wg.Wait()
	close(queueResults)
	var queuedID string
	for result := range queueResults {
		switch {
		case result.err == nil:
			if queuedID != "" {
				t.Fatal("two concurrent jobs claimed one area")
			}
			queuedID = result.id
		case errors.Is(result.err, ErrConflict):
		default:
			t.Fatalf("concurrent queue error = %v", result.err)
		}
	}
	if queuedID == "" {
		t.Fatal("one concurrent queue operation must succeed")
	}
	jobs, err := repo.ListJobsByArea(ctx, areaID)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs after concurrent queue = %d, err=%v; want one", len(jobs), err)
	}
	deletedJobs, err := repo.DeleteArea(ctx, areaID)
	if err != nil || len(deletedJobs) != 1 {
		t.Fatalf("delete area active jobs = %v, err=%v", deletedJobs, err)
	}
	if deletedJobs[0] != queuedID {
		t.Fatalf("deleted active job = %s, queued winner = %s", deletedJobs[0], queuedID)
	}
	if _, err := repo.GetArea(ctx, areaID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted area error = %v, want ErrNotFound", err)
	}
	deletedJob, err := repo.GetJob(ctx, deletedJobs[0])
	if err != nil || deletedJob.Status != domain.JobCancelled {
		t.Fatalf("deleted area job = %+v, err=%v; want cancelled", deletedJob, err)
	}

	resultAreaID := integrationID("result-area")
	cleanupIntegrationArea(t, db, resultAreaID)
	resultArea := integrationArea(resultAreaID)
	if err := repo.CreateArea(ctx, resultArea); err != nil {
		t.Fatalf("create result area: %v", err)
	}
	resultJob := integrationJob(integrationID("result-job"), resultAreaID, resultArea.Period)
	if err := repo.PutJobQueued(ctx, resultJob); err != nil {
		t.Fatalf("queue result job: %v", err)
	}
	if err := repo.SetJobRunning(ctx, resultJob.ID, domain.StageAnalyze); err != nil {
		t.Fatalf("run result job: %v", err)
	}
	result := integrationResult(resultAreaID)
	if err := repo.PutResult(ctx, 1, resultJob.ID, result); err != nil {
		t.Fatalf("put result: %v", err)
	}
	stored, err := repo.GetResult(ctx, resultAreaID, result.ResultVersion)
	if err != nil {
		t.Fatalf("get result: %v", err)
	}
	if stored.AreaID != result.AreaID || stored.ResultVersion != result.ResultVersion || stored.Status != result.Status || len(stored.Series) != 1 || len(stored.Events) != 1 || stored.Provenance["source"] != "fixture" {
		t.Fatalf("result round-trip = %+v", stored)
	}
	published, err := repo.GetArea(ctx, resultAreaID)
	if err != nil || published.ShownResultVersion != result.ResultVersion || published.ShownJobID != resultJob.ID || published.ActiveJobID != "" {
		t.Fatalf("published area = %+v, err=%v", published, err)
	}

	replayJob := integrationJob(integrationID("replay-job"), resultAreaID, resultArea.Period)
	if err := repo.PutJobQueued(ctx, replayJob); err != nil {
		t.Fatalf("queue replay job: %v", err)
	}
	if err := repo.SetJobRunning(ctx, replayJob.ID, domain.StageAnalyze); err != nil {
		t.Fatalf("run replay job: %v", err)
	}
	if err := repo.PutResult(ctx, 1, replayJob.ID, result); err != nil {
		t.Fatalf("replay identical result: %v", err)
	}
	replayStored, err := repo.GetJob(ctx, replayJob.ID)
	if err != nil || replayStored.Status != domain.JobCompleted || replayStored.ResultVersion == nil || *replayStored.ResultVersion != result.ResultVersion {
		t.Fatalf("replayed job = %+v, err=%v", replayStored, err)
	}
	published, err = repo.GetArea(ctx, resultAreaID)
	if err != nil || published.ShownJobID != replayJob.ID {
		t.Fatalf("replayed shown job = %+v, err=%v", published, err)
	}

	conflictJob := integrationJob(integrationID("conflict-job"), resultAreaID, resultArea.Period)
	if err := repo.PutJobQueued(ctx, conflictJob); err != nil {
		t.Fatalf("queue conflict job: %v", err)
	}
	if err := repo.SetJobRunning(ctx, conflictJob.ID, domain.StageAnalyze); err != nil {
		t.Fatalf("run conflict job: %v", err)
	}
	conflicting := result
	conflicting.InputRevision = "different-input"
	if err := repo.PutResult(ctx, 1, conflictJob.ID, conflicting); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting result error = %v, want ErrConflict", err)
	}
	conflictState, err := repo.GetJob(ctx, conflictJob.ID)
	if err != nil || conflictState.Status != domain.JobRunning {
		t.Fatalf("conflicting result changed job = %+v, err=%v", conflictState, err)
	}
	if err := repo.SetJobFailed(ctx, conflictJob.ID, "conflict", "test cleanup", false); err != nil {
		t.Fatalf("fail conflict job: %v", err)
	}

	generationAreaID := integrationID("generation-area")
	cleanupIntegrationArea(t, db, generationAreaID)
	generationArea := integrationArea(generationAreaID)
	if err := repo.CreateArea(ctx, generationArea); err != nil {
		t.Fatalf("create generation area: %v", err)
	}
	generationJob := integrationJob(integrationID("generation-job"), generationAreaID, generationArea.Period)
	if err := repo.PutJobQueued(ctx, generationJob); err != nil {
		t.Fatalf("queue generation job: %v", err)
	}
	if err := repo.SetJobRunning(ctx, generationJob.ID, domain.StageAnalyze); err != nil {
		t.Fatalf("run generation job: %v", err)
	}
	generationArea.ActiveJobID = generationJob.ID
	generationArea.Generation = 2
	if err := repo.UpdateArea(ctx, generationArea); err != nil {
		t.Fatalf("update generation area: %v", err)
	}
	generationResult := integrationResult(generationAreaID)
	if err := repo.PutResult(ctx, 1, generationJob.ID, generationResult); !errors.Is(err, ErrGeneration) {
		t.Fatalf("stale generation error = %v, want ErrGeneration", err)
	}
	if err := repo.SetJobCancelled(ctx, generationJob.ID); err != nil {
		t.Fatalf("cancel stale generation job: %v", err)
	}

	recoveryAreaID := integrationID("recovery-area")
	cleanupIntegrationArea(t, db, recoveryAreaID)
	recoveryArea := integrationArea(recoveryAreaID)
	if err := repo.CreateArea(ctx, recoveryArea); err != nil {
		t.Fatalf("create recovery area: %v", err)
	}
	recoveryJob := integrationJob(integrationID("recovery-job"), recoveryAreaID, recoveryArea.Period)
	if err := repo.PutJobQueued(ctx, recoveryJob); err != nil {
		t.Fatalf("queue recovery job: %v", err)
	}
	if err := repo.SetJobRunning(ctx, recoveryJob.ID, domain.StageAnalyze); err != nil {
		t.Fatalf("run recovery job: %v", err)
	}
	if err := repo.RecoverInterrupted(ctx); err != nil {
		t.Fatalf("recover jobs: %v", err)
	}
	recovered, err := repo.GetJob(ctx, recoveryJob.ID)
	if err != nil || recovered.Status != domain.JobFailed || recovered.ErrorCode == nil || *recovered.ErrorCode != domain.InterruptReason {
		t.Fatalf("recovered job = %+v, err=%v", recovered, err)
	}
	recoveryArea, err = repo.GetArea(ctx, recoveryAreaID)
	if err != nil || recoveryArea.ActiveJobID != "" {
		t.Fatalf("recovered area = %+v, err=%v", recoveryArea, err)
	}
	if deleted, err := repo.DeleteArea(ctx, resultAreaID); err != nil || len(deleted) != 0 {
		t.Fatalf("delete result area = %v, err=%v", deleted, err)
	}
	if _, err := repo.GetResult(ctx, resultAreaID, result.ResultVersion); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted result error = %v, want ErrNotFound", err)
	}
	if retained, err := repo.GetJob(ctx, resultJob.ID); err != nil || retained.Status != domain.JobCompleted {
		t.Fatalf("retained job after area delete = %+v, err=%v", retained, err)
	}
}

func openIntegrationRepository(t *testing.T) (*sqlx.DB, *Repository) {
	t.Helper()
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	db, err := sqlx.Open("pgx", url)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		if os.Getenv("CI") == "" {
			t.Skipf("postgres is unavailable: %v", err)
		}
		t.Fatalf("postgres is unavailable in CI: %v", err)
	}
	var table sql.NullString
	if err := db.GetContext(ctx, &table, "SELECT to_regclass('public.areas')"); err != nil {
		t.Fatalf("check migrated schema: %v", err)
	}
	if !table.Valid || table.String == "" {
		t.Fatalf("areas table is missing; apply migrations before integration tests")
	}
	repo, err := New(db)
	if err != nil {
		t.Fatalf("build repository: %v", err)
	}
	return db, repo
}

func cleanupIntegrationArea(t *testing.T, db *sqlx.DB, areaID string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = db.ExecContext(ctx, "DELETE FROM analysis_results WHERE area_id = $1", areaID)
		_, _ = db.ExecContext(ctx, "DELETE FROM jobs WHERE area_id = $1", areaID)
		_, _ = db.ExecContext(ctx, "DELETE FROM areas WHERE id = $1", areaID)
	})
}

func integrationID(kind string) string {
	return fmt.Sprintf("integration-%s-%d", kind, time.Now().UnixNano())
}

func integrationArea(id string) domain.Area {
	return domain.Area{
		ID: id, Name: "integration field",
		Geometry:  domain.Polygon{Type: "Polygon", Coordinates: [][][]float64{{{30, 50}, {31, 50}, {30, 50}}}},
		Source:    domain.AreaSource{Kind: "drawn"},
		Period:    domain.Period{From: "2026-05-01", To: "2026-05-03"},
		CreatedAt: time.Now().UTC(), Generation: 1,
	}
}

func integrationJob(id, areaID string, period domain.Period) domain.Job {
	return domain.Job{ID: id, AreaID: areaID, Period: period, CreatedAt: time.Now().UTC()}
}

func integrationResult(areaID string) domain.AnalysisRecord {
	severity := domain.SeverityModerate
	value := 0.42
	return domain.AnalysisRecord{
		ResultVersion: "result-deterministic-1", AreaID: areaID,
		Period:     domain.Period{From: "2026-05-01", To: "2026-05-03"},
		ComputedAt: time.Now().UTC(), InputRevision: "input-1",
		SchemaVersion: domain.SchemaVersionV1, FeatureProfile: domain.FeatureProfileNDVIWeatherV1,
		ModelVersion: "model-1", Method: "baseline", Status: domain.StatusCandidate, Severity: &severity,
		Series:     []domain.SeriesPoint{{Date: "2026-05-02", Value: &value, State: domain.StateImputed}},
		Weather:    []domain.WeatherPoint{{Date: "2026-05-02"}},
		Provenance: map[string]any{"source": "fixture"}, Limitations: []string{"integration"},
		Events: []domain.AnomalyEvent{{StartDate: "2026-05-02", EndDate: "2026-05-02", Status: domain.StatusCandidate, Severity: severity}},
	}
}
