package config

import (
	"errors"
	"testing"
)

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	cfg := Config{
		AppEnv:             "test",
		HTTPAddr:           ":8080",
		DatabaseURL:        "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
		MigrationsDir:      "../../migrations",
		InternalToken:      "internal-secret",
		MaxBodyBytes:       1024,
		KafkaBrokers:       []string{"localhost:9092"},
		PaymentEventsTopic: "finflow.payment.events",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsMissingValues(t *testing.T) {
	cfg := Config{}

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
	}
}

func TestConfigValidateRejectsInvalidDatabaseURL(t *testing.T) {
	cfg := Config{
		AppEnv:             "test",
		HTTPAddr:           ":8080",
		DatabaseURL:        "not-a-url",
		MigrationsDir:      "../../migrations",
		InternalToken:      "internal-secret",
		MaxBodyBytes:       1024,
		KafkaBrokers:       []string{"localhost:9092"},
		PaymentEventsTopic: "finflow.payment.events",
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
	}
}

func TestConfigValidateRejectsMissingInternalToken(t *testing.T) {
	cfg := Config{
		AppEnv:             "test",
		HTTPAddr:           ":8080",
		DatabaseURL:        "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
		MigrationsDir:      "../../migrations",
		MaxBodyBytes:       1024,
		KafkaBrokers:       []string{"localhost:9092"},
		PaymentEventsTopic: "finflow.payment.events",
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
	}
}

func TestConfigValidateRejectsInvalidBodyLimit(t *testing.T) {
	cfg := Config{
		AppEnv:             "test",
		HTTPAddr:           ":8080",
		DatabaseURL:        "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
		MigrationsDir:      "../../migrations",
		InternalToken:      "internal-secret",
		MaxBodyBytes:       0,
		KafkaBrokers:       []string{"localhost:9092"},
		PaymentEventsTopic: "finflow.payment.events",
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
	}
}

func TestConfigValidateRejectsMissingKafkaConfig(t *testing.T) {
	cfg := Config{
		AppEnv:        "test",
		HTTPAddr:      ":8080",
		DatabaseURL:   "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
		MigrationsDir: "../../migrations",
		InternalToken: "internal-secret",
		MaxBodyBytes:  1024,
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
	}
}
