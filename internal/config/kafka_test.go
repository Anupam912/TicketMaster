package config

import (
	"strings"
	"testing"
)

func TestKafkaSecurityValidation(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, mechanism, username, password string
		wantError                                     bool
	}{
		{name: "local plaintext"},
		{name: "TLS", protocol: "SSL"},
		{name: "implicit SASL", username: "alice", password: "secret"},
		{name: "explicit SASL", protocol: "sasl_plaintext", mechanism: "plain", username: "alice", password: "secret"},
		{name: "SASL over TLS", protocol: "SASL_SSL", username: "alice", password: "secret"},
		{name: "missing password", username: "alice", wantError: true},
		{name: "missing username", password: "secret", wantError: true},
		{name: "SASL without credentials", protocol: "SASL_PLAINTEXT", wantError: true},
		{name: "credentials with plaintext", protocol: "PLAINTEXT", username: "alice", password: "secret", wantError: true},
		{name: "unsupported mechanism", protocol: "SASL_SSL", mechanism: "GSSAPI", username: "alice", password: "secret", wantError: true},
		{name: "unknown protocol", protocol: "SASL", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := KafkaConfig{Brokers: []string{"localhost:9092"}, SecurityProtocol: tc.protocol,
				SASLMechanism: tc.mechanism, SASLUsername: tc.username, SASLPassword: tc.password}
			err := cfg.ValidateSecurity()
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected validation result: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatal("error disclosed password")
			}
		})
	}
}

func TestLoadKafkaCredentials(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "broker:9092")
	t.Setenv("KAFKA_SECURITY_PROTOCOL", "SASL_PLAINTEXT")
	t.Setenv("KAFKA_SASL_MECHANISM", "PLAIN")
	t.Setenv("KAFKA_SASL_USERNAME", "alice")
	t.Setenv("KAFKA_SASL_PASSWORD", " password with spaces ")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Kafka.SASLUsername != "alice" || cfg.Kafka.SASLPassword != " password with spaces " {
		t.Fatal("credentials were not loaded exactly")
	}
	t.Setenv("KAFKA_SASL_PASSWORD", "")
	if _, err := Load(); err == nil {
		t.Fatal("incomplete credentials must fail startup configuration")
	}
}
