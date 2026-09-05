package handler

import (
	"context"
	"sync"
)

// StubQueue — заглушка очереди из 8 ожидающих; не читает persistence.
type StubQueue struct {
	Limit int
	Fail  error

	mu      sync.Mutex
	waiting map[string]struct{}
}

func NewStubQueue(limit int) *StubQueue {
	if limit <= 0 {
		limit = 8
	}
	return &StubQueue{Limit: limit, waiting: map[string]struct{}{}}
}

func (q *StubQueue) Enqueue(_ context.Context, jobID string) error {
	if q.Fail != nil {
		return q.Fail
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.waiting == nil {
		q.waiting = map[string]struct{}{}
	}
	if _, ok := q.waiting[jobID]; ok {
		return nil
	}
	if len(q.waiting) >= q.Limit {
		return ErrQueueFull
	}
	q.waiting[jobID] = struct{}{}
	return nil
}

func (q *StubQueue) Cancel(jobID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.waiting, jobID)
}

// StubContours — три режима поиска: список, пусто, ошибка.
type StubContours struct {
	Items []Contour
	Err   error
}

func (s StubContours) Find(_ context.Context, _, _, _, _ float64) ([]Contour, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	if s.Items == nil {
		return []Contour{}, nil
	}
	return s.Items, nil
}
