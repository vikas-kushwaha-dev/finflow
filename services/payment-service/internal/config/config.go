package config

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv        string
	HTTPAddr      string
	DatabaseURL   string
	MigrationsDir string
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

var ErrInvalidConfig = errors.New("invalid config")
