package store

import (
	"errors"
	"os"
	"time"
)

func (s *Store) getResultLocked(areaID, version string) (*Result, error) {
	if err := checkID(areaID); err != nil {
		return nil, err
	}
	if err := checkID(version); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.resultPath(areaID, version))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var r Result
	if err := unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// GetResult читает неизменяемый снимок версии.
func (s *Store) GetResult(areaID, version string) (*Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getResultLocked(areaID, version)
}

// PutResult пишет result, затем job completed, затем meta.
func (s *Store) PutResult(areaID string, generationAtStart int, jobID string, result Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.getAreaLocked(areaID)
	if err != nil {
		return err
	}
	if a.Generation != generationAtStart {
		return ErrGeneration
	}
	j, err := s.getJobLocked(jobID)
	if err != nil {
		return err
	}
	if j.Status != JobRunning {
		return ErrBadState
	}
	if result.ResultVersion == "" {
		return errors.New("empty result_version")
	}
	if err := checkID(result.ResultVersion); err != nil {
		return err
	}
	result.AreaID = areaID
	result.JobID = jobID
	b, err := marshal(result)
	if err != nil {
		return err
	}
	if err := replaceFile(s.resultPath(areaID, result.ResultVersion), b); err != nil {
		return err
	}
	ver := result.ResultVersion
	j.Status = JobCompleted
	j.Stage = nil
	j.Error = nil
	j.ResultVersion = &ver
	j.UpdatedAt = time.Now().UTC()
	if err := s.writeJobLocked(*j); err != nil {
		return err
	}
	a.ShownResultVersion = ver
	if a.ActiveJobID == jobID {
		a.ActiveJobID = ""
	}
	return s.writeAreaLocked(*a)
}
