// Package ml — HTTP-клиент ML-сервиса по контракту v1
// (.agent/contracts/go-ml-http.md): POST /v1/analyze и GET /readyz.
//
// Клиент проверяет вход до отправки и ответ до возврата: несовпадение версий,
// чужой request_id, пропущенные даты и неожиданная модель дают
// domain.MLErrorInvalidResponse и такой результат не сохраняется как успешный.
// Адрес сервиса задаётся конфигурацией оператора и не принимается из
// пользовательского ввода.
package ml

import (
	"errors"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// Предопределённые ошибки конфигурации клиента.
var (
	errInvalidTimeouts   = errors.New("client timeouts must be positive")
	errInvalidBodyLimits = errors.New("body size limits must be positive")
)

// Config — настройки клиента. Значения по умолчанию соответствуют начальным
// лимитам контракта v1; изменения согласуют B4 и ML.
type Config struct {
	// BaseURL — базовый адрес ML (например, http://ml:8000 в Compose).
	BaseURL string
	// DialTimeout — предел установления соединения Go → ML.
	DialTimeout time.Duration
	// AnalyzeTimeout — предел полного вызова анализа, включая чтение тела.
	AnalyzeTimeout time.Duration
	// ReadyTimeout — предел вызова readiness.
	ReadyTimeout time.Duration
	// MaxRequestBodyBytes — предел тела запроса.
	MaxRequestBodyBytes int
	// MaxResponseBodyBytes — предел тела ответа.
	MaxResponseBodyBytes int
	// ExpectedModelVersion — ожидаемая версия модели из манифеста выпуска;
	// пустое значение отключает проверку.
	ExpectedModelVersion string
}

// DefaultConfig возвращает настройки по умолчанию для заданного адреса ML.
func DefaultConfig(baseURL string) Config {
	return Config{
		BaseURL:              baseURL,
		DialTimeout:          3 * time.Second,
		AnalyzeTimeout:       120 * time.Second,
		ReadyTimeout:         2 * time.Second,
		MaxRequestBodyBytes:  domain.MaxRequestBodyBytes,
		MaxResponseBodyBytes: domain.MaxResponseBodyBytes,
	}
}

func (c Config) validate() error {
	if err := validateBaseURL(c.BaseURL); err != nil {
		return err
	}
	if c.DialTimeout <= 0 || c.AnalyzeTimeout <= 0 || c.ReadyTimeout <= 0 {
		return errInvalidTimeouts
	}
	if c.MaxRequestBodyBytes <= 0 || c.MaxResponseBodyBytes <= 0 {
		return errInvalidBodyLimits
	}
	return nil
}
