// Package app — composition root и жизненный цикл Go-сервера: конфигурация
// из окружения, PostgreSQL repository, исполнитель анализа, маршруты, старт
// и аккуратная остановка по отмене контекста.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	appPostgres "github.com/Benzocloud/cosmohack/backend/internal/app/postgres"
	"github.com/Benzocloud/cosmohack/backend/internal/config"
	domainsource "github.com/Benzocloud/cosmohack/backend/internal/domain/source"
	"github.com/Benzocloud/cosmohack/backend/internal/handler"
	"github.com/Benzocloud/cosmohack/backend/internal/repository"
	"github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
	"github.com/Benzocloud/cosmohack/backend/internal/service/area"
	mlservice "github.com/Benzocloud/cosmohack/backend/internal/service/ml"
	"github.com/Benzocloud/cosmohack/backend/internal/service/source/factory"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Run поднимает HTTP-сервер и блокируется до ошибки запуска или отмены ctx.
// Некорректная конфигурация ML и недоступная PostgreSQL возвращают ошибку до
// открытия слушателя.
func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	mlCfg := mlservice.DefaultConfig(cfg.ML.BaseURL)
	mlCfg.ExpectedModelVersion = cfg.ML.ExpectedModelVersion
	client, err := mlservice.New(mlCfg)
	if err != nil {
		return fmt.Errorf("build ml client: %w", err)
	}

	db, err := appPostgres.Open(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	storage, err := repository.New(db)
	if err != nil {
		return fmt.Errorf("build repository: %w", err)
	}

	limits := domainsource.DefaultLimits()

	sources, err := factory.New(factory.SettingsFromConfig(cfg.Source, limits, time.Now))
	if err != nil {
		return fmt.Errorf("build source providers: %w", err)
	}

	// Один воркер-исполнитель: сбор → ML → PostgreSQL.
	executor := analysis.New(storage, newB1Collector(sources.Collector()), client, cfg.Analysis.QueueSize)
	if err := executor.Start(ctx); err != nil {
		return err
	}

	queue := &executorQueue{executor}
	mux := handler.NewMux(area.New(storage), analysis.NewQueryService(storage), analysis.NewScheduler(storage, queue), b1ContourFinder{finder: sources.ContourFinder()}, queue, handler.Limits{
		AreaHaMax:     sources.Limits().MaxAreaHectares(),
		VerticesMax:   sources.Limits().MaxPolygonVertices(),
		PeriodDaysMax: sources.Limits().MaxPeriodDays(),
	})
	handler.Register(mux, db.PingContext)
	serveStaticAt(mux, cfg.HTTP.PublicDir)

	return serve(ctx, mux, cfg.HTTP.Addr)
}

func serve(ctx context.Context, mux *http.ServeMux, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()

	slog.Info("server started", "addr", addr)

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return err
	}

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	slog.Info("server stopped gracefully")

	return nil
}

func serveStaticAt(mux *http.ServeMux, publicDir string) {
	fsys := os.DirFS(publicDir)
	if _, err := fs.Stat(fsys, "."); err != nil {
		slog.Info("static public dir not found, serving api only", "dir", publicDir)
		return
	}

	mux.Handle("GET /", http.FileServerFS(fsys))
}
