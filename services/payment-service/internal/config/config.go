package config

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv             string
	HTTPAddr           string
	DatabaseURL        string
	MigrationsDir      string
	InternalToken      string
	MaxBodyBytes       int64
	KafkaBrokers       []string
	PaymentEventsTopic string
	KafkaSecurity      KafkaSecurityConfig
}

func Load() (Config, error) {
	if err := godotenv.Load("../../.env", ".env"); err != nil {
		log.Printf("env file not loaded, using environment variables: %v", err)
	}

	kafkaSecurity, err := loadKafkaSecurity()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:             getEnv("APP_ENV", "local"),
		HTTPAddr:           getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable"),
		MigrationsDir:      getEnv("MIGRATIONS_DIR", "../../migrations"),
		InternalToken:      getEnv("INTERNAL_SERVICE_TOKEN", "local-internal-service-token-change-me"),
		KafkaBrokers:       parseCSVEnv("KAFKA_BROKERS", "localhost:9092"),
		PaymentEventsTopic: getEnv("PAYMENT_EVENTS_TOPIC", "finflow.payment.events"),
		KafkaSecurity:      kafkaSecurity,
	}

	maxBodyBytes, err := parseInt64Env("MAX_REQUEST_BODY_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxBodyBytes = maxBodyBytes

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
		problems = append(problems, "HTTP_ADDR is required")
	}

	if strings.TrimSpace(c.DatabaseURL) == "" {
		problems = append(problems, "DATABASE_URL is required")
	} else if parsed, err := url.Parse(c.DatabaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		problems = append(problems, "DATABASE_URL must be a valid database URL")
	}

	if strings.TrimSpace(c.MigrationsDir) == "" {
		problems = append(problems, "MIGRATIONS_DIR is required")
	}

	if strings.TrimSpace(c.InternalToken) == "" {
		problems = append(problems, "INTERNAL_SERVICE_TOKEN is required")
	}

	if c.MaxBodyBytes <= 0 {
		problems = append(problems, "MAX_REQUEST_BODY_BYTES must be greater than zero")
	}

	if len(c.KafkaBrokers) == 0 {
		problems = append(problems, "KAFKA_BROKERS is required")
	}

	if strings.TrimSpace(c.PaymentEventsTopic) == "" {
		problems = append(problems, "PAYMENT_EVENTS_TOPIC is required")
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

func parseInt64Env(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid config: %w: %s must be an integer", ErrInvalidConfig, key)
	}

	return parsed, nil
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

var ErrInvalidConfig = errors.New("invalid config")
