package config

import (
	"fmt"
	"strings"
)

// KafkaSecurityProtocol preserves unauthenticated local brokers while enabling
// SASL automatically when credentials are supplied without an explicit protocol.
func (k KafkaConfig) KafkaSecurityProtocol() string {
	if protocol := strings.ToUpper(strings.TrimSpace(k.SecurityProtocol)); protocol != "" {
		return protocol
	}
	if k.SASLUsername != "" || k.SASLPassword != "" {
		return "SASL_PLAINTEXT"
	}
	return "PLAINTEXT"
}

func (k KafkaConfig) ValidateSecurity() error {
	if len(k.Brokers) == 0 {
		return nil
	}
	protocol := k.KafkaSecurityProtocol()
	switch protocol {
	case "PLAINTEXT", "SSL":
		if k.SASLUsername != "" || k.SASLPassword != "" {
			return fmt.Errorf("Kafka credentials require KAFKA_SECURITY_PROTOCOL=SASL_PLAINTEXT or SASL_SSL")
		}
	case "SASL_PLAINTEXT", "SASL_SSL":
		if k.SASLUsername == "" || k.SASLPassword == "" {
			return fmt.Errorf("Kafka SASL requires both KAFKA_SASL_USERNAME and KAFKA_SASL_PASSWORD")
		}
		if mechanism := strings.ToUpper(strings.TrimSpace(k.SASLMechanism)); mechanism != "" && mechanism != "PLAIN" {
			return fmt.Errorf("KAFKA_SASL_MECHANISM must be PLAIN")
		}
	default:
		return fmt.Errorf("KAFKA_SECURITY_PROTOCOL must be PLAINTEXT, SSL, SASL_PLAINTEXT, or SASL_SSL")
	}
	return nil
}
