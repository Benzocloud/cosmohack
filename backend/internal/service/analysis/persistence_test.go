package analysis

import (
	"context"
	"sync"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// testPersistence — локальная заглушка пакета для тестов исполнителя. Production-
// реализация — repository.Repository; заглушка моделирует только доменный порт,
// необходимый для проверки переходов воркера без базы данных.
type testPersistence struct {
	mu           sync.Mutex
	areas        map[string]domain.Area
	jobs         map[string]domain.Job
	results      map[string]domain.AnalysisRecord
	putResultErr error
}

func newTestPersistence() *testPersistence {
	return &testPersistence{areas: map[string]domain.Area{}, jobs: map[string]domain.Job{}, results: map[string]domain.AnalysisRecord{}}
}

func (p *testPersistence) CreateArea(area domain.Area) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.areas[area.ID] = area
	return nil
}

func (p *testPersistence) GetArea(_ context.Context, id string) (domain.Area, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	area, ok := p.areas[id]
	if !ok {
		return domain.Area{}, domain.ErrNotFound
	}
	return area, nil
}

func (p *testPersistence) GetJob(_ context.Context, id string) (domain.Job, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, ok := p.jobs[id]
	if !ok {
		return domain.Job{}, domain.ErrNotFound
	}
	return job, nil
}

func (p *testPersistence) PutJobQueued(job domain.Job) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.putJobLocked(job)
}

func (p *testPersistence) putJobLocked(job domain.Job) error {
	area, ok := p.areas[job.AreaID]
	if !ok {
		return domain.ErrNotFound
	}
	if area.ActiveJobID != "" {
		if active, exists := p.jobs[area.ActiveJobID]; exists && IsActiveStatus(active.Status) {
			return domain.ErrConflict
		}
	}
	job.Status = domain.JobQueued
	job.AreaGeneration = area.Generation
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	job.UpdatedAt = job.CreatedAt
	p.jobs[job.ID] = job
	area.ActiveJobID = job.ID
	p.areas[area.ID] = area
	return nil
}

func (p *testPersistence) DeleteJob(_ context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, ok := p.jobs[id]
	if !ok {
		return nil
	}
	delete(p.jobs, id)
	if area, exists := p.areas[job.AreaID]; exists && area.ActiveJobID == id {
		area.ActiveJobID = ""
		p.areas[job.AreaID] = area
	}
	return nil
}

func (p *testPersistence) SetJobRunning(_ context.Context, id, stage string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, ok := p.jobs[id]
	if !ok {
		return domain.ErrNotFound
	}
	if job.Status != domain.JobQueued {
		return domain.ErrBadState
	}
	job.Status, job.Stage, job.UpdatedAt = domain.JobRunning, stringPtr(stage), time.Now().UTC()
	p.jobs[id] = job
	return nil
}

func (p *testPersistence) SetJobStage(_ context.Context, id, stage string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, ok := p.jobs[id]
	if !ok {
		return domain.ErrNotFound
	}
	if job.Status != domain.JobRunning {
		return domain.ErrBadState
	}
	job.Stage, job.UpdatedAt = stringPtr(stage), time.Now().UTC()
	p.jobs[id] = job
	return nil
}

func (p *testPersistence) SetJobFailed(_ context.Context, id, code, message string, retryable bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, ok := p.jobs[id]
	if !ok {
		return domain.ErrNotFound
	}
	if !IsActiveStatus(job.Status) {
		return domain.ErrBadState
	}
	job.Status, job.Stage, job.UpdatedAt = domain.JobFailed, nil, time.Now().UTC()
	job.ErrorCode, job.ErrorMessage, job.ErrorRetryable = stringPtr(code), stringPtr(message), boolPtr(retryable)
	p.jobs[id] = job
	p.clearActiveLocked(job.AreaID, id)
	return nil
}

func (p *testPersistence) SetJobCancelled(_ context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, ok := p.jobs[id]
	if !ok {
		return domain.ErrNotFound
	}
	if !IsActiveStatus(job.Status) {
		return domain.ErrBadState
	}
	job.Status, job.Stage, job.UpdatedAt = domain.JobCancelled, nil, time.Now().UTC()
	p.jobs[id] = job
	p.clearActiveLocked(job.AreaID, id)
	return nil
}

func (p *testPersistence) SetJobInputRevision(_ context.Context, id, revision string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, ok := p.jobs[id]
	if !ok {
		return domain.ErrNotFound
	}
	job.InputRevision, job.UpdatedAt = stringPtr(revision), time.Now().UTC()
	p.jobs[id] = job
	return nil
}

func (p *testPersistence) PutResult(_ context.Context, _ int, jobID string, result domain.AnalysisRecord) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.putResultErr != nil {
		return p.putResultErr
	}
	job, ok := p.jobs[jobID]
	if !ok {
		return domain.ErrNotFound
	}
	if !IsActiveStatus(job.Status) || job.Status != domain.JobRunning {
		return domain.ErrBadState
	}
	p.results[resultKey(result.AreaID, result.ResultVersion)] = result
	job.Status, job.Stage, job.ResultVersion, job.UpdatedAt = domain.JobCompleted, nil, stringPtr(result.ResultVersion), time.Now().UTC()
	p.jobs[jobID] = job
	p.clearActiveLocked(job.AreaID, jobID)
	return nil
}

func (p *testPersistence) GetResult(areaID, version string) (domain.AnalysisRecord, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	result, ok := p.results[resultKey(areaID, version)]
	if !ok {
		return domain.AnalysisRecord{}, domain.ErrNotFound
	}
	return result, nil
}

func (p *testPersistence) RecoverInterrupted(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, job := range p.jobs {
		if !IsActiveStatus(job.Status) {
			continue
		}
		job.Status, job.Stage, job.UpdatedAt = domain.JobFailed, nil, time.Now().UTC()
		job.ErrorCode, job.ErrorMessage, job.ErrorRetryable = stringPtr(domain.InterruptReason), stringPtr("server restarted; rerun analysis"), boolPtr(true)
		p.jobs[id] = job
		p.clearActiveLocked(job.AreaID, id)
	}
	return nil
}

func (p *testPersistence) clearActiveLocked(areaID, jobID string) {
	area, ok := p.areas[areaID]
	if ok && area.ActiveJobID == jobID {
		area.ActiveJobID = ""
		p.areas[areaID] = area
	}
}

func resultKey(areaID, version string) string { return areaID + "\x00" + version }
func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func boolPtr(value bool) *bool { return &value }

var _ Persistence = (*testPersistence)(nil)

func (p *testPersistence) getArea(id string) (domain.Area, error) {
	return p.GetArea(context.Background(), id)
}

func (p *testPersistence) getJob(id string) (domain.Job, error) {
	return p.GetJob(context.Background(), id)
}

func (p *testPersistence) putJobQueued(job domain.Job) error {
	return p.PutJobQueued(job)
}

func (p *testPersistence) getResult(areaID, version string) (domain.AnalysisRecord, error) {
	return p.GetResult(areaID, version)
}
