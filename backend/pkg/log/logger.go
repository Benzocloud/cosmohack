// Пакет log создаёт логгер процесса, используемый корнем композиции.
package log

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

const (
	// DefaultTimeFormat — формат временной метки локального логгера.
	DefaultTimeFormat = "2006-01-02 15:04:05"

	defaultLevel = slog.LevelInfo
)

// New создаёт независимый структурированный логгер. Логгер возвращается
// вызывающему коду и не устанавливается глобальным логгером slog.
func New[L ~string | slog.Level](level L, timeFormat string) *slog.Logger {
	logLevel := defaultLevel
	switch value := any(level).(type) {
	case string:
		logLevel = parseLevel(value)
	case slog.Level:
		logLevel = value
	}
	if timeFormat == "" {
		timeFormat = DefaultTimeFormat
	}

	return slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		AddSource:  logLevel == slog.LevelDebug,
		Level:      logLevel,
		TimeFormat: timeFormat,
	}))
}

func parseLevel(value string) slog.Level {
	switch value {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return defaultLevel
	}
}
