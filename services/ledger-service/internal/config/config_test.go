package config

import (
	"errors"
	"testing"
)

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	cfg := Config{
		AppEnv:             "test",
		HTTPAddr:           ":8081",
		DatabaseURL:        "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
		InternalToken:      "internal-secret",
		KafkaBrokers:       []string{"localhost:9092"},
		PaymentEventsTopic: "finflow.payment.events",
		ConsumerGroupID:    "ledger-service",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsMissingKafkaConfig(t *testing.T) {
	cfg := Config{
		AppEnv:      "test",
		DatabaseURL: "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
	}
}
