package handler

import (
	"net/http"
)

func (h *handler) getContours(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("bbox")
	if raw == "" {
		writeValidation(w, errInvalidBBox)
		return
	}
	b, err := parseBBox(raw)
	if err != nil {
		writeValidation(w, err)
		return
	}
	items, err := h.contours.Find(r.Context(), b.MinLon, b.MinLat, b.MaxLon, b.MaxLat)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "source_unavailable", "Не удалось получить контуры", true)
		return
	}
	if items == nil {
		items = []Contour{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"contours": items})
}

func (h *handler) getJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validateID(id); err != nil {
		writeValidation(w, err)
		return
	}
	j, err := h.storage.GetJob(r.Context(), id)
	if err != nil {
		writePersistenceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projectJob(j))
}
