package factory

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/config"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/cdse"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/openmeteo"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/overpass"
)

const (
	EnvCDSEClientID           = "CDSE_CLIENT_ID"
	EnvCDSEClientSecret       = "CDSE_CLIENT_SECRET"
	EnvCDSEStatisticsURL      = "CDSE_STATISTICS_URL"
	EnvCDSETokenURL           = "CDSE_TOKEN_URL"
	EnvOverpassURL            = "OVERPASS_URL"
	EnvOverpassFallbackURL    = "OVERPASS_FALLBACK_URL"
	EnvWeatherURL             = "WEATHER_URL"
	EnvWeatherFallbackURL     = "WEATHER_FALLBACK_URL"
	EnvAggregationDays        = "SATELLITE_AGGREGATION_DAYS"
	EnvMinValidFraction       = "SATELLITE_MIN_VALID_FRACTION"
	settingsSecretPlaceholder = "(скрыто)"
)

type Lookup func(key string) (string, bool)

type Settings struct {
	CDSEClientID        string
	CDSEClientSecret    string
	CDSEStatisticsURL   string
	CDSETokenURL        string
	OverpassURL         string
	OverpassFallbackURL string
	WeatherURL          string
	WeatherFallbackURL  string
	AggregationDays     int
	MinValidFraction    float64
	Limits              source.Limits
	Clock               source.Clock
}

// SettingsFromConfig adapts validated application configuration to B1 source
// settings. Environment lookup belongs exclusively to internal/config.
func SettingsFromConfig(cfg config.SourceConfig, limits source.Limits, clock source.Clock) Settings {
	if limits == (source.Limits{}) {
		limits = source.DefaultLimits()
	}
	if clock == nil {
		clock = time.Now
	}
	return Settings{
		CDSEClientID: cfg.CDSEClientID, CDSEClientSecret: cfg.CDSEClientSecret,
		CDSEStatisticsURL: cfg.CDSEStatisticsURL, CDSETokenURL: cfg.CDSETokenURL,
		OverpassURL: cfg.OverpassURL, OverpassFallbackURL: cfg.OverpassFallbackURL,
		WeatherURL: cfg.WeatherURL, WeatherFallbackURL: cfg.WeatherFallbackURL,
		AggregationDays: cfg.AggregationDays, MinValidFraction: cfg.MinValidFraction,
		Limits: limits, Clock: clock,
	}
}

func (s Settings) String() string {
	return fmt.Sprintf("Settings{cdse_client_id=%q, cdse_client_secret=%s, statistics=%q, overpass=%q, weather=%q}",
		s.CDSEClientID, settingsSecretPlaceholder, s.CDSEStatisticsURL, s.OverpassURL, s.WeatherURL)
}

func SettingsFromEnv(lookup Lookup) (Settings, error) {
	if lookup == nil {
		return Settings{}, errors.New(source.ErrSettingsLookup)
	}
	settings := Settings{
		CDSEClientID:        value(lookup, EnvCDSEClientID, ""),
		CDSEClientSecret:    value(lookup, EnvCDSEClientSecret, ""),
		CDSEStatisticsURL:   value(lookup, EnvCDSEStatisticsURL, cdse.StatisticsEndpoint),
		CDSETokenURL:        value(lookup, EnvCDSETokenURL, cdse.TokenEndpoint),
		OverpassURL:         value(lookup, EnvOverpassURL, overpass.PrimaryEndpoint),
		OverpassFallbackURL: value(lookup, EnvOverpassFallbackURL, overpass.FallbackEndpoint),
		WeatherURL:          value(lookup, EnvWeatherURL, openmeteo.PrimaryEndpoint),
		WeatherFallbackURL:  value(lookup, EnvWeatherFallbackURL, openmeteo.FallbackEndpoint),
		Limits:              source.DefaultLimits(),
		Clock:               time.Now,
	}
	days, err := integer(lookup, EnvAggregationDays, 0)
	if err != nil {
		return Settings{}, err
	}
	settings.AggregationDays = days
	fraction, err := fractional(lookup, EnvMinValidFraction, 0)
	if err != nil {
		return Settings{}, err
	}
	settings.MinValidFraction = fraction
	return settings, nil
}

type Assembly struct {
	collector *source.Collector
	finder    source.ContourFinder
	limits    source.Limits
}

func New(settings Settings) (*Assembly, error) {
	if settings.Limits == (source.Limits{}) {
		settings.Limits = source.DefaultLimits()
	}
	if settings.Clock == nil {
		settings.Clock = time.Now
	}
	credentials, err := cdse.NewCredentials(settings.CDSEClientID, settings.CDSEClientSecret)
	if err != nil {
		return nil, err
	}
	satelliteConfig := cdse.DefaultConfig(credentials)
	satelliteConfig.StatisticsEndpoint = settings.CDSEStatisticsURL
	satelliteConfig.TokenEndpoint = settings.CDSETokenURL
	satelliteConfig.AggregationDays = settings.AggregationDays
	satelliteConfig.MinValidFraction = settings.MinValidFraction
	satelliteConfig.Clock = settings.Clock
	satellite, err := cdse.NewProvider(satelliteConfig)
	if err != nil {
		return nil, err
	}

	weatherConfig := openmeteo.DefaultConfig()
	weatherConfig.PrimaryEndpoint = settings.WeatherURL
	weatherConfig.FallbackEndpoint = settings.WeatherFallbackURL
	weatherConfig.Clock = settings.Clock
	weather, err := openmeteo.NewProvider(weatherConfig)
	if err != nil {
		return nil, err
	}

	contourConfig := overpass.DefaultConfig()
	contourConfig.PrimaryEndpoint = settings.OverpassURL
	contourConfig.FallbackEndpoint = settings.OverpassFallbackURL
	contourConfig.Limits = settings.Limits
	contourConfig.Clock = settings.Clock
	finder, err := overpass.NewFinder(contourConfig)
	if err != nil {
		return nil, err
	}

	collector, err := source.NewCollector(satellite, weather,
		source.WithLimits(settings.Limits), source.WithClock(settings.Clock))
	if err != nil {
		return nil, err
	}
	return &Assembly{collector: collector, finder: finder, limits: settings.Limits}, nil
}

func (a *Assembly) Collector() *source.Collector {
	return a.collector
}

func (a *Assembly) ContourFinder() source.ContourFinder {
	return a.finder
}

func (a *Assembly) Limits() source.Limits {
	return a.limits
}

func value(lookup Lookup, key, fallback string) string {
	if found, ok := lookup(key); ok && found != "" {
		return found
	}
	return fallback
}

func integer(lookup Lookup, key string, fallback int) (int, error) {
	raw, ok := lookup(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf(source.ErrInvalidInteger, key)
	}
	return parsed, nil
}

func fractional(lookup Lookup, key string, fallback float64) (float64, error) {
	raw, ok := lookup(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf(source.ErrInvalidNumber, key)
	}
	return parsed, nil
}
