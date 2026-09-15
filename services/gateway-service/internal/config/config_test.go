package config

import (
	"testing"
	"time"
)

func TestValidateAcceptsValidConfig(t *testing.T) {
	cfg := Config{
		AppEnv:            "test",
		HTTPAddr:          ":8088",
		APIKey:            "secret",
		InternalToken:     "internal-secret",
		MaxBodyBytes:      1024,
		PaymentServiceURL: "http://payment-service:8080",
		LedgerServiceURL:  "http://ledger-service:8081",
		RateLimitRequests: 10,
		RateLimitWindow:   time.Minute,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestValidateRejectsInvalidTargetURL(t *testing.T) {
	cfg := Config{
		AppEnv:            "test",
		HTTPAddr:          ":8088",
		APIKey:            "secret",
		InternalToken:     "internal-secret",
		MaxBodyBytes:      1024,
		PaymentServiceURL: "payment-service:8080",
		LedgerServiceURL:  "http://ledger-service:8081",
		RateLimitRequests: 10,
		RateLimitWindow:   time.Minute,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid config error")
	}
}
