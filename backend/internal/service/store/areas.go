package store

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func (s *Store) writeAreaLocked(a Area) error {
	if err := checkID(a.ID); err != nil {
		return err
	}
	b, err := marshal(a)
	if err != nil {
		return err
	}
	return replaceFile(s.areaMetaPath(a.ID), b)
}

func (s *Store) getAreaLocked(id string) (*Area, error) {
	if err := checkID(id); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.areaMetaPath(id))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var a Area
	if err := unmarshal(b, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) listAreasLocked() ([]Area, error) {
	ents, err := os.ReadDir(s.areasDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Area, 0, len(ents))
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		a, err := s.getAreaLocked(e.Name())
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// PutArea создаёт или перезаписывает снимок участка.
func (s *Store) PutArea(a Area) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeAreaLocked(a)
}

// GetArea возвращает копию участка.
func (s *Store) GetArea(id string) (*Area, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getAreaLocked(id)
}

// ListAreas — участки по created_at, затем id.
func (s *Store) ListAreas() ([]Area, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listAreasLocked()
}

// DeleteArea отменяет активные job, удаляет area и results, job-файлы оставляет.
func (s *Store) DeleteArea(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkID(id); err != nil {
		return err
	}
	if _, err := s.getAreaLocked(id); err != nil {
		return err
	}
	if err := s.deleteAreaLocked(id); err != nil {
		return err
	}
	return nil
}

func (s *Store) deleteAreaLocked(id string) error {
	jobs, err := s.listJobsLocked()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, j := range jobs {
		if j.AreaID != id {
			continue
		}
		if j.Status != JobQueued && j.Status != JobRunning {
			continue
		}
		j.Status = JobCancelled
		j.Stage = nil
		j.UpdatedAt = now
		if err := s.writeJobLocked(j); err != nil {
			return err
		}
	}
	if err := removeTree(filepath.Join(s.resultsDir(), id)); err != nil {
		return err
	}
	return removeTree(filepath.Join(s.areasDir(), id))
}
