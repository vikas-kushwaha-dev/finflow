package main

import (
	"testing"
	"time"
)

func TestLoadConfigDefaultsToDryRun(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/finflow")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if !cfg.dryRun || cfg.batchSize != 1000 || cfg.outboxRetention != 30*24*time.Hour {
		t.Fatalf("config = %#v", cfg)
	}
}

func TestLoadConfigRejectsUnsafeBatchSize(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/finflow")
	t.Setenv("RETENTION_BATCH_SIZE", "10001")
	if _, err := loadConfig(); err == nil {
		t.Fatal("loadConfig() error = nil")
	}
}

func TestLoadConfigRequiresConfirmationForDeletion(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/finflow")
	t.Setenv("RETENTION_DRY_RUN", "false")
	if _, err := loadConfig(); err == nil {
		t.Fatal("loadConfig() error = nil")
	}
}

func TestLoadConfigAllowsConfirmedDeletion(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/finflow")
	t.Setenv("RETENTION_DRY_RUN", "false")
	t.Setenv("RETENTION_CONFIRM", retentionConfirmation)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.dryRun {
		t.Fatal("dryRun = true")
	}
}
