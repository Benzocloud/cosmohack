package handler

import (
	"errors"
	"net/http"

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

	job, err := h.scheduler.Start(r.Context(), id, req.Period)
	if err != nil {
		if errors.Is(err, analysisusecase.ErrConflict) {
			writeError(w, http.StatusConflict, "conflict", "Анализ по этому участку уже выполняется", false)
			return
		}
		if errors.Is(err, ErrQueueFull) {
			writeError(w, http.StatusTooManyRequests, "queue_full", "Очередь анализа заполнена, повторите позже", true)
			return
		}
		writePersistenceErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": job.ID})
}
