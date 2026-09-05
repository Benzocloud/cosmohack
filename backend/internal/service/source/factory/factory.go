package factory

import (
	"fmt"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/config"
	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/cdse"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/openmeteo"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/overpass"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source"
)

const (
	settingsSecretPlaceholder = "(redacted)"
)

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
	Limits              domainsource.Limits
	Clock               domain.Clock
}

// SettingsFromConfig адаптирует проверенную конфигурацию приложения к настройкам
// источников B1. Чтение окружения выполняется только в internal/config.
func SettingsFromConfig(cfg config.SourceConfig, limits domainsource.Limits, clock domain.Clock) Settings {
	if limits == (domainsource.Limits{}) {
		limits = domainsource.DefaultLimits()
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

type Assembly struct {
	collector *source.Collector
	finder    domainsource.ContourFinder
	limits    domainsource.Limits
}

func New(settings Settings) (*Assembly, error) {
	if settings.Limits == (domainsource.Limits{}) {
		settings.Limits = domainsource.DefaultLimits()
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

func (a *Assembly) ContourFinder() domainsource.ContourFinder {
	return a.finder
}

func (a *Assembly) Limits() domainsource.Limits {
	return a.limits
}
