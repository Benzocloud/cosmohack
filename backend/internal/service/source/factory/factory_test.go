package factory_test

import (
	"strings"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/integration/cdse"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/openmeteo"
	"github.com/Benzocloud/cosmohack/backend/internal/integration/overpass"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/factory"
)

func lookupFrom(values map[string]string) factory.Lookup {
	return func(key string) (string, bool) {
		value, found := values[key]
		return value, found
	}
}

func TestSettingsFromEnvUsesDocumentedDefaults(t *testing.T) {
	settings, err := factory.SettingsFromEnv(lookupFrom(map[string]string{}))
	if err != nil {
		t.Fatalf("settings were not built: %v", err)
	}
	if settings.CDSEStatisticsURL != cdse.StatisticsEndpoint || settings.CDSETokenURL != cdse.TokenEndpoint {
		t.Fatalf("CDSE endpoints %s / %s", settings.CDSEStatisticsURL, settings.CDSETokenURL)
	}
	if settings.OverpassURL != overpass.PrimaryEndpoint || settings.OverpassFallbackURL != overpass.FallbackEndpoint {
		t.Fatalf("Overpass endpoints %s / %s", settings.OverpassURL, settings.OverpassFallbackURL)
	}
	if settings.WeatherURL != openmeteo.PrimaryEndpoint || settings.WeatherFallbackURL != openmeteo.FallbackEndpoint {
		t.Fatalf("weather endpoints %s / %s", settings.WeatherURL, settings.WeatherFallbackURL)
	}
}

func TestSettingsFromEnvRejectsInvalidNumbers(t *testing.T) {
	cases := map[string]map[string]string{
		"aggregation interval": {factory.EnvAggregationDays: "five"},
		"valid fraction":       {factory.EnvMinValidFraction: "half"},
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := factory.SettingsFromEnv(lookupFrom(values)); err == nil {
				t.Fatal("invalid setting was accepted")
			}
		})
	}
}

func TestSettingsHideSecret(t *testing.T) {
	settings, err := factory.SettingsFromEnv(lookupFrom(map[string]string{
		factory.EnvCDSEClientID:     "client",
		factory.EnvCDSEClientSecret: "very-secret-value",
	}))
	if err != nil {
		t.Fatalf("settings were not built: %v", err)
	}
	if strings.Contains(settings.String(), "very-secret-value") {
		t.Fatal("secret leaked into settings string")
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	settings, err := factory.SettingsFromEnv(lookupFrom(map[string]string{}))
	if err != nil {
		t.Fatalf("settings were not built: %v", err)
	}
	if _, err := factory.New(settings); err == nil {
		t.Fatal("assembly succeeded without CDSE credentials")
	}
}

func TestNewBuildsCollectorAndFinder(t *testing.T) {
	settings, err := factory.SettingsFromEnv(lookupFrom(map[string]string{
		factory.EnvCDSEClientID:     "client",
		factory.EnvCDSEClientSecret: "secret",
		factory.EnvOverpassURL:      "https://overpass.example.org/api/interpreter",
		factory.EnvWeatherURL:       "https://weather.example.org/v1/archive",
		factory.EnvAggregationDays:  "7",
		factory.EnvMinValidFraction: "0.4",
	}))
	if err != nil {
		t.Fatalf("settings were not built: %v", err)
	}
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
	settings, err := factory.SettingsFromEnv(lookupFrom(map[string]string{
		factory.EnvCDSEClientID:     "client",
		factory.EnvCDSEClientSecret: "secret",
		factory.EnvOverpassURL:      "overpass-without-scheme",
	}))
	if err != nil {
		t.Fatalf("settings were not built: %v", err)
	}
	if _, err := factory.New(settings); err == nil {
		t.Fatal("invalid provider endpoint was accepted")
	}
}
