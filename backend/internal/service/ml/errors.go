package ml

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// Error — ошибка вызова ML с кодом из контракта. Message безопасен для
// пользователя и попадает в задачу; Err хранит техническую причину для логов.
type Error struct {
	Code      domain.MLErrorCode
	Message   string
	Retryable bool
	Err       error
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = "ml call failed"
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, msg, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, msg)
}

func (e *Error) Unwrap() error { return e.Err }

func newError(code domain.MLErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

func wrapError(code domain.MLErrorCode, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

// classifyTransportError различает отмену задачи, тайм-аут и недоступность.
// Отмена возвращается как есть: исполнитель отличает её по context.Canceled.
func classifyTransportError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	timeoutErr := &Error{Code: domain.MLErrorTimeout, Message: "ml did not respond in time", Err: err}
	if errors.Is(err, context.DeadlineExceeded) {
		return timeoutErr
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return timeoutErr
	}
	return &Error{Code: domain.MLErrorUnavailable, Message: "ml is unreachable", Err: err}
}

// mlErrorRule — строка таблицы обработки ошибок HTTP-контракта.
type mlErrorRule struct {
	status int
	mlCode string
	goCode domain.MLErrorCode
}

// mlErrorRules — отображение статуса и кода ML в код задачи Go.
var mlErrorRules = []mlErrorRule{
	{http.StatusBadRequest, "invalid_json", domain.MLErrorInvalidRequest},
	{http.StatusRequestEntityTooLarge, "payload_too_large", domain.MLErrorInputTooLarge},
	{http.StatusUnsupportedMediaType, "unsupported_media_type", domain.MLErrorInvalidRequest},
	{http.StatusUnprocessableEntity, "invalid_input", domain.MLErrorInvalidRequest},
	{http.StatusUnprocessableEntity, "unsupported_contract", domain.MLErrorContractMismatch},
	{http.StatusTooManyRequests, "busy", domain.MLErrorBusy},
	{http.StatusServiceUnavailable, "not_ready", domain.MLErrorUnavailable},
	{http.StatusInternalServerError, "internal_error", domain.MLErrorInternal},
}

// mlErrorBody — описание ошибки внутри ответа ML. Message безопасен для
// пользователя; retryable не разрешает автоматический повтор текущего POST.
type mlErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// mlErrorResponse — конверт ошибки POST /v1/analyze. RequestID равен null,
// только если ID нельзя извлечь из некорректного тела.
type mlErrorResponse struct {
	SchemaVersion string      `json:"schema_version"`
	RequestID     *string     `json:"request_id"`
	Error         mlErrorBody `json:"error"`
}

// mapHTTPError переводит неуспешный ответ ML в ошибку контракта. Тело без
// ожидаемого конверта, чужой request_id, неизвестный код или статус —
// domain.MLErrorInvalidResponse.
func mapHTTPError(status int, requestID string, body []byte) *Error {
	var envelope mlErrorResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return newError(domain.MLErrorInvalidResponse,
			"ml returned a non-ok status with a malformed error body")
	}
	if envelope.RequestID != nil && *envelope.RequestID != requestID {
		return newError(domain.MLErrorInvalidResponse, "ml returned a foreign request_id in the error")
	}
	if envelope.SchemaVersion != domain.SchemaVersionV1 {
		return newError(domain.MLErrorInvalidResponse, "ml returned an unexpected schema_version in the error")
	}
	for _, rule := range mlErrorRules {
		if rule.status == status && rule.mlCode == envelope.Error.Code {
			return &Error{
				Code:      rule.goCode,
				Message:   envelope.Error.Message,
				Retryable: envelope.Error.Retryable,
			}
		}
	}
	return newError(domain.MLErrorInvalidResponse,
		fmt.Sprintf("ml returned unexpected status %d with code %q", status, envelope.Error.Code))
}
