package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/models"

	kafkago "github.com/segmentio/kafka-go"
)

// BookingEvent represents a domain event emitted to Kafka.
type BookingEvent struct {
	EventType   string                 `json:"event_type"`
	BookingID   string                 `json:"booking_id"`
	UserID      string                 `json:"user_id"`
	EventID     string                 `json:"event_id"`
	SeatID      string                 `json:"seat_id"`
	Status      models.BookingStatus   `json:"status"`
	TotalAmount float64                `json:"total_amount"`
	OccurredAt  time.Time              `json:"occurred_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type Producer struct {
	writer  *kafkago.Writer
	enabled bool
	topic   string
}

func NewProducer(cfg *config.Config) (*Producer, error) {
	if cfg == nil || len(cfg.Kafka.Brokers) == 0 {
		return &Producer{enabled: false}, nil
	}

	acks := kafkago.RequireOne
	switch strings.ToLower(cfg.Kafka.RequiredAcks) {
	case "all":
		acks = kafkago.RequireAll
	case "none":
		acks = kafkago.RequireNone
	}

	writer := &kafkago.Writer{
		Addr:                   kafkago.TCP(cfg.Kafka.Brokers...),
		Topic:                  cfg.Kafka.BookingEventsTopic,
		RequiredAcks:           acks,
		Async:                  cfg.Kafka.Async,
		AllowAutoTopicCreation: true,
		Balancer:               &kafkago.Hash{},
	}

	return &Producer{
		writer:  writer,
		enabled: true,
		topic:   cfg.Kafka.BookingEventsTopic,
	}, nil
}

func (p *Producer) Enabled() bool {
	return p != nil && p.enabled && p.writer != nil
}

func (p *Producer) Close() error {
	if !p.Enabled() {
		return nil
	}
	return p.writer.Close()
}

func (p *Producer) PublishBookingEvent(ctx context.Context, event *BookingEvent) error {
	if !p.Enabled() {
		return nil
	}
	if event == nil {
		return fmt.Errorf("booking event is nil")
	}

	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal booking event: %w", err)
	}

	msg := kafkago.Message{
		Key:   []byte(event.EventID),
		Value: value,
		Time:  event.OccurredAt,
		Headers: []kafkago.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "topic", Value: []byte(p.topic)},
		},
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write kafka message: %w", err)
	}
	return nil
}
