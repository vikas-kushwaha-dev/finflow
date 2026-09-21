package kafkaclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/config"
)

func BuildSecurity(cfg config.KafkaSecurityConfig) (*tls.Config, sasl.Mechanism, error) {
	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	mechanism, err := buildSASLMechanism(cfg)
	if err != nil {
		return nil, nil, err
	}

	return tlsConfig, mechanism, nil
}

func buildTLSConfig(cfg config.KafkaSecurityConfig) (*tls.Config, error) {
	if !cfg.TLSEnabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: cfg.TLSServerName,
	}

	if cfg.TLSCAFile != "" {
		caPEM, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read Kafka TLS CA file: %w", err)
		}

		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("Kafka TLS CA file contains no valid certificates")
		}
		tlsConfig.RootCAs = roots
	}

	if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load Kafka TLS client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	return tlsConfig, nil
}

func buildSASLMechanism(cfg config.KafkaSecurityConfig) (sasl.Mechanism, error) {
	switch cfg.SASLMechanism {
	case "", "none":
		return nil, nil
	case "plain":
		return plain.Mechanism{Username: cfg.SASLUsername, Password: cfg.SASLPassword}, nil
	case "scram-sha-256":
		return scram.Mechanism(scram.SHA256, cfg.SASLUsername, cfg.SASLPassword)
	case "scram-sha-512":
		return scram.Mechanism(scram.SHA512, cfg.SASLUsername, cfg.SASLPassword)
	default:
		return nil, fmt.Errorf("unsupported Kafka SASL mechanism %q", cfg.SASLMechanism)
	}
}
