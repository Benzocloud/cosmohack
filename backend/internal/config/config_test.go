package config

import "testing"

func TestLoadFrom(t *testing.T) {
	values := map[string]string{
		EnvDBURL:                     "postgres://localhost/cosmohack",
		EnvDBTimeout:                 "2s",
		EnvSatelliteAggregationDays:  "7",
		EnvSatelliteMinValidFraction: "0.4",
		EnvAnalysisWorkers:           "3",
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
	if c.HTTP.Addr != defaultHTTPAddr || c.Analysis.Workers != 3 || c.Analysis.QueueSize != 12 {
		t.Fatalf("defaults = %+v %+v", c.HTTP, c.Analysis)
	}
}

func TestLoadFromRequiresDatabase(t *testing.T) {
	if _, err := LoadFrom(func(string) (string, bool) { return "", false }); err == nil {
		t.Fatal("LoadFrom() accepted missing DATABASE_URL")
	}
}
