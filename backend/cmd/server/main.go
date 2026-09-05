// Команда сервера Go-монолита: создаёт signal context и передаёт жизненный
// цикл пакету app. Маршруты и запуск живут в internal/app и internal/handler.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Benzocloud/cosmohack/backend/internal/app"
	"github.com/Benzocloud/cosmohack/backend/internal/config"
	applog "github.com/Benzocloud/cosmohack/backend/pkg/log"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	cfg, err := config.Load()
	var logger *slog.Logger
	if err != nil {
		logger = applog.New("info", applog.DefaultTimeFormat)
	} else {
		logger = applog.New(cfg.LogLevel, applog.DefaultTimeFormat)
		err = app.Run(ctx, cfg, logger)
	}

	stop()

	if err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
