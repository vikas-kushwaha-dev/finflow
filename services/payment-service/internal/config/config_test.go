package config

import (
	"errors"
	"testing"
)

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	cfg := Config{
		AppEnv:        "test",
		HTTPAddr:      ":8080",
		DatabaseURL:   "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
		MigrationsDir: "../../migrations",
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
		AppEnv:        "test",
		HTTPAddr:      ":8080",
		DatabaseURL:   "not-a-url",
		MigrationsDir: "../../migrations",
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
	}
}
