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
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/observability"
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
	shutdownTracing, err := observability.InitTracing(ctx, "ledger-consumer", logger)
	if err != nil {
		logger.Error("trace configuration failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	metrics := observability.NewMetrics()
	go func() {
		if err := observability.RunMetricsServer(ctx, cfg.MetricsAddr, metrics, logger); err != nil {
			logger.Error("worker telemetry failed", "error", err)
			stop()
		}
	}()

	pool, err := database.Connect(ctx, cfg.DatabaseURL, metrics)
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
	deadLetterWriter, err := consumer.NewKafkaWriter(cfg.KafkaBrokers, cfg.DeadLetterTopic, "finflow-ledger-dead-letter", cfg.KafkaSecurity)
	if err != nil {
		logger.Error("Kafka dead-letter client configuration failed", "error", err)
		os.Exit(1)
	}
	deadLetters := consumer.NewDeadLetterPublisher(deadLetterWriter, cfg.DeadLetterTopic)
	paymentConsumer := consumer.NewPaymentConsumer(reader, ledgerService, deadLetters, cfg.ConsumerMaxAttempts, cfg.RetryInitialBackoff, cfg.RetryMaxBackoff, logger, metrics)

	logger.Info(
		"ledger payment consumer started",
		"topic", cfg.PaymentEventsTopic,
		"broker_count", len(cfg.KafkaBrokers),
		"group_id", cfg.ConsumerGroupID,
		"dead_letter_topic", cfg.DeadLetterTopic,
		"max_attempts", cfg.ConsumerMaxAttempts,
		"tls_enabled", cfg.KafkaSecurity.TLSEnabled,
		"sasl_mechanism", cfg.KafkaSecurity.SASLMechanism,
	)
	if err := paymentConsumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("ledger payment consumer stopped", "error", err)
		os.Exit(1)
	}
}
