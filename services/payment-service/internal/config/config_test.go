package config

import (
	"errors"
	"strings"
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

func TestConfigValidateAcceptsSecureKafkaConfig(t *testing.T) {
	cfg := Config{
		AppEnv:             "production",
		HTTPAddr:           ":8080",
		DatabaseURL:        "postgres://finflow:finflow@db:5432/finflow?sslmode=require",
		MigrationsDir:      "/app/migrations",
		InternalToken:      "internal-secret",
		MaxBodyBytes:       1024,
		KafkaBrokers:       []string{"kafka.example.com:9093"},
		PaymentEventsTopic: "finflow.payment.events",
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
		{
			name: "partial client certificate",
			security: KafkaSecurityConfig{
				TLSEnabled:    true,
				TLSCertFile:   "/client/tls.crt",
				SASLMechanism: "none",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				AppEnv:             "test",
				HTTPAddr:           ":8080",
				DatabaseURL:        "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
				MigrationsDir:      "../../migrations",
				InternalToken:      "internal-secret",
				MaxBodyBytes:       1024,
				KafkaBrokers:       []string{"localhost:9092"},
				PaymentEventsTopic: "finflow.payment.events",
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
		HTTPAddr:           ":8080",
		DatabaseURL:        "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable",
		MigrationsDir:      "../../migrations",
		InternalToken:      "internal-secret",
		MaxBodyBytes:       1024,
		KafkaBrokers:       []string{"localhost:9092"},
		PaymentEventsTopic: "finflow.payment.events",
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
