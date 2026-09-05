// Пакет config загружает и проверяет конфигурацию процесса в корне композиции.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvHTTPAddr                  = "HTTP_ADDR"
	EnvPublicDir                 = "PUBLIC_DIR"
	EnvLogLevel                  = "LOG_LEVEL"
	EnvMLBaseURL                 = "ML_BASE_URL"
	EnvMLModelVersion            = "ML_MODEL_VERSION"
	EnvCDSEClientID              = "CDSE_CLIENT_ID"
	EnvCDSEClientSecret          = "CDSE_CLIENT_SECRET"
	EnvCDSEStatisticsURL         = "CDSE_STATISTICS_URL"
	EnvCDSETokenURL              = "CDSE_TOKEN_URL"
	EnvOverpassURL               = "OVERPASS_URL"
	EnvOverpassFallbackURL       = "OVERPASS_FALLBACK_URL"
	EnvWeatherURL                = "WEATHER_URL"
	EnvWeatherFallbackURL        = "WEATHER_FALLBACK_URL"
	EnvSatelliteAggregationDays  = "SATELLITE_AGGREGATION_DAYS"
	EnvSatelliteMinValidFraction = "SATELLITE_MIN_VALID_FRACTION"
	EnvDBURL                     = "DATABASE_URL"
	EnvDBTimeout                 = "DB_TIMEOUT"
	EnvAnalysisQueueSize         = "ANALYSIS_QUEUE_SIZE"
)

const (
	DefaultCDSEStatisticsURL   = "https://sh.dataspace.copernicus.eu/api/v1/statistics"
	DefaultCDSETokenURL        = "https://identity.dataspace.copernicus.eu/auth/realms/CDSE/protocol/openid-connect/token"
	DefaultOverpassURL         = "https://overpass-api.de/api/interpreter"
	DefaultOverpassFallbackURL = "https://overpass.kumi.systems/api/interpreter"
	DefaultWeatherURL          = "https://archive-api.open-meteo.com/v1/archive"
	DefaultWeatherFallbackURL  = "https://archive-api.open-meteo.com/v1/era5"

	defaultHTTPAddr          = ":8080"
	defaultPublicDir         = "/app/public"
	defaultLogLevel          = "info"
	defaultMLBaseURL         = "http://127.0.0.1:8000"
	defaultDBTimeout         = 5 * time.Second
	defaultAnalysisQueueSize = 8
	defaultAggregationDays   = 5
	defaultMinValidFraction  = 0.5
)

type Config struct {
	HTTP     HTTPConfig
	LogLevel string
	ML       MLConfig
	Source   SourceConfig
	Postgres PostgresConfig
	Analysis AnalysisConfig
}

type HTTPConfig struct {
	Addr      string
	PublicDir string
}

type MLConfig struct {
	BaseURL              string
	ExpectedModelVersion string
}

// SourceConfig содержит настройки провайдеров после разбора окружения.
// Секреты хранятся только в памяти и никогда не должны попадать в логи.
type SourceConfig struct {
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
}

type PostgresConfig struct {
	URL     string
	Timeout time.Duration
}

type AnalysisConfig struct {
	QueueSize int
}

type Lookup func(string) (string, bool)

func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

func LoadFrom(lookup Lookup) (cfg Config, err error) {
	if lookup == nil {
		return cfg, fmt.Errorf("config lookup is nil")
	}
	c := Config{
		HTTP: HTTPConfig{
			Addr:      value(lookup, EnvHTTPAddr, defaultHTTPAddr),
			PublicDir: value(lookup, EnvPublicDir, defaultPublicDir),
		},
		LogLevel: value(lookup, EnvLogLevel, defaultLogLevel),
		ML: MLConfig{
			BaseURL:              value(lookup, EnvMLBaseURL, defaultMLBaseURL),
			ExpectedModelVersion: value(lookup, EnvMLModelVersion, ""),
		},
		Source: SourceConfig{
			CDSEClientID:        value(lookup, EnvCDSEClientID, ""),
			CDSEClientSecret:    value(lookup, EnvCDSEClientSecret, ""),
			CDSEStatisticsURL:   value(lookup, EnvCDSEStatisticsURL, DefaultCDSEStatisticsURL),
			CDSETokenURL:        value(lookup, EnvCDSETokenURL, DefaultCDSETokenURL),
			OverpassURL:         value(lookup, EnvOverpassURL, DefaultOverpassURL),
			OverpassFallbackURL: value(lookup, EnvOverpassFallbackURL, DefaultOverpassFallbackURL),
			WeatherURL:          value(lookup, EnvWeatherURL, DefaultWeatherURL),
			WeatherFallbackURL:  value(lookup, EnvWeatherFallbackURL, DefaultWeatherFallbackURL),
		},
		Postgres: PostgresConfig{URL: value(lookup, EnvDBURL, ""), Timeout: defaultDBTimeout},
		Analysis: AnalysisConfig{QueueSize: defaultAnalysisQueueSize},
	}

	if c.Source.AggregationDays, err = integer(lookup, EnvSatelliteAggregationDays, defaultAggregationDays); err != nil {
		return cfg, err
	}
	if c.Source.MinValidFraction, err = fractional(lookup, EnvSatelliteMinValidFraction, defaultMinValidFraction); err != nil {
		return cfg, err
	}
	if c.Analysis.QueueSize, err = integer(lookup, EnvAnalysisQueueSize, defaultAnalysisQueueSize); err != nil {
		return cfg, err
	}
	if c.Postgres.URL == "" {
		return cfg, fmt.Errorf("%s is required", EnvDBURL)
	}
	if raw, ok := lookup(EnvDBTimeout); ok && strings.TrimSpace(raw) != "" {
		c.Postgres.Timeout, err = time.ParseDuration(raw)
		if err != nil || c.Postgres.Timeout <= 0 {
			return cfg, fmt.Errorf("%s must be a positive duration", EnvDBTimeout)
		}
	}
	if c.Source.AggregationDays <= 0 || c.Source.MinValidFraction <= 0 || c.Source.MinValidFraction > 1 {
		return cfg, fmt.Errorf("invalid satellite source limits")
	}
	if c.Analysis.QueueSize <= 0 {
		return cfg, fmt.Errorf("%s must be positive", EnvAnalysisQueueSize)
	}
	cfg = c
	return cfg, nil
}

func value(lookup Lookup, key, fallback string) string {
	if raw, ok := lookup(key); ok && strings.TrimSpace(raw) != "" {
		return raw
	}
	return fallback
}

func integer(lookup Lookup, key string, fallback int) (int, error) {
	raw, ok := lookup(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return n, nil
}

func fractional(lookup Lookup, key string, fallback float64) (float64, error) {
	raw, ok := lookup(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	return n, nil
}
