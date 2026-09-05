package log

import (
	"log/slog"
	"testing"
)

func TestNewCreatesIndependentLogger(t *testing.T) {
	first := New("debug", "")
	second := New("error", "")

	if first == second {
		t.Fatal("New must create an independent logger")
	}
	if slog.Default() == first || slog.Default() == second {
		t.Fatal("New must not replace the default logger")
	}
}

func TestNewLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  slog.Level
	}{
		{name: "default", want: slog.LevelInfo},
		{name: "debug", input: "debug", want: slog.LevelDebug},
		{name: "info", input: "info", want: slog.LevelInfo},
		{name: "warn", input: "warn", want: slog.LevelWarn},
		{name: "error", input: "error", want: slog.LevelError},
		{name: "invalid", input: "unknown", want: slog.LevelInfo},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := New(test.input, "")
			if !logger.Enabled(t.Context(), test.want) {
				t.Fatalf("logger level %v is not enabled", test.want)
			}
			if test.want != slog.LevelDebug && logger.Enabled(t.Context(), test.want-1) {
				t.Fatalf("logger enabled level below %v", test.want)
			}
		})
	}
}
