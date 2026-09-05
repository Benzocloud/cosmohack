// Package postgres содержит подключение приложения к PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Benzocloud/cosmohack/backend/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	driverName   = "pgx"
	maxOpenConns = 10
	maxIdleConns = 5
)

// Open создаёт пул и проверяет соединение до возврата вызывающему коду.
func Open(ctx context.Context, cfg config.PostgresConfig) (*sqlx.DB, error) {
	if ctx == nil {
		return nil, errors.New("postgres context is nil")
	}
	if cfg.URL == "" {
		return nil, errors.New("postgres database URL is empty")
	}
	if cfg.Timeout <= 0 {
		return nil, errors.New("postgres timeout must be positive")
	}

	db, err := sqlx.Open(driverName, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}
