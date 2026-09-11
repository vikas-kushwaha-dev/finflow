package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/publisher"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/repository"
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

	store := repository.NewPostgresPaymentRepository(pool)
	writer := publisher.NewKafkaWriter(cfg.KafkaBrokers, cfg.PaymentEventsTopic)
	outboxPublisher := publisher.NewOutboxPublisher(store, writer, cfg.PaymentEventsTopic, logger)

	logger.Info("outbox publisher started", "topic", cfg.PaymentEventsTopic, "brokers", cfg.KafkaBrokers)
	if err := outboxPublisher.Run(ctx, 2*time.Second, 25); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("outbox publisher stopped", "error", err)
		os.Exit(1)
	}
}
