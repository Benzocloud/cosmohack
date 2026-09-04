package source

import (
	"errors"
	"fmt"
)

type FailureKind string

const (
	FailureInvalidRequest FailureKind = "invalid_request"
	FailureUnauthorized   FailureKind = "provider_unauthorized"
	FailureRateLimited    FailureKind = "provider_rate_limited"
	FailureUnavailable    FailureKind = "provider_unavailable"
	FailureTimeout        FailureKind = "provider_timeout"
	FailureMalformed      FailureKind = "provider_malformed_response"
	FailureLimitExceeded  FailureKind = "limit_exceeded"
	ErrSettingsLookup                 = "source settings lookup is nil"
	ErrInvalidInteger                 = "source setting %s must be an integer"
	ErrInvalidNumber                  = "source setting %s must be a number"
)

type ProviderError struct {
	kind     FailureKind
	provider string
	detail   string
	cause    error
}

func NewProviderError(kind FailureKind, provider, format string, args ...any) *ProviderError {
	return &ProviderError{kind: kind, provider: provider, detail: fmt.Sprintf(format, args...)}
}

func WrapProviderError(kind FailureKind, provider string, cause error, format string, args ...any) *ProviderError {
	return &ProviderError{kind: kind, provider: provider, detail: fmt.Sprintf(format, args...), cause: cause}
}

func (e *ProviderError) Kind() FailureKind {
	return e.kind
}

func (e *ProviderError) Provider() string {
	return e.provider
}

func (e *ProviderError) Detail() string {
	return e.detail
}

func (e *ProviderError) Retryable() bool {
	switch e.kind {
	case FailureRateLimited, FailureUnavailable, FailureTimeout:
		return true
	default:
		return false
	}
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.provider, e.kind, e.detail)
}

func (e *ProviderError) Unwrap() error {
	return e.cause
}

func KindOf(err error) FailureKind {
	var providerError *ProviderError
	if errors.As(err, &providerError) {
		return providerError.kind
	}
	return ""
}

func IsRetryable(err error) bool {
	var providerError *ProviderError
	return errors.As(err, &providerError) && providerError.Retryable()
}

func KindOfOrUnknown(err error) FailureKind {
	if kind := KindOf(err); kind != "" {
		return kind
	}
	return "unknown_error"
}
