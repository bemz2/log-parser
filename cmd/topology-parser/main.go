package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"topology-parser/internal"
	"topology-parser/internal/application"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := internal.NewConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	container := application.NewContainer(cfg, logger)
	if err := container.Init(ctx); err != nil {
		logger.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	app := application.NewApp(container)
	if err := app.Run(ctx); err != nil {
		logger.Error("failed to run application", "error", err)
		_ = app.Shutdown(context.Background())
		os.Exit(1)
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("failed to shutdown application", "error", err)
		os.Exit(1)
	}
}
