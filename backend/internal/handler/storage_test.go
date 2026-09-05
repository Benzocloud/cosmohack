package handler_test

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// testStorage — локальная заглушка пакета для HTTP-тестов. Production использует
// repository.Repository; заглушка рядом с тестами не создаёт второй пакет хранения.
type testStorage struct {
	mu      sync.Mutex
	areas   map[string]domain.Area
	jobs    map[string]domain.Job
	results map[string]domain.AnalysisRecord
}

func newTestStorage() *testStorage {
	return &testStorage{
		areas: map[string]domain.Area{}, jobs: map[string]domain.Job{}, results: map[string]domain.AnalysisRecord{},
	}
}

func (s *testStorage) CreateArea(_ context.Context, area domain.Area) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if area.Generation == 0 {
		area.Generation = 1
	}
	s.areas[area.ID] = area
	return nil
}

func (s *testStorage) UpdateArea(_ context.Context, area domain.Area) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.areas[area.ID] = area
	return nil
}

func (s *testStorage) GetArea(_ context.Context, id string) (domain.Area, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	area, ok := s.areas[id]
	if !ok {
		return domain.Area{}, domain.ErrNotFound
	}
	return area, nil
}

func (s *testStorage) ListAreas(_ context.Context) ([]domain.Area, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	areas := make([]domain.Area, 0, len(s.areas))
	for _, area := range s.areas {
		areas = append(areas, area)
	}
	sort.Slice(areas, func(i, j int) bool { return areas[i].CreatedAt.Before(areas[j].CreatedAt) })
	return areas, nil
}

func (s *testStorage) DeleteArea(_ context.Context, id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.areas[id]; !ok {
		return nil, domain.ErrNotFound
	}
	var cancelled []string
	for jobID, job := range s.jobs {
		if job.AreaID != id || (job.Status != domain.JobQueued && job.Status != domain.JobRunning) {
			continue
		}
		job.Status = domain.JobCancelled
		job.Stage = nil
		job.UpdatedAt = time.Now().UTC()
		s.jobs[jobID] = job
		cancelled = append(cancelled, jobID)
	}
	for key, result := range s.results {
		if result.AreaID == id {
			delete(s.results, key)
		}
	}
	delete(s.areas, id)
	return cancelled, nil
}

func (s *testStorage) GetJob(_ context.Context, id string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, domain.ErrNotFound
	}
	return job, nil
}

func (s *testStorage) PutJobQueued(_ context.Context, job domain.Job) error {
	return s.putJob(job, nil)
}

func (s *testStorage) PutJobQueuedWithPeriod(_ context.Context, job domain.Job, period domain.Period) error {
	return s.putJob(job, &period)
}

func (s *testStorage) putJob(job domain.Job, period *domain.Period) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	area, ok := s.areas[job.AreaID]
	if !ok {
		return domain.ErrNotFound
	}
	if area.ActiveJobID != "" {
		if active, exists := s.jobs[area.ActiveJobID]; exists && (active.Status == domain.JobQueued || active.Status == domain.JobRunning) {
			return domain.ErrConflict
		}
	}
	if period != nil {
		area.Period = *period
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	job.Status = domain.JobQueued
	job.Stage = nil
	job.ErrorCode, job.ErrorMessage, job.ErrorRetryable, job.ResultVersion = nil, nil, nil, nil
	job.UpdatedAt = job.CreatedAt
	job.AreaGeneration = area.Generation
	s.jobs[job.ID] = job
	area.ActiveJobID = job.ID
	s.areas[area.ID] = area
	return nil
}

func (s *testStorage) DeleteJob(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil
	}
	delete(s.jobs, id)
	if area, exists := s.areas[job.AreaID]; exists && area.ActiveJobID == id {
		area.ActiveJobID = ""
		s.areas[job.AreaID] = area
	}
	return nil
}

func (s *testStorage) GetResult(_ context.Context, areaID, version string) (domain.AnalysisRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.results[resultKey(areaID, version)]
	if !ok {
		return domain.AnalysisRecord{}, domain.ErrNotFound
	}
	return result, nil
}

func (s *testStorage) SetJobRunning(id, stage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return domain.ErrNotFound
	}
	if job.Status != domain.JobQueued {
		return domain.ErrBadState
	}
	job.Status, job.Stage, job.UpdatedAt = domain.JobRunning, stringPtr(stage), time.Now().UTC()
	s.jobs[id] = job
	return nil
}

func (s *testStorage) SetJobFailed(id, code, message string, retryable bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return domain.ErrNotFound
	}
	if job.Status == domain.JobCompleted || job.Status == domain.JobCancelled || job.Status == domain.JobFailed {
		return domain.ErrBadState
	}
	job.Status, job.Stage, job.UpdatedAt = domain.JobFailed, nil, time.Now().UTC()
	job.ErrorCode, job.ErrorMessage, job.ErrorRetryable = stringPtr(code), stringPtr(message), boolPtr(retryable)
	s.jobs[id] = job
	s.clearActiveLocked(job.AreaID, id)
	return nil
}

func (s *testStorage) PutResult(areaID string, generation int, jobID string, result domain.AnalysisRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	area, ok := s.areas[areaID]
	if !ok {
		return domain.ErrNotFound
	}
	if area.Generation != generation {
		return domain.ErrGeneration
	}
	job, ok := s.jobs[jobID]
	if !ok {
		return domain.ErrNotFound
	}
	if job.Status != domain.JobRunning {
		return domain.ErrBadState
	}
	result.AreaID = areaID
	s.results[resultKey(areaID, result.ResultVersion)] = result
	job.Status, job.Stage, job.ResultVersion, job.UpdatedAt = domain.JobCompleted, nil, stringPtr(result.ResultVersion), time.Now().UTC()
	s.jobs[jobID] = job
	area.ShownResultVersion, area.ShownJobID, area.ActiveJobID = result.ResultVersion, jobID, ""
	s.areas[areaID] = area
	return nil
}

func (s *testStorage) ListJobsByArea(areaID string) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]domain.Job, 0)
	for _, job := range s.jobs {
		if job.AreaID == areaID {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.Before(jobs[j].CreatedAt) })
	return jobs, nil
}

func (s *testStorage) FailInterrupted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, job := range s.jobs {
		if job.Status != domain.JobQueued && job.Status != domain.JobRunning {
			continue
		}
		job.Status, job.Stage, job.UpdatedAt = domain.JobFailed, nil, time.Now().UTC()
		job.ErrorCode, job.ErrorMessage, job.ErrorRetryable = stringPtr(domain.InterruptReason), stringPtr("server restarted; rerun analysis"), boolPtr(true)
		s.jobs[id] = job
		s.clearActiveLocked(job.AreaID, id)
	}
}

func (s *testStorage) clearActiveLocked(areaID, jobID string) {
	area, ok := s.areas[areaID]
	if ok && area.ActiveJobID == jobID {
		area.ActiveJobID = ""
		s.areas[areaID] = area
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

func (s *testStorage) getArea(id string) (domain.Area, error) {
	return s.GetArea(context.Background(), id)
}

func (s *testStorage) updateArea(area domain.Area) error {
	return s.UpdateArea(context.Background(), area)
}
