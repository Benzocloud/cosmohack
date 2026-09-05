package handler

import (
	"errors"
	"net/http"
	"time"

	analysisusecase "github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
)

func (h *handler) postAnalyses(w http.ResponseWriter, r *http.Request) {
	h.gate.Lock()
	defer h.gate.Unlock()
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
	var req createAnalysisRequest
	if !empty {
		req, err = decodeCreateAnalysis(body)
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

	a, err := h.storage.GetArea(r.Context(), id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if a.ActiveJobID != "" {
		j, jerr := h.storage.GetJob(r.Context(), a.ActiveJobID)
		if jerr == nil && analysisusecase.IsActiveStatus(j.Status) {
			writeError(w, http.StatusConflict, "conflict", "Анализ по этому участку уже выполняется", false)
			return
		}
		if jerr != nil && !errors.Is(jerr, errStorageNotFound) {
			writeStoreErr(w, jerr)
			return
		}
		a.ActiveJobID = ""
		if err := h.storage.UpdateArea(r.Context(), a); err != nil {
			writeStoreErr(w, err)
			return
		}
	}

	job, err := analysisusecase.NewJob(a, analysisusecase.ResolvePeriod(a, req.Period), time.Now().UTC())
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if err := h.storage.PutJobQueued(r.Context(), job); err != nil {
		writeStoreErr(w, err)
		return
	}
	if err := h.queue.Enqueue(r.Context(), job.ID); err != nil {
		if deleteErr := h.storage.DeleteJob(r.Context(), job.ID); deleteErr != nil {
			writeStoreErr(w, deleteErr)
			return
		}
		if errors.Is(err, ErrQueueFull) {
			writeError(w, http.StatusTooManyRequests, "queue_full", "Очередь анализа заполнена, повторите позже", true)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Не удалось поставить задачу в очередь", true)
		return
	}
	if req.Period != nil {
		fresh, err := h.storage.GetArea(r.Context(), a.ID)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		fresh.Period = *req.Period
		if err := h.storage.UpdateArea(r.Context(), fresh); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": job.ID})
}
