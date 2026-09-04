package factory_test

import (
	"strings"
	"testing"

	"github.com/Benzocloud/cosmohack/backend/internal/service/source/cdse"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/factory"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/openmeteo"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/overpass"
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
		t.Fatalf("настройки не собраны: %v", err)
	}
	if settings.CDSEStatisticsURL != cdse.StatisticsEndpoint || settings.CDSETokenURL != cdse.TokenEndpoint {
		t.Fatalf("адреса CDSE %s / %s", settings.CDSEStatisticsURL, settings.CDSETokenURL)
	}
	if settings.OverpassURL != overpass.PrimaryEndpoint || settings.OverpassFallbackURL != overpass.FallbackEndpoint {
		t.Fatalf("адреса Overpass %s / %s", settings.OverpassURL, settings.OverpassFallbackURL)
	}
	if settings.WeatherURL != openmeteo.PrimaryEndpoint || settings.WeatherFallbackURL != openmeteo.FallbackEndpoint {
		t.Fatalf("адреса погоды %s / %s", settings.WeatherURL, settings.WeatherFallbackURL)
	}
}

func TestSettingsFromEnvRejectsInvalidNumbers(t *testing.T) {
	cases := map[string]map[string]string{
		"интервал агрегации": {factory.EnvAggregationDays: "пять"},
		"доля пригодности":   {factory.EnvMinValidFraction: "половина"},
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := factory.SettingsFromEnv(lookupFrom(values)); err == nil {
				t.Fatal("некорректное значение переменной принято")
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
		t.Fatalf("настройки не собраны: %v", err)
	}
	if strings.Contains(settings.String(), "very-secret-value") {
		t.Fatal("секрет попадает в строковое представление настроек")
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	settings, err := factory.SettingsFromEnv(lookupFrom(map[string]string{}))
	if err != nil {
		t.Fatalf("настройки не собраны: %v", err)
	}
	if _, err := factory.New(settings); err == nil {
		t.Fatal("сборка без доступа к CDSE выполнена")
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
		t.Fatalf("настройки не собраны: %v", err)
	}
	assembly, err := factory.New(settings)
	if err != nil {
		t.Fatalf("сборка не выполнена: %v", err)
	}
	if assembly.Collector() == nil || assembly.ContourFinder() == nil {
		t.Fatal("сборка вернула неполный набор источников")
	}
	if assembly.Limits().MaxObservations() <= 0 {
		t.Fatal("пределы сборки не заданы")
	}
}

func TestNewRejectsBrokenEndpoint(t *testing.T) {
	settings, err := factory.SettingsFromEnv(lookupFrom(map[string]string{
		factory.EnvCDSEClientID:     "client",
		factory.EnvCDSEClientSecret: "secret",
		factory.EnvOverpassURL:      "overpass-without-scheme",
	}))
	if err != nil {
		t.Fatalf("настройки не собраны: %v", err)
	}
	if _, err := factory.New(settings); err == nil {
		t.Fatal("некорректный адрес источника принят")
	}
}
