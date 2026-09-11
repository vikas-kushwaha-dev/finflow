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
	AppEnv        string
	HTTPAddr      string
	DatabaseURL   string
	MigrationsDir string
	APIKey        string
	MaxBodyBytes  int64
}

func Load() (Config, error) {
	if err := godotenv.Load("../../.env", ".env"); err != nil {
		log.Printf("env file not loaded, using environment variables: %v", err)
	}

	cfg := Config{
		AppEnv:        getEnv("APP_ENV", "local"),
		HTTPAddr:      getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable"),
		MigrationsDir: getEnv("MIGRATIONS_DIR", "../../migrations"),
		APIKey:        getEnv("API_KEY", "local-dev-api-key-change-me"),
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

	if strings.TrimSpace(c.APIKey) == "" {
		problems = append(problems, "API_KEY is required")
	}

	if c.MaxBodyBytes <= 0 {
		problems = append(problems, "MAX_REQUEST_BODY_BYTES must be greater than zero")
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

var ErrInvalidConfig = errors.New("invalid config")
