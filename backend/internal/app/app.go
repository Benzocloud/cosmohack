// Package app — composition root и жизненный цикл Go-сервера: конфигурация
// из окружения, хранилище, исполнитель анализа, маршруты, старт и аккуратная
// остановка по отмене контекста.
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

	"github.com/Benzocloud/cosmohack/backend/internal/handler"
	"github.com/Benzocloud/cosmohack/backend/internal/service/analysis"
	mlservice "github.com/Benzocloud/cosmohack/backend/internal/service/ml"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

const (
	// addrEnv — переменная окружения с адресом слушателя.
	addrEnv = "HTTP_ADDR"
	// defaultAddr — локальный адрес по умолчанию; Compose задаёт свой.
	defaultAddr = ":8080"
	// dataDirEnv — каталог постоянного хранилища снимков Go.
	dataDirEnv = "DATA_DIR"
	// defaultDataDir — локальный каталог по умолчанию; Compose монтирует том.
	defaultDataDir = "./data"
	// publicDirEnv — каталог собранного frontend внутри образа.
	publicDirEnv = "PUBLIC_DIR"
	// defaultPublicDir — куда Dockerfile кладёт frontend/dist.
	defaultPublicDir = "/app/public"
	// readHeaderTimeout ограничивает время чтения заголовков запроса.
	readHeaderTimeout = 5 * time.Second
	// shutdownTimeout — предел ожидания завершения активных обработчиков.
	shutdownTimeout = 10 * time.Second
)

// Run поднимает HTTP-сервер и блокируется до ошибки запуска или отмены ctx.
// Некорректная конфигурация ML возвращает ошибку до открытия слушателя.
func Run(ctx context.Context) error {
	// Ранняя проверка конфигурации ML: некорректный адрес должен остановить
	// старт, а не превратиться в ошибку первого анализа.
	mlCfg, err := mlservice.ConfigFromEnv()
	if err != nil {
		return err
	}

	dataDir := os.Getenv(dataDirEnv)
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	st, err := store.Open(dataDir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	client, err := mlservice.New(mlCfg)
	if err != nil {
		return fmt.Errorf("build ml client: %w", err)
	}

	// Один воркер-исполнитель: очередь ≤8 внутри Go, сбор → ML → store.
	executor := analysis.New(st, placeholderCollector{}, client)
	if err := executor.Start(ctx); err != nil {
		return err
	}

	addr := os.Getenv(addrEnv)
	if addr == "" {
		addr = defaultAddr
	}

	mux := handler.NewMux(st, placeholderContours{}, &executorQueue{executor}, handler.Limits{})
	handler.Register(mux)
	serveStatic(mux)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()

	slog.Info("server started", "addr", addr, "data_dir", dataDir)
	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	// Shutdown gets its own deadline but keeps values from ctx after cancellation.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Shutdown не дождался обработчиков: закрываем слушатель принудительно.
		_ = srv.Close()
		return err
	}
	// Дожидаемся serve-goroutine: её ошибка после Shutdown не проглатывается.
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("server stopped gracefully")
	return nil
}

// serveStatic подключает раздачу собранного frontend паттерном "GET /":
// конкретные маршруты (/readyz, /api/*) приоритетнее. Каталог может
// отсутствовать при локальной разработке без образа — тогда раздача off.
func serveStatic(mux *http.ServeMux) {
	publicDir := os.Getenv(publicDirEnv)
	if publicDir == "" {
		publicDir = defaultPublicDir
	}
	fsys := os.DirFS(publicDir)
	if _, err := fs.Stat(fsys, "."); err != nil {
		slog.Info("static public dir not found, serving api only", "dir", publicDir)
		return
	}
	mux.Handle("GET /", http.FileServerFS(fsys))
}
