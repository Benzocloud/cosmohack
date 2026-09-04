package source

import "github.com/Benzocloud/cosmohack/backend/internal/domain"

type FailureKind = domain.FailureKind
type ProviderError = domain.ProviderError

const (
	FailureInvalidRequest = domain.FailureInvalidRequest
	FailureLimitExceeded  = domain.FailureLimitExceeded
)

var NewProviderError = domain.NewProviderError
