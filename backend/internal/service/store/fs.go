package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrCorrupt    = errors.New("corrupt snapshot")
	ErrBadID      = errors.New("invalid id")
	ErrBadState   = errors.New("invalid job state")
	ErrGeneration = errors.New("generation mismatch")
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

// Store — файловое хранилище участков, задач и результатов.
type Store struct {
	root string
	mu   sync.Mutex
}

func (s *Store) areasDir() string   { return filepath.Join(s.root, "areas") }
func (s *Store) jobsDir() string    { return filepath.Join(s.root, "jobs") }
func (s *Store) resultsDir() string { return filepath.Join(s.root, "results") }

func (s *Store) areaMetaPath(id string) string {
	return filepath.Join(s.areasDir(), id, "meta.json")
}

func (s *Store) jobPath(id string) string {
	return filepath.Join(s.jobsDir(), id+".json")
}

func (s *Store) resultPath(areaID, version string) string {
	return filepath.Join(s.resultsDir(), areaID, version+".json")
}

func checkID(id string) error {
	if !idPattern.MatchString(id) {
		return ErrBadID
	}
	return nil
}

// ValidID — [A-Za-z0-9_-]{1,128}; тот же алфавит, что у файлов store.
func ValidID(id string) error {
	return checkID(id)
}

// FailInterrupted помечает queued/running как failed/interrupted и сбрасывает active_job_id.
func (s *Store) FailInterrupted() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failInterruptedLocked()
}

// Open создаёт каталоги, сбрасывает незавершённые job и чинит оборванный PutResult.
func Open(root string) (*Store, error) {
	s := &Store{root: root}
	for _, d := range []string{s.areasDir(), s.jobsDir(), s.resultsDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cleanTmp(root)
	if err := s.failInterruptedLocked(); err != nil {
		return nil, err
	}
	if err := s.healActiveLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) failInterruptedLocked() error {
	jobs, err := s.listJobsLocked()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	msg := "Сервер перезапущен, запустите анализ снова"
	affected := map[string]struct{}{}
	for _, j := range jobs {
		if j.Status != JobQueued && j.Status != JobRunning {
			continue
		}
		j.Status = JobFailed
		j.Stage = nil
		j.Error = &JobError{Code: "interrupted", Message: msg, Retryable: true}
		j.UpdatedAt = now
		if err := s.writeJobLocked(j); err != nil {
			return err
		}
		affected[j.AreaID] = struct{}{}
	}
	for areaID := range affected {
		a, err := s.getAreaLocked(areaID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		a.ActiveJobID = ""
		if err := s.writeAreaLocked(*a); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) healActiveLocked() error {
	areas, err := s.listAreasLocked()
	if err != nil {
		return err
	}
	for i := range areas {
		a := areas[i]
		if a.ActiveJobID == "" {
			continue
		}
		j, err := s.getJobLocked(a.ActiveJobID)
		if errors.Is(err, ErrNotFound) {
			a.ActiveJobID = ""
			if err := s.writeAreaLocked(a); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		switch j.Status {
		case JobFailed, JobCancelled:
			a.ActiveJobID = ""
			if err := s.writeAreaLocked(a); err != nil {
				return err
			}
		case JobCompleted:
			if j.ResultVersion != nil && *j.ResultVersion != "" {
				if _, err := s.getResultLocked(a.ID, *j.ResultVersion); err == nil {
					a.ShownResultVersion = *j.ResultVersion
				}
			}
			a.ActiveJobID = ""
			if err := s.writeAreaLocked(a); err != nil {
				return err
			}
		}
	}
	return nil
}

func marshal(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	b = append(b, '\n')
	return b, nil
}

func unmarshal(b []byte, v any) error {
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return nil
}
