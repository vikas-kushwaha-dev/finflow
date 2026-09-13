package config

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var ErrInvalidConfig = errors.New("invalid config")

type Config struct {
	AppEnv            string
	HTTPAddr          string
	APIKey            string
	MaxBodyBytes      int64
	PaymentServiceURL string
	LedgerServiceURL  string
	RateLimitRequests int
	RateLimitWindow   time.Duration
}

func Load() (Config, error) {
	if err := godotenv.Load("../../.env", ".env"); err != nil {
		log.Printf("env file not loaded, using environment variables: %v", err)
	}

	maxBodyBytes, err := parseInt64Env("MAX_REQUEST_BODY_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}

	rateLimitRequests, err := parseIntEnv("GATEWAY_RATE_LIMIT_REQUESTS", 120)
	if err != nil {
		return Config{}, err
	}

	rateLimitWindow, err := parseDurationEnv("GATEWAY_RATE_LIMIT_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:            getEnv("APP_ENV", "local"),
		HTTPAddr:          getEnv("GATEWAY_HTTP_ADDR", ":8088"),
		APIKey:            getEnv("API_KEY", "local-dev-api-key-change-me"),
		MaxBodyBytes:      maxBodyBytes,
		PaymentServiceURL: getEnv("PAYMENT_SERVICE_URL", "http://localhost:8080"),
		LedgerServiceURL:  getEnv("LEDGER_SERVICE_URL", "http://localhost:8081"),
		RateLimitRequests: rateLimitRequests,
		RateLimitWindow:   rateLimitWindow,
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
		problems = append(problems, "GATEWAY_HTTP_ADDR is required")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		problems = append(problems, "API_KEY is required")
	}
	if c.MaxBodyBytes <= 0 {
		problems = append(problems, "MAX_REQUEST_BODY_BYTES must be greater than zero")
	}
	if c.RateLimitRequests <= 0 {
		problems = append(problems, "GATEWAY_RATE_LIMIT_REQUESTS must be greater than zero")
	}
	if c.RateLimitWindow <= 0 {
		problems = append(problems, "GATEWAY_RATE_LIMIT_WINDOW must be greater than zero")
	}
	if !validHTTPURL(c.PaymentServiceURL) {
		problems = append(problems, "PAYMENT_SERVICE_URL must be a valid http URL")
	}
	if !validHTTPURL(c.LedgerServiceURL) {
		problems = append(problems, "LEDGER_SERVICE_URL must be a valid http URL")
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

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}

	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
