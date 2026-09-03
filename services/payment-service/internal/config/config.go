package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv        string
	HTTPAddr      string
	DatabaseURL   string
	MigrationsDir string
}

func Load() Config {
	if err := godotenv.Load("../../.env", ".env"); err != nil {
		log.Printf("env file not loaded, using environment variables: %v", err)
	}

	return Config{
		AppEnv:        getEnv("APP_ENV", "local"),
		HTTPAddr:      getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable"),
		MigrationsDir: getEnv("MIGRATIONS_DIR", "../../migrations"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
