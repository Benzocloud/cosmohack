package store

import (
	"errors"
	"os"
	"strings"
	"time"
)

func (s *Store) writeJobLocked(j Job) error {
	if err := checkID(j.ID); err != nil {
		return err
	}
	b, err := marshal(j)
	if err != nil {
		return err
	}
	return replaceFile(s.jobPath(j.ID), b)
}

func (s *Store) getJobLocked(id string) (*Job, error) {
	if err := checkID(id); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.jobPath(id))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var j Job
	if err := unmarshal(b, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

func (s *Store) listJobsLocked() ([]Job, error) {
	ents, err := os.ReadDir(s.jobsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Job, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		j, err := s.getJobLocked(id)
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrBadID) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, nil
}

func (s *Store) clearActiveIfLocked(areaID, jobID string) error {
	a, err := s.getAreaLocked(areaID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if a.ActiveJobID != jobID {
		return nil
	}
	a.ActiveJobID = ""
	return s.writeAreaLocked(*a)
}

func (s *Store) putJobQueuedLocked(j Job) error {
	if err := checkID(j.ID); err != nil {
		return err
	}
	a, err := s.getAreaLocked(j.AreaID)
	if err != nil {
		return err
	}
	j.Status = JobQueued
	j.Stage = nil
	j.Error = nil
	j.ResultVersion = nil
	if j.CreatedAt.IsZero() {
		j.CreatedAt = time.Now().UTC()
	}
	j.UpdatedAt = j.CreatedAt
	if err := s.writeJobLocked(j); err != nil {
		return err
	}
	a.ActiveJobID = j.ID
	if err := s.writeAreaLocked(*a); err != nil {
		_ = os.Remove(s.jobPath(j.ID))
		return err
	}
	return nil
}

func (s *Store) deleteJobLocked(id string) error {
	j, err := s.getJobLocked(id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.Remove(s.jobPath(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return s.clearActiveIfLocked(j.AreaID, id)
}

// PutJobQueued пишет queued job и выставляет active_job_id.
func (s *Store) PutJobQueued(j Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.putJobQueuedLocked(j)
}

// DeleteJob убирает снимок и сбрасывает active_job_id, если он на этот id.
func (s *Store) DeleteJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteJobLocked(id)
}

// GetJob возвращает копию задачи.
func (s *Store) GetJob(id string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getJobLocked(id)
}

// SetJobRunning — только queued → running.
func (s *Store) SetJobRunning(id, stage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.getJobLocked(id)
	if err != nil {
		return err
	}
	if j.Status != JobQueued {
		return ErrBadState
	}
	j.Status = JobRunning
	if stage == "" {
		j.Stage = nil
	} else {
		s := stage
		j.Stage = &s
	}
	j.UpdatedAt = time.Now().UTC()
	return s.writeJobLocked(*j)
}

// SetJobStage — только running.
func (s *Store) SetJobStage(id, stage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.getJobLocked(id)
	if err != nil {
		return err
	}
	if j.Status != JobRunning {
		return ErrBadState
	}
	if stage == "" {
		j.Stage = nil
	} else {
		st := stage
		j.Stage = &st
	}
	j.UpdatedAt = time.Now().UTC()
	return s.writeJobLocked(*j)
}

// SetJobFailed помечает failed и сбрасывает active_job_id.
func (s *Store) SetJobFailed(id string, jobErr JobError) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.getJobLocked(id)
	if err != nil {
		return err
	}
	if j.Status == JobCompleted || j.Status == JobCancelled || j.Status == JobFailed {
		return ErrBadState
	}
	j.Status = JobFailed
	j.Stage = nil
	j.Error = &jobErr
	j.UpdatedAt = time.Now().UTC()
	if err := s.writeJobLocked(*j); err != nil {
		return err
	}
	return s.clearActiveIfLocked(j.AreaID, id)
}

// SetJobCancelled — терминальный cancelled.
func (s *Store) SetJobCancelled(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.getJobLocked(id)
	if err != nil {
		return err
	}
	if j.Status == JobCompleted || j.Status == JobFailed || j.Status == JobCancelled {
		return ErrBadState
	}
	j.Status = JobCancelled
	j.Stage = nil
	j.UpdatedAt = time.Now().UTC()
	if err := s.writeJobLocked(*j); err != nil {
		return err
	}
	return s.clearActiveIfLocked(j.AreaID, id)
}

// SetJobInputRevision заполняет B4 после заморозки входа.
func (s *Store) SetJobInputRevision(id, rev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.getJobLocked(id)
	if err != nil {
		return err
	}
	r := rev
	j.InputRevision = &r
	j.UpdatedAt = time.Now().UTC()
	return s.writeJobLocked(*j)
}

// ListJobsByArea — все снимки job участка (в том числе после DELETE area).
func (s *Store) ListJobsByArea(areaID string) ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.listJobsLocked()
	if err != nil {
		return nil, err
	}
	out := make([]Job, 0)
	for _, j := range all {
		if j.AreaID == areaID {
			out = append(out, j)
		}
	}
	return out, nil
}
