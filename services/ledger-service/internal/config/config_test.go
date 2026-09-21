package config

import (
	"errors"
	"strings"
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

func TestConfigValidateAcceptsSecureKafkaConfig(t *testing.T) {
	cfg := Config{
		AppEnv:             "production",
		HTTPAddr:           ":8081",
		DatabaseURL:        "postgres://finflow:finflow@db:5432/finflow?sslmode=require",
		InternalToken:      "internal-secret",
		KafkaBrokers:       []string{"kafka.example.com:9093"},
		PaymentEventsTopic: "finflow.payment.events",
		ConsumerGroupID:    "ledger-service",
		KafkaSecurity: KafkaSecurityConfig{
			RequireSecureTransport: true,
			TLSEnabled:             true,
			TLSCAFile:              "/etc/finflow/kafka/ca.crt",
			SASLMechanism:          "scram-sha-512",
			SASLUsername:           "finflow",
			SASLPassword:           "secret",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsInsecureKafkaCombinations(t *testing.T) {
	tests := []struct {
		name     string
		security KafkaSecurityConfig
	}{
		{
			name: "required transport without TLS",
			security: KafkaSecurityConfig{
				RequireSecureTransport: true,
				SASLMechanism:          "none",
			},
		},
		{
			name: "plain SASL without TLS",
			security: KafkaSecurityConfig{
				SASLMechanism: "plain",
				SASLUsername:  "finflow",
				SASLPassword:  "secret",
			},
		},
		{
			name: "missing SASL password",
			security: KafkaSecurityConfig{
				TLSEnabled:    true,
				SASLMechanism: "scram-sha-512",
				SASLUsername:  "finflow",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				AppEnv:             "test",
				HTTPAddr:           ":8081",
				DatabaseURL:        "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
				InternalToken:      "internal-secret",
				KafkaBrokers:       []string{"localhost:9092"},
				PaymentEventsTopic: "finflow.payment.events",
				ConsumerGroupID:    "ledger-service",
				KafkaSecurity:      tt.security,
			}

			if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Validate() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestConfigValidationDoesNotExposeKafkaPassword(t *testing.T) {
	cfg := Config{
		AppEnv:             "test",
		HTTPAddr:           ":8081",
		DatabaseURL:        "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
		InternalToken:      "internal-secret",
		KafkaBrokers:       []string{"localhost:9092"},
		PaymentEventsTopic: "finflow.payment.events",
		ConsumerGroupID:    "ledger-service",
		KafkaSecurity: KafkaSecurityConfig{
			TLSEnabled:    true,
			SASLMechanism: "unsupported",
			SASLUsername:  "finflow",
			SASLPassword:  "do-not-log-this",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid mechanism error")
	}
	if strings.Contains(err.Error(), cfg.KafkaSecurity.SASLPassword) {
		t.Fatal("Validate() exposed Kafka SASL password")
	}
}

func TestLoadRejectsInvalidKafkaTLSFlag(t *testing.T) {
	t.Setenv("KAFKA_TLS_ENABLED", "not-a-boolean")

	_, err := Load()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Load() error = %v, want ErrInvalidConfig", err)
	}
}
