package kafkaclient

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/config"
)

func TestBuildSecurityPlaintext(t *testing.T) {
	tlsConfig, mechanism, err := BuildSecurity(config.KafkaSecurityConfig{SASLMechanism: "none"})
	if err != nil {
		t.Fatalf("BuildSecurity() error = %v", err)
	}
	if tlsConfig != nil || mechanism != nil {
		t.Fatalf("BuildSecurity() = (%v, %v), want plaintext nil settings", tlsConfig, mechanism)
	}
}

func TestBuildSecurityTLSAndSASLPlain(t *testing.T) {
	tlsConfig, mechanism, err := BuildSecurity(config.KafkaSecurityConfig{
		TLSEnabled:    true,
		TLSServerName: "kafka.example.com",
		SASLMechanism: "plain",
		SASLUsername:  "finflow",
		SASLPassword:  "secret",
	})
	if err != nil {
		t.Fatalf("BuildSecurity() error = %v", err)
	}
	if tlsConfig == nil || tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLS minimum version = %v, want TLS 1.2", tlsConfig)
	}
	if tlsConfig.ServerName != "kafka.example.com" {
		t.Fatalf("TLS server name = %q", tlsConfig.ServerName)
	}
	if mechanism == nil || mechanism.Name() != "PLAIN" {
		t.Fatalf("SASL mechanism = %v, want PLAIN", mechanism)
	}
}

func TestBuildSecuritySCRAM(t *testing.T) {
	_, mechanism, err := BuildSecurity(config.KafkaSecurityConfig{
		SASLMechanism: "scram-sha-512",
		SASLUsername:  "finflow",
		SASLPassword:  "secret",
	})
	if err != nil {
		t.Fatalf("BuildSecurity() error = %v", err)
	}
	if mechanism == nil || mechanism.Name() != "SCRAM-SHA-512" {
		t.Fatalf("SASL mechanism = %v, want SCRAM-SHA-512", mechanism)
	}
}

func TestBuildSecurityRejectsInvalidCA(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(caFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write CA fixture: %v", err)
	}

	_, _, err := BuildSecurity(config.KafkaSecurityConfig{
		TLSEnabled:    true,
		TLSCAFile:     caFile,
		SASLMechanism: "none",
	})
	if err == nil {
		t.Fatal("BuildSecurity() error = nil, want invalid CA error")
	}
}
