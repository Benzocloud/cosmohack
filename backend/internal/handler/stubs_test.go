package handler_test

import (
	"context"
	"sync"

	"github.com/Benzocloud/cosmohack/backend/internal/handler"
)

type stubQueue struct {
	Limit int
	Fail  error

	mu      sync.Mutex
	waiting map[string]struct{}
}

func newStubQueue(limit int) *stubQueue {
	if limit <= 0 {
		limit = 8
	}
	return &stubQueue{Limit: limit, waiting: map[string]struct{}{}}
}

func (q *stubQueue) Enqueue(_ context.Context, jobID string) error {
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
		return handler.ErrQueueFull
	}
	q.waiting[jobID] = struct{}{}
	return nil
}

func (q *stubQueue) Cancel(jobID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.waiting, jobID)
}

type stubContours struct {
	Items []handler.Contour
	Err   error
}

func (s stubContours) Find(_ context.Context, _, _, _, _ float64) ([]handler.Contour, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	if s.Items == nil {
		return []handler.Contour{}, nil
	}
	return s.Items, nil
}
