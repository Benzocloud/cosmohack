package domain

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
)

type ProviderError struct {
	KindValue                  FailureKind
	ProviderValue, DetailValue string
	Cause                      error
}

func NewProviderError(kind FailureKind, provider, format string, args ...any) *ProviderError {
	return &ProviderError{KindValue: kind, ProviderValue: provider, DetailValue: fmt.Sprintf(format, args...)}
}
func WrapProviderError(kind FailureKind, provider string, cause error, format string, args ...any) *ProviderError {
	return &ProviderError{KindValue: kind, ProviderValue: provider, DetailValue: fmt.Sprintf(format, args...), Cause: cause}
}
func (e *ProviderError) Kind() FailureKind { return e.KindValue }
func (e *ProviderError) Provider() string  { return e.ProviderValue }
func (e *ProviderError) Detail() string    { return e.DetailValue }
func (e *ProviderError) Retryable() bool {
	return e.KindValue == FailureRateLimited || e.KindValue == FailureUnavailable || e.KindValue == FailureTimeout
}
func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.ProviderValue, e.KindValue, e.DetailValue)
}
func (e *ProviderError) Unwrap() error { return e.Cause }
func KindOf(err error) FailureKind {
	var e *ProviderError
	if errors.As(err, &e) {
		return e.KindValue
	}
	return ""
}
func IsRetryable(err error) bool { var e *ProviderError; return errors.As(err, &e) && e.Retryable() }
func KindOfOrUnknown(err error) FailureKind {
	if k := KindOf(err); k != "" {
		return k
	}
	return "unknown_error"
}
