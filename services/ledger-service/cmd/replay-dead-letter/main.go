package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/consumer"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/replay"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("dead-letter replay failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	partition, err := requiredInt("REPLAY_PARTITION")
	if err != nil {
		return err
	}
	offset, err := requiredInt64("REPLAY_OFFSET")
	if err != nil {
		return err
	}
	eventID := strings.TrimSpace(os.Getenv("REPLAY_EVENT_ID"))
	operator := strings.TrimSpace(os.Getenv("REPLAY_OPERATOR"))
	reason := strings.TrimSpace(os.Getenv("REPLAY_REASON"))
	if eventID == "" || operator == "" || reason == "" {
		return fmt.Errorf("REPLAY_EVENT_ID, REPLAY_OPERATOR, and REPLAY_REASON are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	reader, err := consumer.NewPartitionReader(cfg.KafkaBrokers, cfg.DeadLetterTopic, partition, offset, "finflow-dead-letter-replay", cfg.KafkaSecurity)
	if err != nil {
		return fmt.Errorf("configure Kafka replay reader: %w", err)
	}
	defer reader.Close()
	message, err := reader.FetchMessage(ctx)
	if err != nil {
		return fmt.Errorf("fetch dead-letter record: %w", err)
	}
	if message.Offset != offset {
		return fmt.Errorf("dead-letter offset mismatch: received %d, expected %d", message.Offset, offset)
	}
	var record consumer.DeadLetterRecord
	if err := json.Unmarshal(message.Value, &record); err != nil {
		return fmt.Errorf("decode dead-letter record: %w", err)
	}
	if record.EventID != eventID {
		return fmt.Errorf("dead-letter event mismatch: record event_id %q does not match requested event_id", record.EventID)
	}

	writer, err := consumer.NewKafkaWriter(cfg.KafkaBrokers, cfg.PaymentEventsTopic, "finflow-dead-letter-replay", cfg.KafkaSecurity)
	if err != nil {
		return fmt.Errorf("configure Kafka replay writer: %w", err)
	}
	defer writer.Close()
	replayer := replay.New(repository.NewReplayAuditRepository(pool), writer, cfg.PaymentEventsTopic)
	if err := replayer.Replay(ctx, record, operator, reason); err != nil {
		return err
	}
	logger.Info("dead-letter replay completed", "event_id", eventID, "partition", partition, "offset", offset, "operator", operator)
	return nil
}

func requiredInt(key string) (int, error) {
	value, err := requiredInt64(key)
	return int(value), err
}

func requiredInt64(key string) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return parsed, nil
}
