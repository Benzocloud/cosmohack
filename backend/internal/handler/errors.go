package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

const (
	errorCodeInvalidJSON       = "invalid_json"
	errorCodeInvalidGeometry   = "invalid_geometry"
	errorCodeInvalidBBox       = "invalid_bbox"
	errorCodeInvalidPeriod     = "invalid_period"
	errorCodeInvalidName       = "invalid_name"
	errorCodeInvalidSource     = "invalid_source"
	errorCodeLimitExceeded     = "limit_exceeded"
	errorCodeInvalidVersion    = "invalid_version"
	errorCodeNotFound          = "not_found"
	errorCodeConflict          = "conflict"
	errorCodeQueueFull         = "queue_full"
	errorCodeSourceUnavailable = "source_unavailable"
	errorCodeInternal          = "internal_error"
)

var publicErrorMessages = map[string]string{
	errorCodeInvalidJSON:       "request body must be a JSON object",
	errorCodeInvalidGeometry:   "polygon must be closed and valid",
	errorCodeInvalidBBox:       "bbox must contain valid coordinates",
	errorCodeInvalidPeriod:     "period from/to must be valid dates",
	errorCodeInvalidName:       "area name is invalid",
	errorCodeInvalidSource:     "area source is invalid",
	errorCodeLimitExceeded:     "area exceeds the configured limits",
	errorCodeInvalidVersion:    "result version is invalid",
	errorCodeNotFound:          "object was not found",
	errorCodeConflict:          "operation conflicts with the current state",
	errorCodeQueueFull:         "analysis queue is full; retry later",
	errorCodeSourceUnavailable: "contour source is unavailable",
	errorCodeInternal:          "an internal server error occurred",
}

func publicErrorMessage(code string) string { return publicErrorMessages[code] }

func writePublicError(w http.ResponseWriter, status int, code string, retryable bool) {
	writeError(w, status, code, publicErrorMessage(code), retryable)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string, retryable bool) {
	writeJSON(w, status, errorEnvelope{Error: errorBody{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}})
}

func writeValidation(w http.ResponseWriter, err error) {
	code, msg, retry := validationMessage(err)

	status := http.StatusBadRequest
	if code == "not_found" {
		status = http.StatusNotFound
	}

	if code == "internal_error" {
		status = http.StatusInternalServerError
	}

	writeError(w, status, code, msg, retry)
}

func writePersistenceErr(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	if errors.Is(err, domain.ErrNotFound) {
		writePublicError(w, http.StatusNotFound, errorCodeNotFound, false)
		return
	}

	if errors.Is(err, domain.ErrConflict) {
		writePublicError(w, http.StatusConflict, errorCodeConflict, false)
		return
	}

	if errors.Is(err, domain.ErrBadState) {
		writePublicError(w, http.StatusConflict, errorCodeConflict, false)
		return
	}

	writePublicError(w, http.StatusInternalServerError, errorCodeInternal, true)
}
