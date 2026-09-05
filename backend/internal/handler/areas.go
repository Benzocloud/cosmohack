package handler

import (
	"net/http"

	"github.com/Benzocloud/cosmohack/backend/internal/service/area"
)

func (h *handler) listAreas(w http.ResponseWriter, r *http.Request) {
	areas, err := h.areas.ListAreas(r.Context())
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := make([]publicArea, 0, len(areas))
	for _, a := range areas {
		p, err := h.projectArea(r.Context(), a)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"areas": out})
}

func (h *handler) createArea(w http.ResponseWriter, r *http.Request) {
	body, empty, err := readBody(w, r)
	if err != nil {
		writeValidation(w, err)
		return
	}
	if empty {
		writeValidation(w, errInvalidJSON)
		return
	}
	req, err := decodeCreateArea(body)
	if err != nil {
		writeValidation(w, err)
		return
	}
	if err := validateCreate(req, h.limits); err != nil {
		writeValidation(w, err)
		return
	}
	domainArea, err := h.areas.CreateArea(r.Context(), area.CreateInput{
		Name: req.Name, Period: req.Period, Geometry: req.Geometry, Source: *req.Source,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Не удалось прочитать или записать снимок", true)
		return
	}
	p, err := h.projectArea(r.Context(), domainArea)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *handler) deleteArea(w http.ResponseWriter, r *http.Request) {
	h.gate.Lock()
	defer h.gate.Unlock()
	id := r.PathValue("id")
	if err := validateID(id); err != nil {
		writeValidation(w, err)
		return
	}
	cancelIDs, err := h.areas.DeleteArea(r.Context(), id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if c, ok := h.queue.(Canceller); ok {
		for _, jobID := range cancelIDs {
			c.Cancel(jobID)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
