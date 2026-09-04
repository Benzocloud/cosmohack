package source

import "github.com/Benzocloud/cosmohack/backend/internal/domain"

type FailureKind = domain.FailureKind
type ProviderError = domain.ProviderError

const (
	FailureInvalidRequest = domain.FailureInvalidRequest
	FailureUnauthorized   = domain.FailureUnauthorized
	FailureRateLimited    = domain.FailureRateLimited
	FailureUnavailable    = domain.FailureUnavailable
	FailureTimeout        = domain.FailureTimeout
	FailureMalformed      = domain.FailureMalformed
	FailureLimitExceeded  = domain.FailureLimitExceeded
	ErrSettingsLookup     = domain.ErrSettingsLookup
	ErrInvalidInteger     = domain.ErrInvalidInteger
	ErrInvalidNumber      = domain.ErrInvalidNumber
)

var NewProviderError = domain.NewProviderError
var WrapProviderError = domain.WrapProviderError
var KindOf = domain.KindOf
var IsRetryable = domain.IsRetryable
var KindOfOrUnknown = domain.KindOfOrUnknown
