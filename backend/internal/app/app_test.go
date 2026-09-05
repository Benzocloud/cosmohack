package app

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/config"
	applog "github.com/Benzocloud/cosmohack/backend/pkg/log"
)

func TestRunRejectsInvalidMLConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/cosmohack")
	t.Setenv("ML_BASE_URL", "not-a-url")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	err = Run(context.Background(), cfg, applog.New(slog.LevelInfo, ""))
	if err == nil || !strings.Contains(err.Error(), "ml base url scheme is not http or https") {
		t.Fatal("Run must fail fast on an invalid ML base url")
	}
}

func TestServeGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	mux := http.NewServeMux()
	go func() {
		done <- serve(ctx, mux, "127.0.0.1:0", applog.New(slog.LevelInfo, ""))
	}()
	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("graceful Run must return nil, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}
