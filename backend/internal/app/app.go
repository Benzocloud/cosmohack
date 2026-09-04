// Package app — composition root и жизненный цикл Go-сервера: конфигурация
// из окружения, маршруты, старт и аккуратная остановка по отмене контекста.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Benzocloud/cosmohack/backend/internal/handler"
	mlservice "github.com/Benzocloud/cosmohack/backend/internal/service/ml"
)

const (
	// addrEnv — переменная окружения с адресом слушателя.
	addrEnv = "HTTP_ADDR"
	// defaultAddr — локальный адрес по умолчанию; Compose задаёт свой.
	defaultAddr = ":8080"
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
	if _, err := mlservice.ConfigFromEnv(); err != nil {
		return err
	}
	addr := os.Getenv(addrEnv)
	if addr == "" {
		addr = defaultAddr
	}

	mux := http.NewServeMux()
	handler.Register(mux)

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
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
