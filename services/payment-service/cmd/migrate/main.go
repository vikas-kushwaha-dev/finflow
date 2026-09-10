package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/migration"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration invalid", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := migration.Run(ctx, pool, cfg.MigrationsDir); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	logger.Info("migrations completed", "dir", cfg.MigrationsDir)
}
