package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	AppEnv             string
	HTTPAddr           string
	DatabaseURL        string
	InternalToken      string
	KafkaBrokers       []string
	PaymentEventsTopic string
	ConsumerGroupID    string
}

var ErrInvalidConfig = errors.New("invalid config")

func Load() (Config, error) {
	cfg := Config{
		AppEnv:             getEnv("APP_ENV", "local"),
		HTTPAddr:           getEnv("LEDGER_HTTP_ADDR", ":8081"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable"),
		InternalToken:      getEnv("INTERNAL_SERVICE_TOKEN", "local-internal-service-token-change-me"),
		KafkaBrokers:       parseCSVEnv("KAFKA_BROKERS", "localhost:9092"),
		PaymentEventsTopic: getEnv("PAYMENT_EVENTS_TOPIC", "finflow.payment.events"),
		ConsumerGroupID:    getEnv("LEDGER_CONSUMER_GROUP_ID", "ledger-service"),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	var problems []string

	if strings.TrimSpace(c.AppEnv) == "" {
		problems = append(problems, "APP_ENV is required")
	}
	if strings.TrimSpace(c.HTTPAddr) == "" {
		problems = append(problems, "LEDGER_HTTP_ADDR is required")
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		problems = append(problems, "DATABASE_URL is required")
	} else if parsed, err := url.Parse(c.DatabaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		problems = append(problems, "DATABASE_URL must be a valid database URL")
	}
	if strings.TrimSpace(c.InternalToken) == "" {
		problems = append(problems, "INTERNAL_SERVICE_TOKEN is required")
	}
	if len(c.KafkaBrokers) == 0 {
		problems = append(problems, "KAFKA_BROKERS is required")
	}
	if strings.TrimSpace(c.PaymentEventsTopic) == "" {
		problems = append(problems, "PAYMENT_EVENTS_TOPIC is required")
	}
	if strings.TrimSpace(c.ConsumerGroupID) == "" {
		problems = append(problems, "LEDGER_CONSUMER_GROUP_ID is required")
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid config: %w: %s", ErrInvalidConfig, strings.Join(problems, "; "))
	}

	return nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func parseCSVEnv(key string, fallback string) []string {
	value := getEnv(key, fallback)
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}
