package handler

import (
	"errors"
	"net/http"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

func (h *handler) getSeries(w http.ResponseWriter, r *http.Request) {
	h.writeResultView(w, r, true)
}

func (h *handler) getEvents(w http.ResponseWriter, r *http.Request) {
	h.writeResultView(w, r, false)
}

func (h *handler) writeResultView(w http.ResponseWriter, r *http.Request, series bool) {
	id := r.PathValue("id")
	if err := validateID(id); err != nil {
		writeValidation(w, err)
		return
	}
	a, err := h.storage.GetArea(r.Context(), id)
	if err != nil {
		writePersistenceErr(w, err)
		return
	}
	if a.ShownResultVersion == "" {
		if series {
			writeJSON(w, http.StatusOK, emptySeries(a.ID))
			return
		}
		writeJSON(w, http.StatusOK, emptyEvents(a.ID))
		return
	}
	res, err := h.storage.GetResult(r.Context(), a.ID, a.ShownResultVersion)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "internal_error", "Не удалось прочитать или записать снимок", true)
			return
		}
		writePersistenceErr(w, err)
		return
	}
	if series {
		writeJSON(w, http.StatusOK, projectSeries(res))
		return
	}
	writeJSON(w, http.StatusOK, projectEvents(res))
}
