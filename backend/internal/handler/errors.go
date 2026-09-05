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
		writeError(w, http.StatusNotFound, "not_found", "Объект не найден", false)
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		writeError(w, http.StatusConflict, "conflict", "Операция конфликтует с текущим состоянием", false)
		return
	}
	if errors.Is(err, domain.ErrBadState) {
		writeError(w, http.StatusConflict, "conflict", "Операция недоступна в текущем состоянии", false)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", "Не удалось прочитать или записать снимок", true)
}
