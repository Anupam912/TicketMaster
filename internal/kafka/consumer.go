package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"event-ticketing-system/internal/config"

	kafkago "github.com/segmentio/kafka-go"
)

const bookingEventFetchTimeout = 5 * time.Second

type ConsumedBookingEvent struct {
	Event     *BookingEvent
	RawValue  []byte
	Partition int
	Offset    int64
}

type BookingEventConsumer struct {
	reader    *kafkago.Reader
	dlqWriter *kafkago.Writer
	enabled   bool
	brokers   []string
	groupID   string
	topic     string
	dlqTopic  string
	mu        sync.Mutex
}

func NewBookingEventConsumer(cfg *config.Config, groupID string) *BookingEventConsumer {
	if cfg == nil || len(cfg.Kafka.Brokers) == 0 {
		return &BookingEventConsumer{}
	}
	if groupID == "" {
		groupID = "booking-event-consumers"
	}

	consumer := &BookingEventConsumer{
		dlqWriter: &kafkago.Writer{
			Addr:                   kafkago.TCP(cfg.Kafka.Brokers...),
			Topic:                  cfg.Kafka.BookingEventsDLQTopic,
			RequiredAcks:           kafkaRequiredAcks(cfg.Kafka.RequiredAcks),
			AllowAutoTopicCreation: true,
			Balancer:               &kafkago.Hash{},
		},
		enabled:  true,
		brokers:  cfg.Kafka.Brokers,
		groupID:  groupID,
		topic:    cfg.Kafka.BookingEventsTopic,
		dlqTopic: cfg.Kafka.BookingEventsDLQTopic,
	}
	consumer.reader = consumer.newReader()
	return consumer
}

func (c *BookingEventConsumer) Enabled() bool {
	return c != nil && c.enabled && c.reader != nil
}

func (c *BookingEventConsumer) Close() error {
	if !c.Enabled() {
		return nil
	}
	var err error
	if closeErr := c.reader.Close(); closeErr != nil {
		err = closeErr
	}
	if c.dlqWriter != nil {
		if closeErr := c.dlqWriter.Close(); closeErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; %v", err, closeErr)
			} else {
				err = closeErr
			}
		}
	}
	return err
}

func (c *BookingEventConsumer) Fetch(ctx context.Context) (*ConsumedBookingEvent, error) {
	if !c.Enabled() {
		return nil, nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, bookingEventFetchTimeout)
	defer cancel()

	msg, err := c.reader.FetchMessage(fetchCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, nil
		}
		if errors.Is(err, io.EOF) {
			c.resetReader()
			return nil, nil
		}
		return nil, fmt.Errorf("fetch booking event: %w", err)
	}

	var event BookingEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return &ConsumedBookingEvent{
			RawValue:  msg.Value,
			Partition: msg.Partition,
			Offset:    msg.Offset,
		}, fmt.Errorf("unmarshal booking event: %w", err)
	}

	return &ConsumedBookingEvent{
		Event:     &event,
		RawValue:  msg.Value,
		Partition: msg.Partition,
		Offset:    msg.Offset,
	}, nil
}

func (c *BookingEventConsumer) newReader() *kafkago.Reader {
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        c.brokers,
		GroupID:        c.groupID,
		Topic:          c.topic,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        bookingEventFetchTimeout,
		CommitInterval: 0,
	})
}

func (c *BookingEventConsumer) resetReader() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.reader != nil {
		_ = c.reader.Close()
	}
	c.reader = c.newReader()
}

func (c *BookingEventConsumer) Ack(ctx context.Context, consumed *ConsumedBookingEvent) error {
	if !c.Enabled() || consumed == nil {
		return nil
	}
	return c.reader.CommitMessages(ctx, kafkago.Message{
		Topic:     c.topic,
		Partition: consumed.Partition,
		Offset:    consumed.Offset,
	})
}

func (c *BookingEventConsumer) DeadLetter(ctx context.Context, consumed *ConsumedBookingEvent, reason string) error {
	if !c.Enabled() || c.dlqWriter == nil || consumed == nil {
		return nil
	}

	key := []byte(fmt.Sprintf("%d:%d", consumed.Partition, consumed.Offset))
	if consumed.Event != nil && consumed.Event.EventID != "" {
		key = []byte(consumed.Event.EventID)
	}

	headers := []kafkago.Header{
		{Key: "source_topic", Value: []byte(c.topic)},
		{Key: "dlq_topic", Value: []byte(c.dlqTopic)},
		{Key: "reason", Value: []byte(reason)},
	}
	if consumed.Event != nil {
		headers = append(headers, kafkago.Header{Key: "event_type", Value: []byte(consumed.Event.EventType)})
	}

	if err := c.dlqWriter.WriteMessages(ctx, kafkago.Message{
		Key:     key,
		Value:   consumed.RawValue,
		Time:    time.Now().UTC(),
		Headers: headers,
	}); err != nil {
		return fmt.Errorf("write booking event dlq: %w", err)
	}
	return nil
}

func kafkaRequiredAcks(value string) kafkago.RequiredAcks {
	switch strings.ToLower(value) {
	case "all":
		return kafkago.RequireAll
	case "none":
		return kafkago.RequireNone
	default:
		return kafkago.RequireOne
	}
}
