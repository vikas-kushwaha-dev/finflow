package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/retention"
)

const retentionLockID int64 = 220022
const retentionConfirmation = "DELETE_OPERATIONAL_DATA"

type retentionConfig struct {
	databaseURL       string
	dryRun            bool
	batchSize         int
	outboxRetention   time.Duration
	consumedRetention time.Duration
	replayRetention   time.Duration
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("retention run failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, err := database.Connect(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire retention connection: %w", err)
	}
	defer conn.Release()

	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", retentionLockID).Scan(&locked); err != nil {
		return fmt.Errorf("acquire retention lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("another retention run holds the advisory lock")
	}
	defer func() {
		var unlocked bool
		_ = conn.QueryRow(context.Background(), "SELECT pg_advisory_unlock($1)", retentionLockID).Scan(&unlocked)
	}()

	for _, rule := range retention.Rules(cfg.outboxRetention, cfg.consumedRetention, cfg.replayRetention) {
		result, err := retention.RunRule(ctx, conn, rule, time.Now(), cfg.batchSize, cfg.dryRun)
		if err != nil {
			return err
		}
		logger.Info("retention rule completed", "rule", result.Name, "dry_run", result.DryRun, "eligible", result.Eligible, "deleted", result.Deleted)
	}
	logger.Info("financial records retained", "tables", "payments,ledger_accounts,ledger_entries")
	return nil
}

func loadConfig() (retentionConfig, error) {
	cfg := retentionConfig{
		databaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		dryRun:      true,
		batchSize:   1000,
	}
	if cfg.databaseURL == "" {
		return retentionConfig{}, fmt.Errorf("DATABASE_URL is required")
	}
	var err error
	if value := strings.TrimSpace(os.Getenv("RETENTION_DRY_RUN")); value != "" {
		cfg.dryRun, err = strconv.ParseBool(value)
		if err != nil {
			return retentionConfig{}, fmt.Errorf("RETENTION_DRY_RUN must be a boolean")
		}
	}
	if !cfg.dryRun && strings.TrimSpace(os.Getenv("RETENTION_CONFIRM")) != retentionConfirmation {
		return retentionConfig{}, fmt.Errorf("RETENTION_CONFIRM must equal %q when RETENTION_DRY_RUN=false", retentionConfirmation)
	}
	if value := strings.TrimSpace(os.Getenv("RETENTION_BATCH_SIZE")); value != "" {
		cfg.batchSize, err = strconv.Atoi(value)
		if err != nil || cfg.batchSize < 1 || cfg.batchSize > 10000 {
			return retentionConfig{}, fmt.Errorf("RETENTION_BATCH_SIZE must be between 1 and 10000")
		}
	}
	if cfg.outboxRetention, err = durationEnv("OUTBOX_RETENTION", 30*24*time.Hour); err != nil {
		return retentionConfig{}, err
	}
	if cfg.consumedRetention, err = durationEnv("CONSUMED_EVENT_RETENTION", 90*24*time.Hour); err != nil {
		return retentionConfig{}, err
	}
	if cfg.replayRetention, err = durationEnv("REPLAY_AUDIT_RETENTION", 365*24*time.Hour); err != nil {
		return retentionConfig{}, err
	}
	return cfg, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return parsed, nil
}
