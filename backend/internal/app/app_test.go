package app

import (
	"context"
	"testing"
	"time"
)

func TestRunRejectsInvalidMLConfig(t *testing.T) {
	t.Setenv("ML_BASE_URL", "not-a-url")
	if err := Run(context.Background()); err == nil {
		t.Fatal("Run must fail fast on an invalid ML base url")
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:0")
	t.Setenv("ML_BASE_URL", "http://127.0.0.1:1")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx)
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
