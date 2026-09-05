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
)

// queueWaitingLimit — начальный лимит ожидающих задач; активная задача
// в лимит не входит. Изменение согласуют B4 и ML и фиксируют в README.
const queueWaitingLimit = 8

// ErrQueueFull — очередь ожидающих заполнена; повтор запуска явный.
// Обработчик B3 транслирует его в 429 публичного API.
var ErrQueueFull = errors.New("analysis queue is full")

// ErrBadState indicates that a concurrent cancellation or terminal transition
// already won the job race.
var ErrBadState = domain.ErrBadState

// ErrConflict indicates that an area already has an active analysis job.
var ErrConflict = errors.New("analysis already active")

// ErrNotFound indicates that a job or area disappeared before persistence.
var ErrNotFound = domain.ErrNotFound

// ErrGeneration indicates that an area changed during analysis.
var ErrGeneration = domain.ErrGeneration

// Persistence is the consumer-owned storage port of the executor.
type Persistence interface {
	GetJob(context.Context, string) (domain.Job, error)
	GetArea(context.Context, string) (domain.Area, error)
	SetJobRunning(context.Context, string, string) error
	SetJobStage(context.Context, string, string) error
	SetJobFailed(context.Context, string, string, string, bool) error
	SetJobCancelled(context.Context, string) error
	SetJobInputRevision(context.Context, string, string) error
	PutResult(context.Context, int, string, domain.AnalysisRecord) error
	RecoverInterrupted(context.Context) error
}

// Analyzer — синхронный клиент ML; реализуется *ml.Client.
type Analyzer interface {
	Analyze(ctx context.Context, req *domain.AnalysisRequest) (*domain.AnalysisResult, error)
}

// Collector собирает замороженный вход анализа по задаче. Производственная
// реализация — зона B1; подробности стадий сообщает сам коллектор через
// StageReporter, исполнитель их только переносит в хранилище.
type Collector interface {
	Collect(ctx context.Context, job domain.Job, area domain.Area, report StageReporter) (Collected, error)
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
// принадлежат реализации Persistence: она защищает позднее сохранение и
// переводит незавершённые задачи в failed/interrupted при рестарте.
type Executor struct {
	persistence Persistence
	collector   Collector
	analyzer    Analyzer
	queue       chan string

	mu     sync.Mutex
	active map[string]context.CancelFunc
	queued map[string]bool
}

// New собирает исполнитель с лимитом очереди из контракта.
func New(persistence Persistence, collector Collector, analyzer Analyzer) *Executor {
	return &Executor{
		persistence: persistence,
		collector:   collector,
		analyzer:    analyzer,
		queue:       make(chan string, queueWaitingLimit),
		active:      map[string]context.CancelFunc{},
		queued:      map[string]bool{},
	}
}

// Start помечает прерванные задачи и запускает единственного воркера.
// Повторный вызов не допускается.
func (e *Executor) Start(ctx context.Context) error {
	if err := e.persistence.RecoverInterrupted(ctx); err != nil {
		return fmt.Errorf("recover interrupted jobs: %w", err)
	}
	go e.worker(ctx)
	return nil
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
	if err := e.persistence.SetJobCancelled(context.Background(), jobID); err != nil && !errors.Is(err, ErrBadState) {
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
func (e *Executor) setStage(ctx context.Context, jobID, stage string) bool {
	if err := e.persistence.SetJobStage(ctx, jobID, stage); err != nil {
		if errors.Is(err, ErrBadState) {
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
	job, err := e.persistence.GetJob(jobCtx, jobID)
	if err != nil {
		slog.Error("job snapshot unavailable, skipping", "job_id", jobID, "error", err)
		return
	}
	if job.Status != domain.JobQueued {
		return
	}
	if err := e.persistence.SetJobRunning(jobCtx, jobID, domain.StageCollectSatellite); err != nil {
		if errors.Is(err, ErrBadState) {
			return
		}
		slog.Error("mark running failed", "job_id", jobID, "error", err)
		return
	}
	area, err := e.persistence.GetArea(jobCtx, job.AreaID)
	if err != nil {
		e.failSource(jobCtx, jobID, fmt.Sprintf("area %s is unavailable", job.AreaID))
		return
	}

	collected, err := e.collector.Collect(jobCtx, job, area, e.reportStage(jobCtx, jobID))
	if err != nil {
		e.failSource(jobCtx, jobID, err.Error())
		return
	}
	if err := e.persistence.SetJobInputRevision(jobCtx, jobID, collected.Request.InputRevision); err != nil && !errors.Is(err, ErrBadState) {
		slog.Error("set input revision failed", "job_id", jobID, "error", err)
	}
	if !e.setStage(jobCtx, jobID, domain.StageAnalyze) {
		return
	}

	result, err := e.analyzer.Analyze(jobCtx, &collected.Request)
	if err != nil {
		e.failAnalyze(jobCtx, jobID, err)
		return
	}
	if jobCtx.Err() != nil {
		e.markCancelled(jobCtx, jobID)
		return
	}

	if !e.setStage(jobCtx, jobID, domain.StageSaveResult) {
		return
	}
	domainResult := mapResult(&collected.Request, collected.Provenance, result)
	// PutResult отклоняет запись, если участок удалён (generation сменился)
	// или задача уже не running: поздний результат не сохраняется.
	if err := e.persistence.PutResult(jobCtx, job.AreaGeneration, jobID, domainResult); err != nil {
		switch {
		case errors.Is(err, ErrGeneration), errors.Is(err, ErrBadState), errors.Is(err, ErrNotFound):
			slog.Info("late result discarded", "job_id", jobID, "error", err)
		default:
			slog.Error("save result failed", "job_id", jobID, "error", err)
		}
	}
}

// reportStage возвращает колбэк для коллектора: стадии чужого модуля
// переносятся как есть и только если задача ещё running.
func (e *Executor) reportStage(ctx context.Context, jobID string) StageReporter {
	return func(stage string) {
		e.setStage(ctx, jobID, stage)
	}
}

// failSource помечает ошибку сбора; отмена задачи сильнее ошибки источника.
func (e *Executor) failSource(ctx context.Context, jobID, message string) {
	if ctx.Err() != nil {
		e.markCancelled(ctx, jobID)
		return
	}
	if err := e.persistence.SetJobFailed(ctx, jobID, "source_failed", message, false); err != nil && !errors.Is(err, ErrBadState) {
		slog.Error("mark source failure failed", "job_id", jobID, "error", err)
	}
}

// failAnalyze различает отмену, коды контракта и неожиданный сбой вызова.
func (e *Executor) failAnalyze(ctx context.Context, jobID string, err error) {
	if ctx.Err() != nil {
		e.markCancelled(ctx, jobID)
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
	if serr := e.persistence.SetJobFailed(ctx, jobID, code, message, retryable); serr != nil && !errors.Is(serr, ErrBadState) {
		slog.Error("mark analyze failure failed", "job_id", jobID, "error", serr)
	}
}

func (e *Executor) markCancelled(ctx context.Context, jobID string) {
	if err := e.persistence.SetJobCancelled(ctx, jobID); err != nil && !errors.Is(err, ErrBadState) {
		slog.Error("mark cancelled failed", "job_id", jobID, "error", err)
	}
}
