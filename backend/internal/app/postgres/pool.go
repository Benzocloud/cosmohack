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

// Pool — настроенный пул PostgreSQL.
type Pool struct {
	db *sqlx.DB
}

// Open создаёт пул и проверяет соединение до возврата вызывающему коду.
func Open(ctx context.Context, cfg config.PostgresConfig) (*Pool, error) {
	if ctx == nil {
		return nil, errors.New("postgres context is nil")
	}
	if cfg.URL == "" {
		return nil, errors.New("postgres database URL is empty")
	}

	db, err := sqlx.Open(driverName, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)

	pingCtx := ctx
	cancel := func() {}
	if cfg.Timeout > 0 {
		pingCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
	}
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Pool{db: db}, nil
}

// DB возвращает sqlx-пул для repository constructors.
func (p *Pool) DB() *sqlx.DB {
	if p == nil {
		return nil
	}
	return p.db
}

// Close закрывает пул. Повторный вызов безопасен.
func (p *Pool) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	if err := p.db.Close(); err != nil {
		return fmt.Errorf("close postgres pool: %w", err)
	}
	p.db = nil
	return nil
}
