package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"slices"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// pathAnalyze и pathReady — маршруты контракта v1.
const (
	pathAnalyze = "/v1/analyze"
	pathReady   = "/readyz"
)

// Client — HTTP-клиент ML-сервиса для одного фонового воркера Go.
type Client struct {
	cfg  Config
	http *http.Client
}

// New создаёт клиента с ограничением времени соединения. Полный лимит вызова
// задаётся контекстом каждого запроса, а не общим тайм-аутом клиента.
func New(cfg Config) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			// Сжатие в v1 контрактом не предусмотрено; лимит размера
			// применяется к телу ответа без распаковки.
			Transport: &http.Transport{
				DialContext:         (&net.Dialer{Timeout: cfg.DialTimeout}).DialContext,
				TLSHandshakeTimeout: cfg.DialTimeout,
				DisableCompression:  true,
			},
		},
	}, nil
}

// Ready возвращает состояние готовности ML. Для статуса 503 возвращается
// тело с Status="not_ready" и nil-ошибкой: caller различает их по полю.
func (c *Client) Ready(ctx context.Context) (domain.ReadyInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.ReadyTimeout)
	defer cancel()

	meta, body, err := c.do(ctx, http.MethodGet, pathReady, nil)
	if err != nil {
		return domain.ReadyInfo{}, err
	}

	switch meta.statusCode {
	case http.StatusOK, http.StatusServiceUnavailable:
		if err := checkJSONContentType(meta.contentType); err != nil {
			return domain.ReadyInfo{}, err
		}
		var info domain.ReadyInfo
		if err := json.Unmarshal(body, &info); err != nil {
			return domain.ReadyInfo{}, wrapError(domain.MLErrorInvalidResponse,
				"ml returned a malformed readiness body", err)
		}

		switch meta.statusCode {
		case http.StatusOK:
			if err := validateReadyBody(info, c.cfg.ExpectedModelVersion); err != nil {
				return domain.ReadyInfo{}, err
			}
		case http.StatusServiceUnavailable:
			if info.Status != domain.MLNotReadyStatus {
				return domain.ReadyInfo{}, newError(domain.MLErrorInvalidResponse,
					"ml returned an unexpected readiness status with 503")
			}

			if !nonEmptyPtr(info.Reason) {
				return domain.ReadyInfo{}, newError(domain.MLErrorInvalidResponse,
					"ml returned 503 readiness without a reason")
			}
		}
		return info, nil
	default:
		return domain.ReadyInfo{}, newError(domain.MLErrorInvalidResponse,
			fmt.Sprintf("ml readiness returned unexpected status %d", meta.statusCode))
	}
}

// Analyze выполняет синхронный расчёт: проверка входа, POST /v1/analyze,
// проверка ответа. Успешный результат гарантированно соответствует контракту
// и эху запроса; отмена контекста возвращает context.Canceled как есть.
func (c *Client) Analyze(ctx context.Context, req *domain.AnalysisRequest) (*domain.AnalysisResult, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	effectiveReq := req
	if req.SchemaVersion == domain.SchemaVersionV11 {
		var err error
		effectiveReq, err = c.negotiateV11(ctx, req)
		if err != nil {
			return nil, err
		}
	}
	payload, err := json.Marshal(effectiveReq)
	if err != nil {
		return nil, wrapError(domain.MLErrorInvalidRequest, "analyze request is not encodable to json", err)
	}
	maxRequestBodyBytes := c.cfg.MaxRequestBodyBytes
	if effectiveReq.FeatureProfile == domain.FeatureProfileMultisensorV1 {
		maxRequestBodyBytes = c.cfg.MaxMultisensorRequestBodyBytes
	}
	if len(payload) > maxRequestBodyBytes {
		return nil, newError(domain.MLErrorInputTooLarge, "request body exceeds the limit")
	}

	ctx, cancel := context.WithTimeout(ctx, c.cfg.AnalyzeTimeout)
	defer cancel()

	meta, body, err := c.do(ctx, http.MethodPost, pathAnalyze, payload)
	if err != nil {
		return nil, err
	}

	if meta.statusCode != http.StatusOK {
		// Контракт требует JSON и для ответов с ошибкой.
		if err := checkJSONContentType(meta.contentType); err != nil {
			return nil, err
		}

		return nil, mapHTTPError(meta.statusCode, effectiveReq.RequestID, body)
	}

	if err := checkJSONContentType(meta.contentType); err != nil {
		return nil, err
	}

	var result domain.AnalysisResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, wrapError(domain.MLErrorInvalidResponse, "ml returned invalid result json", err)
	}
	if err := validateResult(effectiveReq, &result, c.cfg.ExpectedModelVersion); err != nil {
		return nil, err
	}
	return &result, nil
}

// negotiateV11 выбирает v1.1 только после явного объявления readiness о схеме
// и запрошенном профиле. Ошибка согласования, кроме отмены, возвращает корректный
// запрос v1.0 без расширенных полей.
func (c *Client) negotiateV11(ctx context.Context, req *domain.AnalysisRequest) (*domain.AnalysisRequest, error) {
	info, err := c.Ready(ctx)
	if err == nil && supportsRequest(info, req.FeatureProfile) {
		return req, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return v1FallbackRequest(req), nil
}

func supportsRequest(info domain.ReadyInfo, profile string) bool {
	return info.Status == domain.MLReadyStatus &&
		slices.Contains(info.SchemaVersions, domain.SchemaVersionV11) &&
		slices.Contains(info.FeatureProfiles, profile)
}

func v1FallbackRequest(req *domain.AnalysisRequest) *domain.AnalysisRequest {
	fallback := *req
	fallback.SchemaVersion = domain.SchemaVersionV1
	fallback.FeatureProfile = domain.FeatureProfileNDVIWeatherV1
	fallback.AreaContext = nil
	fallback.Peers = nil
	fallback.Observations = make([]domain.Observation, len(req.Observations))
	copy(fallback.Observations, req.Observations)
	for i := range fallback.Observations {
		fallback.Observations[i].Indices = nil
	}
	return &fallback
}

type responseMeta struct {
	statusCode  int
	contentType string
}

// do выполняет один HTTP-вызов и читает тело с ограничением размера.
func (c *Client) do(ctx context.Context, method, path string, payload []byte) (responseMeta, []byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, method, joinURL(c.cfg.BaseURL, path), bytes.NewReader(payload))
	if err != nil {
		return responseMeta{}, nil, wrapError(domain.MLErrorInvalidRequest, "request to ml could not be created", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return responseMeta{}, nil, classifyTransportError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readLimited(resp.Body, c.cfg.MaxResponseBodyBytes)
	if err != nil {
		return responseMeta{}, nil, err
	}

	return responseMeta{statusCode: resp.StatusCode, contentType: resp.Header.Get("Content-Type")}, body, nil
}

// readLimited читает тело ответа и запрещает превышение лимита контракта.
func readLimited(r io.Reader, limit int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, wrapError(domain.MLErrorInvalidResponse, "ml response body could not be read", err)
	}
	if len(data) > limit {
		return nil, newError(domain.MLErrorInvalidResponse, "response body exceeds the limit")
	}
	return data, nil
}

// checkJSONContentType требует точный media type application/json.
func checkJSONContentType(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return newError(domain.MLErrorInvalidResponse, "ml responded with a non-json content type")
	}
	return nil
}

// validateReadyBody проверяет успешное тело readiness: готовность, версии
// контракта, профиля и модели против манифеста выпуска.
func validateReadyBody(info domain.ReadyInfo, expectedModelVersion string) error {
	if info.Status != domain.MLReadyStatus {
		return newError(domain.MLErrorInvalidResponse, "ml readiness returned an unexpected status")
	}
	if info.SchemaVersion != domain.SchemaVersionV1 {
		return newError(domain.MLErrorInvalidResponse, "ml readiness returned an unexpected schema_version")
	}
	if !slices.Contains(info.FeatureProfiles, domain.FeatureProfileNDVIWeatherV1) {
		return newError(domain.MLErrorInvalidResponse, "ml readiness does not support the contract feature profile")
	}
	seenVersions := make(map[string]bool, len(info.SchemaVersions))
	for _, version := range info.SchemaVersions {
		if version != domain.SchemaVersionV1 && version != domain.SchemaVersionV11 {
			return newError(domain.MLErrorInvalidResponse, "ml readiness returned an unsupported schema version")
		}
		if seenVersions[version] {
			return newError(domain.MLErrorInvalidResponse, "ml readiness returned duplicate schema versions")
		}
		seenVersions[version] = true
	}
	if len(info.SchemaVersions) > 0 && !seenVersions[domain.SchemaVersionV1] {
		return newError(domain.MLErrorInvalidResponse, "ml readiness does not support schema_version 1.0")
	}
	if info.ModelVersion == "" {
		return newError(domain.MLErrorInvalidResponse, "ml readiness did not report a model version")
	}
	if expectedModelVersion != "" && info.ModelVersion != expectedModelVersion {
		return newError(domain.MLErrorInvalidResponse, "ml model version does not match the release manifest")
	}
	return nil
}
