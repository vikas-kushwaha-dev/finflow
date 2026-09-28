package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv              string
	HTTPAddr            string
	MetricsAddr         string
	DatabaseURL         string
	InternalToken       string
	KafkaBrokers        []string
	PaymentEventsTopic  string
	ConsumerGroupID     string
	DeadLetterTopic     string
	ConsumerMaxAttempts int
	RetryInitialBackoff time.Duration
	RetryMaxBackoff     time.Duration
	KafkaSecurity       KafkaSecurityConfig
}

var ErrInvalidConfig = errors.New("invalid config")

func Load() (Config, error) {
	kafkaSecurity, err := loadKafkaSecurity()
	if err != nil {
		return Config{}, err
	}
	maxAttempts, err := parseIntEnv("LEDGER_CONSUMER_MAX_ATTEMPTS", 5)
	if err != nil {
		return Config{}, err
	}
	initialBackoff, err := parseDurationEnv("LEDGER_RETRY_INITIAL_BACKOFF", 250*time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	maxBackoff, err := parseDurationEnv("LEDGER_RETRY_MAX_BACKOFF", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:              getEnv("APP_ENV", "local"),
		HTTPAddr:            getEnv("LEDGER_HTTP_ADDR", ":8081"),
		MetricsAddr:         getEnv("METRICS_ADDR", ":9090"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable"),
		InternalToken:       getEnv("INTERNAL_SERVICE_TOKEN", "local-internal-service-token-change-me"),
		KafkaBrokers:        parseCSVEnv("KAFKA_BROKERS", "localhost:9092"),
		PaymentEventsTopic:  getEnv("PAYMENT_EVENTS_TOPIC", "finflow.payment.events"),
		ConsumerGroupID:     getEnv("LEDGER_CONSUMER_GROUP_ID", "ledger-service"),
		DeadLetterTopic:     getEnv("LEDGER_DEAD_LETTER_TOPIC", "finflow.payment.events.dead-letter"),
		ConsumerMaxAttempts: maxAttempts,
		RetryInitialBackoff: initialBackoff,
		RetryMaxBackoff:     maxBackoff,
		KafkaSecurity:       kafkaSecurity,
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
	if strings.TrimSpace(c.MetricsAddr) == "" {
		problems = append(problems, "METRICS_ADDR is required")
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
	if strings.TrimSpace(c.DeadLetterTopic) == "" {
		problems = append(problems, "LEDGER_DEAD_LETTER_TOPIC is required")
	} else if c.DeadLetterTopic == c.PaymentEventsTopic {
		problems = append(problems, "LEDGER_DEAD_LETTER_TOPIC must differ from PAYMENT_EVENTS_TOPIC")
	}
	if c.ConsumerMaxAttempts < 1 {
		problems = append(problems, "LEDGER_CONSUMER_MAX_ATTEMPTS must be at least 1")
	}
	if c.RetryInitialBackoff <= 0 {
		problems = append(problems, "LEDGER_RETRY_INITIAL_BACKOFF must be greater than zero")
	}
	if c.RetryMaxBackoff < c.RetryInitialBackoff {
		problems = append(problems, "LEDGER_RETRY_MAX_BACKOFF must be at least LEDGER_RETRY_INITIAL_BACKOFF")
	}
	problems = append(problems, c.KafkaSecurity.validationProblems()...)

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

func parseIntEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid config: %w: %s must be an integer", ErrInvalidConfig, key)
	}
	return parsed, nil
}

func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid config: %w: %s must be a duration", ErrInvalidConfig, key)
	}
	return parsed, nil
}
