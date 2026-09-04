package cdse

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/httpx"
)

const (
	TokenEndpoint     = "https://identity.dataspace.copernicus.eu/auth/realms/CDSE/protocol/openid-connect/token"
	credentialsHidden = "cdse-credentials(скрыто)"
	expiryMargin      = 30 * time.Second
)

type Credentials struct {
	clientID     string
	clientSecret string
}

func NewCredentials(clientID, clientSecret string) (Credentials, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return Credentials{}, fmt.Errorf("client_id и client_secret CDSE должны быть заданы")
	}
	return Credentials{clientID: clientID, clientSecret: clientSecret}, nil
}

func (c Credentials) ClientID() string {
	return c.clientID
}

func (c Credentials) IsZero() bool {
	return c.clientID == "" || c.clientSecret == ""
}

func (c Credentials) String() string {
	return credentialsHidden
}

func (c Credentials) GoString() string {
	return credentialsHidden
}

func (c Credentials) form() url.Values {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	return form
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type TokenSource struct {
	endpoint    string
	credentials Credentials
	client      *httpx.Client
	clock       source.Clock

	mutex     sync.Mutex
	token     string
	expiresAt time.Time
}

func NewTokenSource(endpoint string, credentials Credentials, client *httpx.Client, clock source.Clock) (*TokenSource, error) {
	if endpoint == "" {
		endpoint = TokenEndpoint
	}
	if credentials.IsZero() {
		return nil, fmt.Errorf("доступ к CDSE не настроен")
	}
	if client == nil {
		client = httpx.NewClient()
	}
	if clock == nil {
		clock = time.Now
	}
	return &TokenSource{endpoint: endpoint, credentials: credentials, client: client, clock: clock}, nil
}

func (t *TokenSource) Token(ctx context.Context) (string, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	now := t.clock()
	if t.token != "" && now.Before(t.expiresAt) {
		return t.token, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, strings.NewReader(t.credentials.form().Encode()))
	if err != nil {
		return "", source.WrapProviderError(source.FailureInvalidRequest, ProviderName, err, "запрос токена не построен")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	payload := &tokenResponse{}
	if err := t.client.DoJSON(ctx, ProviderName, request, payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" || payload.ExpiresIn <= 0 {
		return "", source.NewProviderError(source.FailureMalformed, ProviderName, "ответ сервиса токенов неполон")
	}
	t.token = payload.AccessToken
	t.expiresAt = now.Add(time.Duration(payload.ExpiresIn)*time.Second - expiryMargin)
	return t.token, nil
}
