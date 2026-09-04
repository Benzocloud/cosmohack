package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
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
		return source.NewProviderError(source.FailureMalformed, provider, "ответ не разбирается как JSON")
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
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, c.maxResponseBytes+1))
	if err != nil {
		return nil, source.WrapProviderError(source.FailureUnavailable, provider, err, "ответ не прочитан")
	}
	if int64(len(body)) > c.maxResponseBytes {
		return nil, source.NewProviderError(source.FailureMalformed, provider,
			"ответ больше предела %d байт", c.maxResponseBytes)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, source.NewProviderError(statusKind(response.StatusCode), provider,
			"HTTP %d", response.StatusCode)
	}
	return body, nil
}

func transportError(ctx context.Context, provider string, err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return source.WrapProviderError(source.FailureUnavailable, provider, err, "запрос отменён")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(ctx.Err(), context.DeadlineExceeded):
		return source.WrapProviderError(source.FailureTimeout, provider, err, "истёк тайм-аут запроса")
	default:
		return source.WrapProviderError(source.FailureUnavailable, provider, err, "соединение не установлено")
	}
}

func statusKind(status int) source.FailureKind {
	switch {
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return source.FailureUnauthorized
	case status == http.StatusTooManyRequests:
		return source.FailureRateLimited
	case status == http.StatusRequestTimeout, status == http.StatusGatewayTimeout:
		return source.FailureTimeout
	case status >= 500:
		return source.FailureUnavailable
	default:
		return source.FailureInvalidRequest
	}
}
