package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/consumer"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/repository"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/service"
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

	ledgerRepository := repository.NewPostgresLedgerRepository(pool)
	ledgerService := service.NewLedgerService(ledgerRepository)
	reader, err := consumer.NewKafkaReader(cfg.KafkaBrokers, cfg.PaymentEventsTopic, cfg.ConsumerGroupID, cfg.KafkaSecurity)
	if err != nil {
		logger.Error("Kafka client configuration failed", "error", err)
		os.Exit(1)
	}
	paymentConsumer := consumer.NewPaymentConsumer(reader, ledgerService, logger)

	logger.Info(
		"ledger payment consumer started",
		"topic", cfg.PaymentEventsTopic,
		"broker_count", len(cfg.KafkaBrokers),
		"group_id", cfg.ConsumerGroupID,
		"tls_enabled", cfg.KafkaSecurity.TLSEnabled,
		"sasl_mechanism", cfg.KafkaSecurity.SASLMechanism,
	)
	if err := paymentConsumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("ledger payment consumer stopped", "error", err)
		os.Exit(1)
	}
}
