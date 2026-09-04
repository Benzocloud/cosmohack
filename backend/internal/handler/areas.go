package handler

import (
	"net/http"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

func (h *handler) listAreas(w http.ResponseWriter, r *http.Request) {
	areas, err := h.store.ListAreas()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := make([]publicArea, 0, len(areas))
	for _, a := range areas {
		p, err := h.projectArea(a)
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
	src := *req.Source
	id, err := newUUID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Не удалось прочитать или записать снимок", true)
		return
	}
	a := store.Area{
		ID:         id,
		Name:       req.Name,
		Geometry:   req.Geometry,
		Source:     src,
		Period:     req.Period,
		CreatedAt:  time.Now().UTC(),
		Generation: 1,
	}
	if err := h.store.PutArea(a); err != nil {
		writeStoreErr(w, err)
		return
	}
	p, err := h.projectArea(a)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *handler) deleteArea(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validateID(id); err != nil {
		writeValidation(w, err)
		return
	}
	var cancelIDs []string
	err := h.store.WithLock(func(tx *store.Tx) error {
		jobs, err := tx.Jobs()
		if err != nil {
			return err
		}
		for _, j := range jobs {
			if j.AreaID == id && (j.Status == store.JobQueued || j.Status == store.JobRunning) {
				cancelIDs = append(cancelIDs, j.ID)
			}
		}
		return tx.DeleteArea(id)
	})
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
