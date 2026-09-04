package openmeteo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/domain/geo"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/httpx"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

const (
	ProviderName     = "open-meteo"
	PrimaryEndpoint  = "https://archive-api.open-meteo.com/v1/archive"
	FallbackEndpoint = "https://archive-api.open-meteo.com/v1/era5"
	SourceID         = "weather-open-meteo-era5"
	Model            = "era5"
	License          = "CC-BY-4.0 Open-Meteo; ERA5 — Copernicus Climate Change Service"

	temperatureVariable       = "temperature_2m_mean"
	precipitationVariable     = "precipitation_sum"
	expectedTemperatureUnit   = "°C"
	expectedPrecipitationUnit = "mm"
	expectedOffsetSeconds     = 0
)

type Config struct {
	PrimaryEndpoint  string
	FallbackEndpoint string
	Model            string
	Client           *httpx.Client
	Clock            domain.Clock
}

func DefaultConfig() Config {
	return Config{
		PrimaryEndpoint:  PrimaryEndpoint,
		FallbackEndpoint: FallbackEndpoint,
		Model:            Model,
		Client:           httpx.NewClient(httpx.WithTimeout(30 * time.Second)),
		Clock:            time.Now,
	}
}

type Provider struct {
	failover *httpx.Failover
	model    string
	clock    domain.Clock
}

func NewProvider(config Config) (*Provider, error) {
	if config.PrimaryEndpoint == "" {
		config.PrimaryEndpoint = PrimaryEndpoint
	}
	if config.Model == "" {
		config.Model = Model
	}
	if config.Client == nil {
		config.Client = httpx.NewClient(httpx.WithTimeout(30 * time.Second))
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	endpoints, err := httpx.NewEndpoints(config.PrimaryEndpoint, config.FallbackEndpoint)
	if err != nil {
		return nil, err
	}
	failover, err := httpx.NewFailover(config.Client, endpoints)
	if err != nil {
		return nil, err
	}
	return &Provider{failover: failover, model: config.Model, clock: config.Clock}, nil
}

func (p *Provider) FetchDaily(ctx context.Context, request source.WeatherRequest) (source.WeatherSeries, error) {
	document := &responseDocument{}
	if _, err := p.failover.DoJSON(ctx, ProviderName, p.requestFactory(request), document); err != nil {
		return source.WeatherSeries{}, err
	}
	if err := document.validate(); err != nil {
		return source.WeatherSeries{}, err
	}
	cell, err := geom.NewCoordinate(document.Longitude, document.Latitude)
	if err != nil {
		return source.WeatherSeries{}, domain.WrapProviderError(domain.FailureMalformed, ProviderName, err,
			"ячейка реанализа вне допустимых координат")
	}
	days, notes, err := document.days(request.Period())
	if err != nil {
		return source.WeatherSeries{}, err
	}
	descriptor, err := source.NewDescriptor(source.DescriptorSpec{
		ID:       SourceID,
		Kind:     source.KindWeather,
		Provider: ProviderName,
		Dataset:  fmt.Sprintf("ERA5 reanalysis, archive-api v1, models=%s", p.model),
		Mapping: fmt.Sprintf(
			"суточная агрегация UTC; ячейка реанализа longitude=%.4f latitude=%.4f, высота %.0f м; %s в %s; %s в %s; значения относятся к ячейке реанализа, а не к измерению на поле",
			cell.Lon(), cell.Lat(), document.Elevation,
			temperatureVariable, expectedTemperatureUnit,
			precipitationVariable, expectedPrecipitationUnit),
		License:     source.License(License),
		RetrievedAt: p.clock().UTC(),
	})
	if err != nil {
		return source.WeatherSeries{}, domain.WrapProviderError(domain.FailureMalformed, ProviderName, err,
			"описание погодного источника не построено")
	}
	return source.NewWeatherSeries(descriptor, cell, days, notes)
}

func (p *Provider) requestFactory(request source.WeatherRequest) httpx.RequestFactory {
	return func(ctx context.Context, endpoint string) (*http.Request, error) {
		query := url.Values{}
		query.Set("latitude", strconv.FormatFloat(request.Point().Lat(), 'f', 6, 64))
		query.Set("longitude", strconv.FormatFloat(request.Point().Lon(), 'f', 6, 64))
		query.Set("start_date", request.Period().From().String())
		query.Set("end_date", request.Period().To().String())
		query.Set("daily", temperatureVariable+","+precipitationVariable)
		query.Set("timezone", "UTC")
		query.Set("models", p.model)
		return http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	}
}
