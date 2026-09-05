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
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := app.Run(ctx)

	stop()

	if err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
