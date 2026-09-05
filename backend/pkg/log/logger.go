// Package log creates the process logger used by the composition root.
package log

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

const (
	// DefaultTimeFormat is the timestamp format used by the local logger.
	DefaultTimeFormat = "2006-01-02 15:04:05"

	defaultLevel = slog.LevelInfo
)

// New creates an independent structured logger. The logger is returned to the
// caller and is never installed as the process-wide slog default.
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
