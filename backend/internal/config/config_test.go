package config

import "testing"

func TestLoadFrom(t *testing.T) {
	values := map[string]string{
		EnvDBURL:                     "postgres://localhost/cosmohack",
		EnvLogLevel:                  "debug",
		EnvDBTimeout:                 "2s",
		EnvSatelliteAggregationDays:  "7",
		EnvSatelliteMinValidFraction: "0.4",
		EnvAnalysisQueueSize:         "12",
	}
	c, err := LoadFrom(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if c.Postgres.URL != values[EnvDBURL] || c.Postgres.Timeout.String() != "2s" {
		t.Fatalf("postgres config = %+v", c.Postgres)
	}
	if c.Source.AggregationDays != 7 || c.Source.MinValidFraction != 0.4 {
		t.Fatalf("source config = %+v", c.Source)
	}
	if c.Source.CDSEStatisticsURL != DefaultCDSEStatisticsURL || c.Source.WeatherURL != DefaultWeatherURL {
		t.Fatalf("source defaults = %+v", c.Source)
	}
	if c.HTTP.Addr != defaultHTTPAddr || c.LogLevel != "debug" || c.Analysis.QueueSize != 12 {
		t.Fatalf("defaults = %+v %+v", c.HTTP, c.Analysis)
	}
}

func TestLoadFromUsesDefaultLogLevel(t *testing.T) {
	values := map[string]string{EnvDBURL: "postgres://localhost/cosmohack"}
	c, err := LoadFrom(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if c.LogLevel != defaultLogLevel {
		t.Fatalf("log level = %q, want %q", c.LogLevel, defaultLogLevel)
	}
}

func TestLoadFromRejectsInvalidQueueSize(t *testing.T) {
	values := map[string]string{EnvDBURL: "postgres://localhost/cosmohack", EnvAnalysisQueueSize: "0"}
	_, err := LoadFrom(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	if err == nil {
		t.Fatal("LoadFrom() accepted a non-positive analysis queue size")
	}
}

func TestLoadFromUsesDefaultQueueSize(t *testing.T) {
	values := map[string]string{EnvDBURL: "postgres://localhost/cosmohack"}
	c, err := LoadFrom(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if c.Analysis.QueueSize != defaultAnalysisQueueSize {
		t.Fatalf("queue size = %d, want %d", c.Analysis.QueueSize, defaultAnalysisQueueSize)
	}
}

func TestLoadFromRequiresDatabase(t *testing.T) {
	if _, err := LoadFrom(func(string) (string, bool) { return "", false }); err == nil {
		t.Fatal("LoadFrom() accepted missing DATABASE_URL")
	}
}
