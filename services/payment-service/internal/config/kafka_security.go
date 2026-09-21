package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type KafkaSecurityConfig struct {
	RequireSecureTransport bool
	TLSEnabled             bool
	TLSCAFile              string
	TLSCertFile            string
	TLSKeyFile             string
	TLSServerName          string
	SASLMechanism          string
	SASLUsername           string
	SASLPassword           string
}

func loadKafkaSecurity() (KafkaSecurityConfig, error) {
	requireSecureTransport, err := parseBoolEnv("KAFKA_REQUIRE_SECURE_TRANSPORT", false)
	if err != nil {
		return KafkaSecurityConfig{}, err
	}
	tlsEnabled, err := parseBoolEnv("KAFKA_TLS_ENABLED", false)
	if err != nil {
		return KafkaSecurityConfig{}, err
	}

	return KafkaSecurityConfig{
		RequireSecureTransport: requireSecureTransport,
		TLSEnabled:             tlsEnabled,
		TLSCAFile:              strings.TrimSpace(os.Getenv("KAFKA_TLS_CA_FILE")),
		TLSCertFile:            strings.TrimSpace(os.Getenv("KAFKA_TLS_CERT_FILE")),
		TLSKeyFile:             strings.TrimSpace(os.Getenv("KAFKA_TLS_KEY_FILE")),
		TLSServerName:          strings.TrimSpace(os.Getenv("KAFKA_TLS_SERVER_NAME")),
		SASLMechanism:          strings.ToLower(strings.TrimSpace(getEnv("KAFKA_SASL_MECHANISM", "none"))),
		SASLUsername:           strings.TrimSpace(os.Getenv("KAFKA_SASL_USERNAME")),
		SASLPassword:           os.Getenv("KAFKA_SASL_PASSWORD"),
	}, nil
}

func (c KafkaSecurityConfig) validationProblems() []string {
	var problems []string

	if c.RequireSecureTransport && !c.TLSEnabled {
		problems = append(problems, "KAFKA_TLS_ENABLED must be true when KAFKA_REQUIRE_SECURE_TRANSPORT is true")
	}
	if !c.TLSEnabled && (c.TLSCAFile != "" || c.TLSCertFile != "" || c.TLSKeyFile != "" || c.TLSServerName != "") {
		problems = append(problems, "Kafka TLS settings require KAFKA_TLS_ENABLED=true")
	}
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		problems = append(problems, "KAFKA_TLS_CERT_FILE and KAFKA_TLS_KEY_FILE must be set together")
	}

	switch c.SASLMechanism {
	case "none", "":
		if c.SASLUsername != "" || c.SASLPassword != "" {
			problems = append(problems, "Kafka SASL credentials require KAFKA_SASL_MECHANISM")
		}
	case "plain", "scram-sha-256", "scram-sha-512":
		if c.SASLUsername == "" || c.SASLPassword == "" {
			problems = append(problems, "KAFKA_SASL_USERNAME and KAFKA_SASL_PASSWORD are required when SASL is enabled")
		}
	default:
		problems = append(problems, "KAFKA_SASL_MECHANISM must be none, plain, scram-sha-256, or scram-sha-512")
	}

	if c.SASLMechanism == "plain" && !c.TLSEnabled {
		problems = append(problems, "KAFKA_TLS_ENABLED must be true when SASL PLAIN is used")
	}

	return problems
}

func parseBoolEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid config: %w: %s must be a boolean", ErrInvalidConfig, key)
	}

	return parsed, nil
}
