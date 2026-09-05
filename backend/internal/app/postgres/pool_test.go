package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/config"
)

func TestOpenRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		cfg  config.PostgresConfig
	}{
		{name: "nil context", cfg: config.PostgresConfig{URL: "postgres://localhost/db", Timeout: time.Second}},
		{name: "empty url", ctx: context.Background(), cfg: config.PostgresConfig{Timeout: time.Second}},
		{name: "non-positive timeout", ctx: context.Background(), cfg: config.PostgresConfig{URL: "postgres://localhost/db"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Open(tt.ctx, tt.cfg); err == nil {
				t.Fatal("Open must reject invalid configuration")
			}
		})
	}
}
