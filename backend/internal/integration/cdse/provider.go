package cdse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/httpx"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/source"
)

const (
	ProviderName       = "copernicus-dataspace"
	StatisticsEndpoint = "https://sh.dataspace.copernicus.eu/api/v1/statistics"
	SourceID           = "satellite-cdse-s2l2a-ndvi"
	Collection         = "sentinel-2-l2a"
	License            = "Copernicus Sentinel data; условия использования Copernicus Data Space Ecosystem"

	defaultAggregationDays  = 5
	defaultResolutionMeters = 10.0
	defaultMinValidFraction = 0.5
	defaultMosaickingOrder  = "leastCC"
	reasonOutOfRange        = "ndvi_out_of_range"
)

type Config struct {
	StatisticsEndpoint string
	TokenEndpoint      string
	Credentials        Credentials
	Collection         string
	AggregationDays    int
	ResolutionMeters   float64
	MinValidFraction   float64
	MosaickingOrder    string
	Client             *httpx.Client
	Clock              domain.Clock
}

func DefaultConfig(credentials Credentials) Config {
	return Config{
		StatisticsEndpoint: StatisticsEndpoint,
		TokenEndpoint:      TokenEndpoint,
		Credentials:        credentials,
		Collection:         Collection,
		AggregationDays:    defaultAggregationDays,
		ResolutionMeters:   defaultResolutionMeters,
		MinValidFraction:   defaultMinValidFraction,
		MosaickingOrder:    defaultMosaickingOrder,
		Client:             httpx.NewClient(httpx.WithTimeout(120 * time.Second)),
		Clock:              time.Now,
	}
}

type Provider struct {
	endpoint         string
	tokens           *TokenSource
	client           *httpx.Client
	builder          *requestBuilder
	minValidFraction float64
	aggregationDays  int
	collection       string
	mosaickingOrder  string
	resolutionMeters float64
	clock            domain.Clock
}

func NewProvider(config Config) (*Provider, error) {
	if config.StatisticsEndpoint == "" {
		config.StatisticsEndpoint = StatisticsEndpoint
	}
	if config.Collection == "" {
		config.Collection = Collection
	}
	if config.AggregationDays <= 0 {
		config.AggregationDays = defaultAggregationDays
	}
	if config.ResolutionMeters <= 0 {
		config.ResolutionMeters = defaultResolutionMeters
	}
	if config.MinValidFraction <= 0 || config.MinValidFraction > 1 {
		config.MinValidFraction = defaultMinValidFraction
	}
	if config.MosaickingOrder == "" {
		config.MosaickingOrder = defaultMosaickingOrder
	}
	if config.Client == nil {
		config.Client = httpx.NewClient(httpx.WithTimeout(120 * time.Second))
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	tokens, err := NewTokenSource(config.TokenEndpoint, config.Credentials, config.Client, config.Clock)
	if err != nil {
		return nil, err
	}
	return &Provider{
		endpoint: config.StatisticsEndpoint,
		tokens:   tokens,
		client:   config.Client,
		builder: newRequestBuilder(
			config.Collection, config.MosaickingOrder, config.AggregationDays, config.ResolutionMeters),
		minValidFraction: config.MinValidFraction,
		aggregationDays:  config.AggregationDays,
		collection:       config.Collection,
		mosaickingOrder:  config.MosaickingOrder,
		resolutionMeters: config.ResolutionMeters,
		clock:            config.Clock,
	}, nil
}

func (p *Provider) FetchNDVI(ctx context.Context, request source.SatelliteRequest) (source.SatelliteSeries, error) {
	token, err := p.tokens.Token(ctx)
	if err != nil {
		return source.SatelliteSeries{}, err
	}
	body, err := p.builder.build(request)
	if err != nil {
		return source.SatelliteSeries{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return source.SatelliteSeries{}, domain.WrapProviderError(domain.FailureInvalidRequest, ProviderName, err,
			"запрос статистики не построен")
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+token)

	document := &statisticsResponse{}
	if err := p.client.DoJSON(ctx, ProviderName, httpRequest, document); err != nil {
		return source.SatelliteSeries{}, err
	}
	samples, notes, err := document.samples(p.minValidFraction)
	if err != nil {
		return source.SatelliteSeries{}, err
	}
	descriptor, err := source.NewDescriptor(source.DescriptorSpec{
		ID:       SourceID,
		Kind:     source.KindSatellite,
		Provider: ProviderName,
		Dataset:  fmt.Sprintf("Sentinel-2 L2A (%s), CDSE Statistical API v1", p.collection),
		Mapping: fmt.Sprintf(
			"NDVI=(B08-B04)/(B08+B04) как среднее по полигону; исключены классы SCL %v; агрегация P%dD в UTC; разрешение %.0f м; mosaickingOrder=%s; valid_fraction=(sampleCount-noDataCount)/sampleCount; пригодным считается valid_fraction не ниже %.2f",
			maskedSceneClasses, p.aggregationDays, p.resolutionMeters, p.mosaickingOrder, p.minValidFraction),
		License:     source.License(License),
		RetrievedAt: p.clock().UTC(),
	})
	if err != nil {
		return source.SatelliteSeries{}, domain.WrapProviderError(domain.FailureMalformed, ProviderName, err,
			"описание спутникового источника не построено")
	}
	return source.NewSatelliteSeries(descriptor, samples, notes)
}

type requestBuilder struct {
	collection       string
	mosaickingOrder  string
	aggregationDays  int
	resolutionMeters float64
}

func newRequestBuilder(collection, mosaickingOrder string, aggregationDays int, resolutionMeters float64) *requestBuilder {
	return &requestBuilder{
		collection:       collection,
		mosaickingOrder:  mosaickingOrder,
		aggregationDays:  aggregationDays,
		resolutionMeters: resolutionMeters,
	}
}

func (b *requestBuilder) build(request source.SatelliteRequest) ([]byte, error) {
	geometry, err := geom.NewPolygonCodec(0).Encode(request.Polygon())
	if err != nil {
		return nil, domain.WrapProviderError(domain.FailureInvalidRequest, ProviderName, err,
			"геометрия не сериализована")
	}
	payload := map[string]any{
		"input": map[string]any{
			"bounds": map[string]any{
				"geometry":   json.RawMessage(geometry),
				"properties": map[string]any{"crs": "http://www.opengis.net/def/crs/OGC/1.3/CRS84"},
			},
			"data": []map[string]any{{
				"type":       b.collection,
				"dataFilter": map[string]any{"mosaickingOrder": b.mosaickingOrder},
			}},
		},
		"aggregation": map[string]any{
			"timeRange": map[string]string{
				"from": request.Period().From().Time().Format(time.RFC3339),
				"to":   request.Period().To().AddDays(1).Time().Format(time.RFC3339),
			},
			"aggregationInterval": map[string]string{"of": fmt.Sprintf("P%dD", b.aggregationDays)},
			"resx":                fmt.Sprintf("%g", b.resolutionMeters),
			"resy":                fmt.Sprintf("%g", b.resolutionMeters),
			"evalscript":          evalscript(),
		},
		"calculations": map[string]any{
			"ndvi": map[string]any{},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, domain.WrapProviderError(domain.FailureInvalidRequest, ProviderName, err,
			"тело запроса не сериализовано")
	}
	return body, nil
}
