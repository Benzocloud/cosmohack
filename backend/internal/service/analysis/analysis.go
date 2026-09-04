// Package analysis — исполнитель анализа: один воркер-goroutine и
// ограниченная очередь внутри Go, последовательность сбор → ML → сохранение
// через операции хранилища B3 (.agent/plan.md, «Архитектура»).
//
// Исполнитель не повторяет POST автоматически; отмена задачи прекращает
// ожидание Go и блокирует позднее сохранение. Занятый ML не прерывается
// отменой клиента: слот удерживается сервисом ML до фактического завершения.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/service/ml"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

// queueWaitingLimit — начальный лимит ожидающих задач; активная задача
// в лимит не входит. Изменение согласуют B4 и ML и фиксируют в README.
const queueWaitingLimit = 8

// ErrQueueFull — очередь ожидающих заполнена; повтор запуска явный.
// Обработчик B3 транслирует его в 429 публичного API.
var ErrQueueFull = errors.New("analysis queue is full")

// Analyzer — синхронный клиент ML; реализуется *ml.Client.
type Analyzer interface {
	Analyze(ctx context.Context, req *domain.AnalysisRequest) (*domain.AnalysisResult, error)
}

// Collector собирает замороженный вход анализа по задаче. Производственная
// реализация — зона B1; подробности стадий сообщает сам коллектор через
// StageReporter, исполнитель их только переносит в хранилище.
type Collector interface {
	Collect(ctx context.Context, job store.Job, area store.Area, report StageReporter) (Collected, error)
}

// StageReporter сообщает фактическую стадию выполняемого модуля.
type StageReporter func(stage string)

// Collected — результат сбора: запрос ML с input_revision и метаданные
// происхождения для публичного результата.
type Collected struct {
	Request    domain.AnalysisRequest
	Provenance map[string]any
}

// Executor — очередь ожидающих задач и один воркер. Состояния и результаты
// принадлежат *store.Store: он же защищает от позднего сохранения (generation
// и состояние job проверяются в PutResult) и переводит незавершённые задачи
// в failed/interrupted при рестарте (FailInterrupted).
type Executor struct {
	store     *store.Store
	collector Collector
	analyzer  Analyzer
	queue     chan string

	mu     sync.Mutex
	active map[string]context.CancelFunc
	queued map[string]bool
}

// New собирает исполнитель с лимитом очереди из контракта.
func New(st *store.Store, collector Collector, analyzer Analyzer) *Executor {
	return &Executor{
		store:     st,
		collector: collector,
		analyzer:  analyzer,
		queue:     make(chan string, queueWaitingLimit),
		active:    map[string]context.CancelFunc{},
		queued:    map[string]bool{},
	}
}

// Start помечает прерванные задачи и запускает единственного воркера.
// Повторный вызов не допускается.
func (e *Executor) Start(ctx context.Context) {
	if err := e.store.FailInterrupted(); err != nil {
		slog.Error("interrupted jobs sweep failed", "error", err)
	}
	go e.worker(ctx)
}

// Enqueue ставит задачу в очередь; вызывается под mutex store обработчиком
// B3 сразу после PutJobQueued и должен оставаться мгновенным. Параметр ctx
// остаётся в сигнатуре ради интерфейса Queue обработчика B3 и на задачу не
// влияет: время её жизни — от постановки до терминального состояния.
// Переполнение — ErrQueueFull без зависшей queued-записи.
func (e *Executor) Enqueue(_ context.Context, jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	select {
	case e.queue <- jobID:
		e.queued[jobID] = true
		return nil
	default:
		return ErrQueueFull
	}
}

// Cancel отменяет ожидание или выполнение задачи и помечает её cancelled
// сразу: у активной задачи отменяется ожидание HTTP, у ожидающей воркера
// ставится терминальный статус. Поздние ответы не сохраняются.
func (e *Executor) Cancel(jobID string) {
	e.mu.Lock()
	cancelFn, ok := e.active[jobID]
	wasQueued := e.queued[jobID]
	delete(e.queued, jobID)
	e.mu.Unlock()

	if ok {
		cancelFn()
	}
	if !ok && !wasQueued {
		return
	}
	if err := e.store.SetJobCancelled(jobID); err != nil && !errors.Is(err, store.ErrBadState) {
		slog.Error("cancel job failed", "job_id", jobID, "error", err)
	}
}

func (e *Executor) worker(base context.Context) {
	for {
		select {
		case <-base.Done():
			return
		case jobID := <-e.queue:
			e.runJob(base, jobID)
		}
	}
}

// bindCancel привязывает отмену задачи ко времени жизни исполнителя и
// снимает её с учёта ожидающих.
func (e *Executor) bindCancel(jobID string, base context.Context) (context.Context, context.CancelFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.queued, jobID)
	ctx, cancel := context.WithCancel(base)
	e.active[jobID] = cancel
	return ctx, cancel
}

func (e *Executor) forgetCancel(jobID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.active, jobID)
}

// setStage переносит стадию в хранилище; false значит «задача уже не running,
// выполнение прекращается». Прочие ошибки логируются, но не срывают задачу.
func (e *Executor) setStage(jobID, stage string) bool {
	if err := e.store.SetJobStage(jobID, stage); err != nil {
		if errors.Is(err, store.ErrBadState) {
			return false
		}
		slog.Error("set stage failed", "job_id", jobID, "stage", stage, "error", err)
	}
	return true
}

// runJob выполняет одну задачу: сбор → ML → сохранение. Каждый шаг
// проверяет отмену; поздний результат не сохраняется (PutResult хранилища
// отклоняет запись по generation и состоянию job).
func (e *Executor) runJob(base context.Context, jobID string) {
	jobCtx, cancel := e.bindCancel(jobID, base)
	defer cancel()
	defer e.forgetCancel(jobID)

	// Отмена до старта или удаление участка: воркер пропускает запись.
	job, err := e.store.GetJob(jobID)
	if err != nil {
		slog.Error("job snapshot unavailable, skipping", "job_id", jobID, "error", err)
		return
	}
	if job.Status != store.JobQueued {
		return
	}
	if err := e.store.SetJobRunning(jobID, domain.StageCollectSatellite); err != nil {
		if errors.Is(err, store.ErrBadState) {
			return
		}
		slog.Error("mark running failed", "job_id", jobID, "error", err)
		return
	}
	area, err := e.store.GetArea(job.AreaID)
	if err != nil {
		e.failSource(jobCtx, jobID, fmt.Sprintf("area %s is unavailable", job.AreaID))
		return
	}

	collected, err := e.collector.Collect(jobCtx, *job, *area, e.reportStage(jobID))
	if err != nil {
		e.failSource(jobCtx, jobID, err.Error())
		return
	}
	if err := e.store.SetJobInputRevision(jobID, collected.Request.InputRevision); err != nil && !errors.Is(err, store.ErrBadState) {
		slog.Error("set input revision failed", "job_id", jobID, "error", err)
	}
	if !e.setStage(jobID, domain.StageAnalyze) {
		return
	}

	result, err := e.analyzer.Analyze(jobCtx, &collected.Request)
	if err != nil {
		e.failAnalyze(jobCtx, jobID, err)
		return
	}
	if jobCtx.Err() != nil {
		e.markCancelled(jobID)
		return
	}

	if !e.setStage(jobID, domain.StageSaveResult) {
		return
	}
	storeResult := mapResult(&collected.Request, collected.Provenance, result)
	// PutResult отклоняет запись, если участок удалён (generation сменился)
	// или задача уже не running: поздний результат не сохраняется.
	if err := e.store.PutResult(job.AreaID, job.AreaGeneration, jobID, storeResult); err != nil {
		switch {
		case errors.Is(err, store.ErrGeneration), errors.Is(err, store.ErrBadState), errors.Is(err, store.ErrNotFound):
			slog.Info("late result discarded", "job_id", jobID, "error", err)
		default:
			slog.Error("save result failed", "job_id", jobID, "error", err)
		}
	}
}

// reportStage возвращает колбэк для коллектора: стадии чужого модуля
// переносятся как есть и только если задача ещё running.
func (e *Executor) reportStage(jobID string) StageReporter {
	return func(stage string) {
		e.setStage(jobID, stage)
	}
}

// failSource помечает ошибку сбора; отмена задачи сильнее ошибки источника.
func (e *Executor) failSource(ctx context.Context, jobID, message string) {
	if ctx.Err() != nil {
		e.markCancelled(jobID)
		return
	}
	if err := e.store.SetJobFailed(jobID, store.JobError{Code: "source_failed", Message: message}); err != nil && !errors.Is(err, store.ErrBadState) {
		slog.Error("mark source failure failed", "job_id", jobID, "error", err)
	}
}

// failAnalyze различает отмену, коды контракта и неожиданный сбой вызова.
func (e *Executor) failAnalyze(ctx context.Context, jobID string, err error) {
	if ctx.Err() != nil {
		e.markCancelled(jobID)
		return
	}
	var mlErr *ml.Error
	code := "ml_internal_error"
	message := "ml returned an unexpected failure"
	retryable := true
	if errors.As(err, &mlErr) {
		code = string(mlErr.Code)
		message = mlErr.Message
		retryable = mlErr.Retryable
	}
	if serr := e.store.SetJobFailed(jobID, store.JobError{Code: code, Message: message, Retryable: retryable}); serr != nil && !errors.Is(serr, store.ErrBadState) {
		slog.Error("mark analyze failure failed", "job_id", jobID, "error", serr)
	}
}

func (e *Executor) markCancelled(jobID string) {
	if err := e.store.SetJobCancelled(jobID); err != nil && !errors.Is(err, store.ErrBadState) {
		slog.Error("mark cancelled failed", "job_id", jobID, "error", err)
	}
}
