package kafka

import (
	"context"
	"fmt"
	"log"
	"time"

	"event-ticketing-system/internal/config"

	kafkago "github.com/segmentio/kafka-go"
)

const topicEnsureTimeout = 10 * time.Second

func EnsureTopics(ctx context.Context, cfg *config.Config) error {
	if cfg == nil || len(cfg.Kafka.Brokers) == 0 {
		return nil
	}

	ensureCtx, cancel := context.WithTimeout(ctx, topicEnsureTimeout)
	defer cancel()

	conn, err := kafkago.DialContext(ensureCtx, "tcp", cfg.Kafka.Brokers[0])
	if err != nil {
		return fmt.Errorf("dial kafka broker %s: %w", cfg.Kafka.Brokers[0], err)
	}
	defer conn.Close()

	existingTopics, err := readTopicSet(conn)
	if err != nil {
		return fmt.Errorf("read kafka topics: %w", err)
	}

	topicConfigs := missingTopicConfigs(cfg, existingTopics)
	if len(topicConfigs) == 0 {
		return nil
	}

	if err := conn.CreateTopics(topicConfigs...); err != nil {
		return fmt.Errorf("create kafka topics: %w", err)
	}

	for _, topic := range topicConfigs {
		log.Printf("Kafka topic ensured: %s partitions=%d replication_factor=%d", topic.Topic, topic.NumPartitions, topic.ReplicationFactor)
	}
	return nil
}

func readTopicSet(conn *kafkago.Conn) (map[string]struct{}, error) {
	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, err
	}

	topics := make(map[string]struct{}, len(partitions))
	for _, partition := range partitions {
		topics[partition.Topic] = struct{}{}
	}
	return topics, nil
}

func missingTopicConfigs(cfg *config.Config, existingTopics map[string]struct{}) []kafkago.TopicConfig {
	names := []string{
		cfg.Kafka.BookingEventsTopic,
		cfg.Kafka.BookingEventsDLQTopic,
		cfg.Kafka.BookingCommandsTopic,
		cfg.Kafka.PurchaseCommandsTopic,
		cfg.Kafka.BookingDLQTopic,
		cfg.Kafka.PurchaseDLQTopic,
	}

	partitions := cfg.Kafka.TopicPartitions
	if partitions <= 0 {
		partitions = 6
	}

	replicationFactor := cfg.Kafka.TopicReplicationFactor
	if replicationFactor <= 0 {
		replicationFactor = 1
	}

	topicConfigs := make([]kafkago.TopicConfig, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		if _, ok := existingTopics[name]; ok {
			continue
		}

		topicConfigs = append(topicConfigs, kafkago.TopicConfig{
			Topic:             name,
			NumPartitions:     partitions,
			ReplicationFactor: replicationFactor,
		})
	}

	return topicConfigs
}
