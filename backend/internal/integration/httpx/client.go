package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

const (
	DefaultTimeout          = 30 * time.Second
	DefaultMaxResponseBytes = 8 << 20
	DefaultUserAgent        = "cosmohack-b1/0.1 (+https://github.com/Benzocloud/cosmohack)"
)

type Doer interface {
	Do(request *http.Request) (*http.Response, error)
}

type Option func(*Client)

func WithDoer(doer Doer) Option {
	return func(client *Client) {
		if doer != nil {
			client.doer = doer
		}
	}
}

func WithUserAgent(agent string) Option {
	return func(client *Client) {
		if agent != "" {
			client.userAgent = agent
		}
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(client *Client) {
		if timeout > 0 {
			client.timeout = timeout
		}
	}
}

func WithMaxResponseBytes(limit int64) Option {
	return func(client *Client) {
		if limit > 0 {
			client.maxResponseBytes = limit
		}
	}
}

type Client struct {
	doer             Doer
	userAgent        string
	timeout          time.Duration
	maxResponseBytes int64
}

func NewClient(options ...Option) *Client {
	client := &Client{
		doer:             &http.Client{Timeout: DefaultTimeout},
		userAgent:        DefaultUserAgent,
		timeout:          DefaultTimeout,
		maxResponseBytes: DefaultMaxResponseBytes,
	}
	for _, option := range options {
		option(client)
	}
	return client
}

func (c *Client) DoJSON(ctx context.Context, provider string, request *http.Request, target any) error {
	body, err := c.Do(ctx, provider, request)
	if err != nil {
		return err
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(body, target); err != nil {
		return domain.NewProviderError(domain.FailureMalformed, provider, "ответ не разбирается как JSON")
	}
	return nil
}

func (c *Client) Do(ctx context.Context, provider string, request *http.Request) ([]byte, error) {
	requestContext, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	request = request.WithContext(requestContext)
	request.Header.Set("User-Agent", c.userAgent)
	if request.Header.Get("Accept") == "" {
		request.Header.Set("Accept", "application/json")
	}

	response, err := c.doer.Do(request)
	if err != nil {
		return nil, transportError(requestContext, provider, err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(response.Body, c.maxResponseBytes+1))
	if err != nil {
		return nil, domain.WrapProviderError(domain.FailureUnavailable, provider, err, "ответ не прочитан")
	}
	if int64(len(body)) > c.maxResponseBytes {
		return nil, domain.NewProviderError(domain.FailureMalformed, provider,
			"ответ больше предела %d байт", c.maxResponseBytes)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, domain.NewProviderError(statusKind(response.StatusCode), provider,
			"HTTP %d", response.StatusCode)
	}
	return body, nil
}

func transportError(ctx context.Context, provider string, err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return domain.WrapProviderError(domain.FailureUnavailable, provider, err, "запрос отменён")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(ctx.Err(), context.DeadlineExceeded):
		return domain.WrapProviderError(domain.FailureTimeout, provider, err, "истёк тайм-аут запроса")
	default:
		return domain.WrapProviderError(domain.FailureUnavailable, provider, err, "соединение не установлено")
	}
}

func statusKind(status int) domain.FailureKind {
	switch {
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return domain.FailureUnauthorized
	case status == http.StatusTooManyRequests:
		return domain.FailureRateLimited
	case status == http.StatusRequestTimeout, status == http.StatusGatewayTimeout:
		return domain.FailureTimeout
	case status >= 500:
		return domain.FailureUnavailable
	default:
		return domain.FailureInvalidRequest
	}
}
