package app

import (
	"context"
	"errors"

	"github.com/Benzocloud/cosmohack/backend/internal/handler"
	"github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
)

// executorQueue адаптирует исполнитель к порту очереди обработчика.
type executorQueue struct {
	exec *analysis.Executor
}

func (q *executorQueue) Enqueue(ctx context.Context, jobID string) error {
	if err := q.exec.Enqueue(ctx, jobID); err != nil {
		if errors.Is(err, analysis.ErrQueueFull) {
			return handler.ErrQueueFull
		}

		return err
	}

	return nil
}
