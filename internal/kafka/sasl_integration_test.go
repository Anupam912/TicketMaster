package kafka_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/kafka"
	"event-ticketing-system/internal/queue"

	"github.com/google/uuid"
)

// Run against an isolated SASL/PLAIN broker, never production:
// TEST_KAFKA_BROKER=localhost:19094 TEST_KAFKA_USERNAME=test
// TEST_KAFKA_PASSWORD=local-test-password go test ./internal/kafka -run TestSASLBroker -v
func TestSASLBroker(t *testing.T) {
	broker := os.Getenv("TEST_KAFKA_BROKER")
	if broker == "" {
		t.Skip("TEST_KAFKA_BROKER not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	prefix := fmt.Sprintf("sasl-test-%d", time.Now().UnixNano())
	cfg := &config.Config{Kafka: config.KafkaConfig{
		Brokers: []string{broker}, SecurityProtocol: "SASL_PLAINTEXT", SASLMechanism: "PLAIN",
		SASLUsername: os.Getenv("TEST_KAFKA_USERNAME"), SASLPassword: os.Getenv("TEST_KAFKA_PASSWORD"),
		ClientID: prefix, RequiredAcks: "leader", TopicPartitions: 1, TopicReplicationFactor: 1,
		BookingEventsTopic: prefix + "-events", BookingEventsDLQTopic: prefix + "-events-dlq",
		BookingCommandsTopic: prefix + "-booking", PurchaseCommandsTopic: prefix + "-purchase",
		BookingDLQTopic: prefix + "-booking-dlq", PurchaseDLQTopic: prefix + "-purchase-dlq",
	}}
	if err := kafka.EnsureTopics(ctx, cfg); err != nil {
		t.Fatal("authenticated topic initialization:", err)
	}
	bad := cfg.Kafka
	bad.SASLPassword = "incorrect-password"
	badClient, err := kafka.NewClient(bad)
	if err != nil {
		t.Fatal(err)
	}
	defer badClient.Close()
	if conn, err := badClient.Dialer.DialContext(ctx, "tcp", broker); err == nil {
		conn.Close()
		t.Fatal("broker accepted incorrect credentials")
	}

	producer, err := kafka.NewProducer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()
	eventID := uuid.NewString()
	if err := producer.PublishBookingEvent(ctx, &kafka.BookingEvent{EventID: eventID, OccurredAt: time.Now()}); err != nil {
		t.Fatal("authenticated event publish:", err)
	}
	consumer, err := kafka.NewBookingEventConsumer(cfg, prefix+"-event-group")
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	event, err := consumer.Fetch(ctx)
	if err != nil || event == nil || event.Event == nil || event.Event.EventID != eventID {
		t.Fatalf("authenticated event read failed: %v", err)
	}
	if err := consumer.DeadLetter(ctx, event, "integration-test"); err != nil {
		t.Fatal("authenticated event DLQ:", err)
	}
	if err := consumer.Ack(ctx, event); err != nil {
		t.Fatal(err)
	}

	q, err := queue.NewQueue(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	bookingDone := make(chan uuid.UUID, 1)
	purchaseDone := make(chan uuid.UUID, 1)
	bookingID := uuid.New()
	if err := q.StartBookingConsumer(ctx, func(ctx context.Context, job *queue.BookingJob, _ string) error {
		bookingDone <- job.ID
		return q.CompleteJob(ctx, job.ID, bookingID)
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.StartPurchaseConsumer(ctx, func(ctx context.Context, job *queue.PurchaseJob, _ string) error {
		purchaseDone <- job.ID
		return q.CompleteJob(ctx, job.ID, job.BookingID)
	}); err != nil {
		t.Fatal(err)
	}
	job := &queue.BookingJob{UserID: uuid.New(), EventID: uuid.New(), SeatNumber: "A1"}
	if err := q.EnqueueBookingJob(ctx, job); err != nil {
		t.Fatal("authenticated booking publish:", err)
	}
	select {
	case id := <-bookingDone:
		if id != job.ID {
			t.Fatal("wrong booking command consumed")
		}
	case <-ctx.Done():
		t.Fatal("authenticated booking consumer:", ctx.Err())
	}
	purchase := &queue.PurchaseJob{UserID: job.UserID, BookingID: bookingID}
	if err := q.EnqueuePurchaseJob(ctx, purchase); err != nil {
		t.Fatal("authenticated purchase publish:", err)
	}
	select {
	case id := <-purchaseDone:
		if id != purchase.ID {
			t.Fatal("wrong purchase command consumed")
		}
	case <-ctx.Done():
		t.Fatal("authenticated purchase consumer:", ctx.Err())
	}
	q.SetMaxRetries(0)
	if err := q.HandleBookingJobFailure(ctx, job, "integration-test"); err != nil {
		t.Fatal("authenticated booking DLQ:", err)
	}
	if err := q.HandlePurchaseJobFailure(ctx, purchase, "integration-test"); err != nil {
		t.Fatal("authenticated purchase DLQ:", err)
	}
}
