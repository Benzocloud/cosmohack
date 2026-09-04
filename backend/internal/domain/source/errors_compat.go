package source

import "github.com/Benzocloud/cosmohack/backend/internal/domain"

type FailureKind = domain.FailureKind
type ProviderError = domain.ProviderError

const (
	FailureInvalidRequest = domain.FailureInvalidRequest
	FailureLimitExceeded  = domain.FailureLimitExceeded
	FailureUnauthorized   = domain.FailureUnauthorized
	FailureRateLimited    = domain.FailureRateLimited
	FailureUnavailable    = domain.FailureUnavailable
	FailureTimeout        = domain.FailureTimeout
	FailureMalformed      = domain.FailureMalformed
)

var NewProviderError = domain.NewProviderError
var WrapProviderError = domain.WrapProviderError
var KindOf = domain.KindOf
var IsRetryable = domain.IsRetryable
var KindOfOrUnknown = domain.KindOfOrUnknown
