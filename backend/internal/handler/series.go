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

	version, explicitVersion := r.URL.Query()["version"]
	resultVersion := a.ShownResultVersion

	if explicitVersion {
		if len(version) != 1 || version[0] == "" || validateVersion(version[0]) != nil {
			writeValidation(w, errInvalidVersion)
			return
		}

		resultVersion = version[0]
	}

	if resultVersion == "" {
		if series {
			writeJSON(w, http.StatusOK, emptySeries(a.ID))
			return
		}

		writeJSON(w, http.StatusOK, emptyEvents(a.ID))

		return
	}

	res, err := h.storage.GetResult(r.Context(), a.ID, resultVersion)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			if explicitVersion {
				writePublicError(w, http.StatusNotFound, errorCodeNotFound, false)
				return
			}

			writePublicError(w, http.StatusInternalServerError, errorCodeInternal, true)

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
