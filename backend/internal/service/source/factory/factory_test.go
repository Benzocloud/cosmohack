package factory_test

import (
	"strings"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/config"
	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/factory"
)

func settings(overrides func(*config.SourceConfig)) factory.Settings {
	cfg := config.SourceConfig{
		CDSEStatisticsURL:   config.DefaultCDSEStatisticsURL,
		CDSETokenURL:        config.DefaultCDSETokenURL,
		OverpassURL:         config.DefaultOverpassURL,
		OverpassFallbackURL: config.DefaultOverpassFallbackURL,
		WeatherURL:          config.DefaultWeatherURL,
		WeatherFallbackURL:  config.DefaultWeatherFallbackURL,
		AggregationDays:     5,
		MinValidFraction:    0.5,
	}
	overrides(&cfg)
	return factory.SettingsFromConfig(cfg, domainsource.Limits{}, nil)
}

func TestSettingsFromConfigPreservesSourceConfig(t *testing.T) {
	cfg := config.SourceConfig{
		CDSEClientID:        "client",
		CDSEClientSecret:    "very-secret-value",
		CDSEStatisticsURL:   "https://cdse.example/statistics",
		CDSETokenURL:        "https://cdse.example/token",
		OverpassURL:         "https://overpass.example/primary",
		OverpassFallbackURL: "https://overpass.example/fallback",
		WeatherURL:          "https://weather.example/primary",
		WeatherFallbackURL:  "https://weather.example/fallback",
		AggregationDays:     7,
		MinValidFraction:    0.4,
	}
	got := factory.SettingsFromConfig(cfg, domainsource.Limits{}, nil)
	if got.CDSEClientID != cfg.CDSEClientID || got.CDSEClientSecret != cfg.CDSEClientSecret ||
		got.CDSEStatisticsURL != cfg.CDSEStatisticsURL || got.CDSETokenURL != cfg.CDSETokenURL ||
		got.OverpassURL != cfg.OverpassURL || got.OverpassFallbackURL != cfg.OverpassFallbackURL ||
		got.WeatherURL != cfg.WeatherURL || got.WeatherFallbackURL != cfg.WeatherFallbackURL ||
		got.AggregationDays != cfg.AggregationDays || got.MinValidFraction != cfg.MinValidFraction {
		t.Fatalf("settings were not adapted: %+v", got)
	}
}

func TestSettingsHideSecret(t *testing.T) {
	settings := settings(func(cfg *config.SourceConfig) {
		cfg.CDSEClientID = "client"
		cfg.CDSEClientSecret = "very-secret-value"
	})
	if strings.Contains(settings.String(), "very-secret-value") {
		t.Fatal("secret leaked into settings string")
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	if _, err := factory.New(settings(func(*config.SourceConfig) {})); err == nil {
		t.Fatal("assembly succeeded without CDSE credentials")
	}
}

func TestNewBuildsCollectorAndFinder(t *testing.T) {
	settings := settings(func(cfg *config.SourceConfig) {
		cfg.CDSEClientID = "client"
		cfg.CDSEClientSecret = "secret"
		cfg.OverpassURL = "https://overpass.example.org/api/interpreter"
		cfg.WeatherURL = "https://weather.example.org/v1/archive"
		cfg.AggregationDays = 7
		cfg.MinValidFraction = 0.4
	})
	var err error
	assembly, err := factory.New(settings)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}
	if assembly.Collector() == nil || assembly.ContourFinder() == nil {
		t.Fatal("assembly returned incomplete providers")
	}
	if assembly.Limits().MaxObservations() <= 0 {
		t.Fatal("assembly limits are not configured")
	}
}

func TestNewRejectsBrokenEndpoint(t *testing.T) {
	settings := settings(func(cfg *config.SourceConfig) {
		cfg.CDSEClientID = "client"
		cfg.CDSEClientSecret = "secret"
		cfg.OverpassURL = "overpass-without-scheme"
	})
	if _, err := factory.New(settings); err == nil {
		t.Fatal("invalid provider endpoint was accepted")
	}
}
