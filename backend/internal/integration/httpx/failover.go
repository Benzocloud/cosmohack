package httpx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

type Endpoints struct {
	primary  string
	fallback string
}

func NewEndpoints(primary, fallback string) (Endpoints, error) {
	if err := validateEndpoint("основной адрес", primary); err != nil {
		return Endpoints{}, err
	}
	if fallback != "" {
		if err := validateEndpoint("резервный адрес", fallback); err != nil {
			return Endpoints{}, err
		}
	}
	return Endpoints{primary: primary, fallback: fallback}, nil
}

func (e Endpoints) Primary() string {
	return e.primary
}

func (e Endpoints) Fallback() string {
	return e.fallback
}

func (e Endpoints) HasFallback() bool {
	return e.fallback != ""
}

type RequestFactory func(ctx context.Context, endpoint string) (*http.Request, error)

type Failover struct {
	client    *Client
	endpoints Endpoints
}

func NewFailover(client *Client, endpoints Endpoints) (*Failover, error) {
	if client == nil {
		return nil, fmt.Errorf("резервный вызов без HTTP-клиента")
	}
	return &Failover{client: client, endpoints: endpoints}, nil
}

func (f *Failover) DoJSON(ctx context.Context, provider string, factory RequestFactory, target any) (string, error) {
	endpoint := f.endpoints.Primary()
	err := f.attempt(ctx, provider, endpoint, factory, target)
	if err == nil {
		return endpoint, nil
	}
	if !f.endpoints.HasFallback() || !domain.IsRetryable(err) {
		return endpoint, err
	}
	fallback := f.endpoints.Fallback()
	fallbackErr := f.attempt(ctx, provider, fallback, factory, target)
	if fallbackErr == nil {
		return fallback, nil
	}
	return fallback, domain.WrapProviderError(domain.KindOfOrUnknown(fallbackErr), provider, fallbackErr,
		"основной адрес отказал (%v), резервный адрес отказал", err)
}

func (f *Failover) attempt(ctx context.Context, provider, endpoint string, factory RequestFactory, target any) error {
	request, err := factory(ctx, endpoint)
	if err != nil {
		return domain.WrapProviderError(domain.FailureInvalidRequest, provider, err, "запрос не построен")
	}
	return f.client.DoJSON(ctx, provider, request, target)
}

func validateEndpoint(label, value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s не разбирается: %w", label, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s должен использовать http или https", label)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s не содержит хоста", label)
	}
	return nil
}
