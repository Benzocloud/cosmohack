package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	analysisusecase "github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

type analysisReply struct {
	status int
	body   any
	code   string
	msg    string
	retry  bool
}

func (h *handler) postAnalyses(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validateID(id); err != nil {
		writeValidation(w, err)
		return
	}
	body, empty, err := readBody(w, r)
	if err != nil {
		writeValidation(w, err)
		return
	}
	var req analysesRequest
	if !empty {
		req, err = decodeAnalyses(body)
		if err != nil {
			writeValidation(w, err)
			return
		}
		if req.Period != nil {
			if err := validatePeriod(*req.Period); err != nil {
				writeValidation(w, err)
				return
			}
		}
	}

	var reply analysisReply
	// Enqueue вызывается под mutex store: DELETE не вклинивается между PutJobQueued и каналом.
	err = h.store.WithLock(func(tx *store.Tx) error {
		a, err := tx.Area(id)
		if err != nil {
			return err
		}
		if a.ActiveJobID != "" {
			j, jerr := tx.Job(a.ActiveJobID)
			if jerr == nil && analysisusecase.IsActiveStatus(domain.JobStatus(j.Status)) {
				reply = analysisReply{
					status: http.StatusConflict,
					code:   "conflict",
					msg:    "Анализ по этому участку уже выполняется",
				}
				return nil
			}
			if errors.Is(jerr, store.ErrNotFound) || (jerr == nil && !analysisusecase.IsActiveStatus(domain.JobStatus(j.Status))) {
				a.ActiveJobID = ""
				if err := tx.PutArea(*a); err != nil {
					return err
				}
			} else if jerr != nil {
				return jerr
			}
		}
		period := store.Period(analysisusecase.ResolvePeriod(
			domain.Area{Period: domain.Period(a.Period)}, req.Period,
		))
		domainJob, err := analysisusecase.NewJob(domain.Area{
			ID: a.ID, Generation: a.Generation,
		}, domain.Period(period), time.Now().UTC())
		if err != nil {
			return err
		}
		job := storeJobFromDomain(domainJob)
		if err := tx.PutJobQueued(job); err != nil {
			return err
		}
		if err := h.queue.Enqueue(r.Context(), job.ID); err != nil {
			if derr := tx.DeleteJob(job.ID); derr != nil {
				return derr
			}
			if errors.Is(err, ErrQueueFull) {
				reply = analysisReply{
					status: http.StatusTooManyRequests,
					code:   "queue_full",
					msg:    "Очередь анализа заполнена, повторите позже",
					retry:  true,
				}
				return nil
			}
			reply = analysisReply{
				status: http.StatusInternalServerError,
				code:   "internal_error",
				msg:    "Не удалось поставить задачу в очередь",
				retry:  true,
			}
			return nil
		}
		if req.Period != nil {
			fresh, ferr := tx.Area(a.ID)
			if ferr == nil {
				fresh.Period = store.Period(*req.Period)
				_ = tx.PutArea(*fresh)
			}
		}
		reply = analysisReply{
			status: http.StatusAccepted,
			body:   map[string]string{"job_id": job.ID},
		}
		return nil
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if reply.code != "" {
		writeError(w, reply.status, reply.code, reply.msg, reply.retry)
		return
	}
	writeJSON(w, reply.status, reply.body)
}
