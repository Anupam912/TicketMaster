package kafka

import (
	"crypto/tls"
	"strings"
	"time"

	"event-ticketing-system/internal/config"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// Client provides the same authentication to metadata calls, readers, consumer
// groups and writers. Each owner closes its readers/writers before Close.
type Client struct {
	Dialer    *kafkago.Dialer
	Transport *kafkago.Transport
}

func NewClient(cfg config.KafkaConfig) (*Client, error) {
	if err := cfg.ValidateSecurity(); err != nil {
		return nil, err
	}
	protocol := cfg.KafkaSecurityProtocol()
	var mechanism sasl.Mechanism
	if strings.HasPrefix(protocol, "SASL_") {
		mechanism = plain.Mechanism{Username: cfg.SASLUsername, Password: cfg.SASLPassword}
	}
	var tlsConfig *tls.Config
	if protocol == "SSL" || protocol == "SASL_SSL" {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &Client{
		Dialer: &kafkago.Dialer{
			ClientID: cfg.ClientID, Timeout: 10 * time.Second, DualStack: true,
			SASLMechanism: mechanism, TLS: tlsConfig,
		},
		Transport: &kafkago.Transport{
			ClientID: cfg.ClientID, DialTimeout: 10 * time.Second,
			SASL: mechanism, TLS: tlsConfig,
		},
	}, nil
}

func (c *Client) Close() {
	if c != nil && c.Transport != nil {
		c.Transport.CloseIdleConnections()
	}
}
