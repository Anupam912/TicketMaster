package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"event-ticketing-system/internal/config"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	jobStatusKeyPrefix = "job:status:"
	jobStatusTTL       = 24 * time.Hour
	dequeueBlockTime   = 5 * time.Second
	defaultMaxRetries  = 3

	defaultBookingTopic  = "booking-commands"
	defaultPurchaseTopic = "purchase-commands"
	defaultBookingDLQ    = "booking-commands-dlq"
	defaultPurchaseDLQ   = "purchase-commands-dlq"

	bookingDepthKey  = "kafka:queue:booking:depth"
	purchaseDepthKey = "kafka:queue:purchase:depth"
	bookingDLQKey    = "kafka:queue:booking:dlq:depth"
	purchaseDLQKey   = "kafka:queue:purchase:dlq:depth"
)

const (
	JobStatusPending    = "pending"
	JobStatusRetrying   = "retrying"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
	JobStatusDeadLetter = "dead_lettered"
)

var (
	ErrRedisUnavailable = errors.New("redis not available")
	ErrJobNotFound      = errors.New("job not found")
	ErrKafkaUnavailable = errors.New("kafka queue not configured")
)

type BookingJob struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	EventID    uuid.UUID `json:"event_id"`
	SeatNumber string    `json:"seat_number"`
	Attempt    int       `json:"attempt"`
	CreatedAt  time.Time `json:"created_at"`
}

type PurchaseJob struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	BookingID      uuid.UUID `json:"booking_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Attempt        int       `json:"attempt"`
	CreatedAt      time.Time `json:"created_at"`
}

type JobStatus struct {
	JobID     uuid.UUID  `json:"job_id"`
	Status    string     `json:"status"`
	BookingID *uuid.UUID `json:"booking_id,omitempty"`
	Attempt   int        `json:"attempt,omitempty"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type QueueMetrics struct {
	BookingQueueLength  int64 `json:"booking_queue_length"`
	PurchaseQueueLength int64 `json:"purchase_queue_length"`
	BookingDLQLength    int64 `json:"booking_dlq_length"`
	PurchaseDLQLength   int64 `json:"purchase_dlq_length"`
	BookingPending      int64 `json:"booking_pending"`
	PurchasePending     int64 `json:"purchase_pending"`
	MaxRetries          int   `json:"max_retries"`
}

type Queue struct {
	redis *redis.Client

	maxRetries int

	kafkaEnabled bool
	brokers      []string

	bookingTopic  string
	purchaseTopic string
	bookingDLQ    string
	purchaseDLQ   string

	bookingWriter  *kafkago.Writer
	purchaseWriter *kafkago.Writer
	bookingDLQW    *kafkago.Writer
	purchaseDLQW   *kafkago.Writer

	mu              sync.Mutex
	bookingReaders  map[string]*kafkago.Reader
	purchaseReaders map[string]*kafkago.Reader

	localStatus sync.Map
}

func NewQueue(cfg *config.Config, redisClient *redis.Client) *Queue {
	q := &Queue{
		redis:           redisClient,
		maxRetries:      defaultMaxRetries,
		bookingReaders:  make(map[string]*kafkago.Reader),
		purchaseReaders: make(map[string]*kafkago.Reader),
		bookingTopic:    defaultBookingTopic,
		purchaseTopic:   defaultPurchaseTopic,
		bookingDLQ:      defaultBookingDLQ,
		purchaseDLQ:     defaultPurchaseDLQ,
	}

	if cfg != nil {
		if cfg.Kafka.BookingCommandsTopic != "" {
			q.bookingTopic = cfg.Kafka.BookingCommandsTopic
		}
		if cfg.Kafka.PurchaseCommandsTopic != "" {
			q.purchaseTopic = cfg.Kafka.PurchaseCommandsTopic
		}
		if cfg.Kafka.BookingDLQTopic != "" {
			q.bookingDLQ = cfg.Kafka.BookingDLQTopic
		}
		if cfg.Kafka.PurchaseDLQTopic != "" {
			q.purchaseDLQ = cfg.Kafka.PurchaseDLQTopic
		}
		if len(cfg.Kafka.Brokers) > 0 {
			q.kafkaEnabled = true
			q.brokers = cfg.Kafka.Brokers
			q.bookingWriter = newKafkaWriter(cfg.Kafka.Brokers, q.bookingTopic)
			q.purchaseWriter = newKafkaWriter(cfg.Kafka.Brokers, q.purchaseTopic)
			q.bookingDLQW = newKafkaWriter(cfg.Kafka.Brokers, q.bookingDLQ)
			q.purchaseDLQW = newKafkaWriter(cfg.Kafka.Brokers, q.purchaseDLQ)
		}
	}

	return q
}

func newKafkaWriter(brokers []string, topic string) *kafkago.Writer {
	return &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Topic:                  topic,
		RequiredAcks:           kafkago.RequireOne,
		AllowAutoTopicCreation: true,
		Balancer:               &kafkago.LeastBytes{},
	}
}

func (q *Queue) Enabled() bool {
	return q != nil && q.kafkaEnabled
}

func (q *Queue) SetMaxRetries(maxRetries int) {
	if maxRetries < 0 {
		maxRetries = 0
	}
	q.maxRetries = maxRetries
}

func (q *Queue) EnqueueBookingJob(ctx context.Context, job *BookingJob) error {
	if !q.kafkaEnabled {
		return ErrKafkaUnavailable
	}

	job.ID = uuid.New()
	job.Attempt = 0
	job.CreatedAt = time.Now().UTC()

	status := JobStatus{JobID: job.ID, Status: JobStatusPending, Attempt: 0, CreatedAt: job.CreatedAt, UpdatedAt: job.CreatedAt}
	if err := q.setJobStatus(ctx, &status); err != nil {
		return fmt.Errorf("set initial booking job status: %w", err)
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal booking job: %w", err)
	}

	msg := kafkago.Message{Key: []byte(job.ID.String()), Value: payload, Time: time.Now().UTC()}
	if err := q.bookingWriter.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write booking command: %w", err)
	}
	q.bumpCounter(ctx, bookingDepthKey, 1)
	return nil
}

func (q *Queue) EnqueuePurchaseJob(ctx context.Context, job *PurchaseJob) error {
	if !q.kafkaEnabled {
		return ErrKafkaUnavailable
	}

	job.ID = uuid.New()
	job.Attempt = 0
	job.CreatedAt = time.Now().UTC()

	status := JobStatus{JobID: job.ID, Status: JobStatusPending, BookingID: &job.BookingID, Attempt: 0, CreatedAt: job.CreatedAt, UpdatedAt: job.CreatedAt}
	if err := q.setJobStatus(ctx, &status); err != nil {
		return fmt.Errorf("set initial purchase job status: %w", err)
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal purchase job: %w", err)
	}

	msg := kafkago.Message{Key: []byte(job.ID.String()), Value: payload, Time: time.Now().UTC()}
	if err := q.purchaseWriter.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write purchase command: %w", err)
	}
	q.bumpCounter(ctx, purchaseDepthKey, 1)
	return nil
}

func (q *Queue) DequeueBookingJob(ctx context.Context, consumerGroup, _ string) (*BookingJob, string, error) {
	if !q.kafkaEnabled {
		return nil, "", ErrKafkaUnavailable
	}

	reader := q.getBookingReader(consumerGroup)
	pollCtx, cancel := context.WithTimeout(ctx, dequeueBlockTime)
	defer cancel()

	msg, err := reader.FetchMessage(pollCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("fetch booking command: %w", err)
	}

	var job BookingJob
	if err := json.Unmarshal(msg.Value, &job); err != nil {
		return nil, "", fmt.Errorf("unmarshal booking command: %w", err)
	}

	q.bumpCounter(ctx, bookingDepthKey, -1)
	messageID := fmt.Sprintf("%d:%d", msg.Partition, msg.Offset)
	return &job, messageID, nil
}

func (q *Queue) DequeuePurchaseJob(ctx context.Context, consumerGroup, _ string) (*PurchaseJob, string, error) {
	if !q.kafkaEnabled {
		return nil, "", ErrKafkaUnavailable
	}

	reader := q.getPurchaseReader(consumerGroup)
	pollCtx, cancel := context.WithTimeout(ctx, dequeueBlockTime)
	defer cancel()

	msg, err := reader.FetchMessage(pollCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("fetch purchase command: %w", err)
	}

	var job PurchaseJob
	if err := json.Unmarshal(msg.Value, &job); err != nil {
		return nil, "", fmt.Errorf("unmarshal purchase command: %w", err)
	}

	q.bumpCounter(ctx, purchaseDepthKey, -1)
	messageID := fmt.Sprintf("%d:%d", msg.Partition, msg.Offset)
	return &job, messageID, nil
}

func (q *Queue) AckBookingJob(ctx context.Context, consumerGroup, messageID string) error {
	if !q.kafkaEnabled {
		return ErrKafkaUnavailable
	}
	partition, offset, err := parseMessageID(messageID)
	if err != nil {
		return err
	}
	reader := q.getBookingReader(consumerGroup)
	return reader.CommitMessages(ctx, kafkago.Message{Topic: q.bookingTopic, Partition: partition, Offset: offset})
}

func (q *Queue) AckPurchaseJob(ctx context.Context, consumerGroup, messageID string) error {
	if !q.kafkaEnabled {
		return ErrKafkaUnavailable
	}
	partition, offset, err := parseMessageID(messageID)
	if err != nil {
		return err
	}
	reader := q.getPurchaseReader(consumerGroup)
	return reader.CommitMessages(ctx, kafkago.Message{Topic: q.purchaseTopic, Partition: partition, Offset: offset})
}

func (q *Queue) GetJobStatus(ctx context.Context, jobID uuid.UUID) (*JobStatus, error) {
	key := jobStatusKeyPrefix + jobID.String()

	if q.redis != nil {
		data, err := q.redis.Get(ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return nil, ErrJobNotFound
			}
			return nil, fmt.Errorf("get job status: %w", err)
		}
		var status JobStatus
		if err := json.Unmarshal([]byte(data), &status); err != nil {
			return nil, fmt.Errorf("unmarshal job status: %w", err)
		}
		return &status, nil
	}

	if v, ok := q.localStatus.Load(key); ok {
		data, _ := v.([]byte)
		var status JobStatus
		if err := json.Unmarshal(data, &status); err != nil {
			return nil, fmt.Errorf("unmarshal local job status: %w", err)
		}
		return &status, nil
	}

	return nil, ErrJobNotFound
}

func (q *Queue) CompleteJob(ctx context.Context, jobID, bookingID uuid.UUID) error {
	status := &JobStatus{JobID: jobID, Status: JobStatusCompleted, BookingID: &bookingID}
	return q.setJobStatus(ctx, status)
}

func (q *Queue) FailJob(ctx context.Context, jobID uuid.UUID, errMsg string) error {
	status := &JobStatus{JobID: jobID, Status: JobStatusFailed, Error: errMsg}
	return q.setJobStatus(ctx, status)
}

func (q *Queue) HandleBookingJobFailure(ctx context.Context, job *BookingJob, errMsg string) error {
	if !q.kafkaEnabled {
		return ErrKafkaUnavailable
	}
	if job.Attempt < q.maxRetries {
		job.Attempt++
		status := &JobStatus{JobID: job.ID, Status: JobStatusRetrying, Attempt: job.Attempt, Error: errMsg}
		if err := q.setJobStatus(ctx, status); err != nil {
			return err
		}
		payload, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("marshal booking retry: %w", err)
		}
		if err := q.bookingWriter.WriteMessages(ctx, kafkago.Message{Key: []byte(job.ID.String()), Value: payload, Time: time.Now().UTC()}); err != nil {
			return fmt.Errorf("write booking retry: %w", err)
		}
		q.bumpCounter(ctx, bookingDepthKey, 1)
		return nil
	}

	status := &JobStatus{JobID: job.ID, Status: JobStatusDeadLetter, Attempt: job.Attempt, Error: errMsg}
	if err := q.setJobStatus(ctx, status); err != nil {
		return err
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal booking dlq: %w", err)
	}
	if err := q.bookingDLQW.WriteMessages(ctx, kafkago.Message{Key: []byte(job.ID.String()), Value: payload, Time: time.Now().UTC()}); err != nil {
		return fmt.Errorf("write booking dlq: %w", err)
	}
	q.bumpCounter(ctx, bookingDLQKey, 1)
	return nil
}

func (q *Queue) HandlePurchaseJobFailure(ctx context.Context, job *PurchaseJob, errMsg string) error {
	if !q.kafkaEnabled {
		return ErrKafkaUnavailable
	}
	if job.Attempt < q.maxRetries {
		job.Attempt++
		status := &JobStatus{JobID: job.ID, Status: JobStatusRetrying, Attempt: job.Attempt, Error: errMsg}
		if err := q.setJobStatus(ctx, status); err != nil {
			return err
		}
		payload, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("marshal purchase retry: %w", err)
		}
		if err := q.purchaseWriter.WriteMessages(ctx, kafkago.Message{Key: []byte(job.ID.String()), Value: payload, Time: time.Now().UTC()}); err != nil {
			return fmt.Errorf("write purchase retry: %w", err)
		}
		q.bumpCounter(ctx, purchaseDepthKey, 1)
		return nil
	}

	status := &JobStatus{JobID: job.ID, Status: JobStatusDeadLetter, Attempt: job.Attempt, Error: errMsg}
	if err := q.setJobStatus(ctx, status); err != nil {
		return err
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal purchase dlq: %w", err)
	}
	if err := q.purchaseDLQW.WriteMessages(ctx, kafkago.Message{Key: []byte(job.ID.String()), Value: payload, Time: time.Now().UTC()}); err != nil {
		return fmt.Errorf("write purchase dlq: %w", err)
	}
	q.bumpCounter(ctx, purchaseDLQKey, 1)
	return nil
}

func (q *Queue) GetMetrics(ctx context.Context, _, _ string) (*QueueMetrics, error) {
	if !q.kafkaEnabled {
		return nil, ErrKafkaUnavailable
	}
	return &QueueMetrics{
		BookingQueueLength:  q.readCounter(ctx, bookingDepthKey),
		PurchaseQueueLength: q.readCounter(ctx, purchaseDepthKey),
		BookingDLQLength:    q.readCounter(ctx, bookingDLQKey),
		PurchaseDLQLength:   q.readCounter(ctx, purchaseDLQKey),
		BookingPending:      0,
		PurchasePending:     0,
		MaxRetries:          q.maxRetries,
	}, nil
}

func (q *Queue) setJobStatus(ctx context.Context, status *JobStatus) error {
	status.UpdatedAt = time.Now().UTC()
	if status.CreatedAt.IsZero() {
		status.CreatedAt = status.UpdatedAt
	}
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal job status: %w", err)
	}

	key := jobStatusKeyPrefix + status.JobID.String()
	if q.redis != nil {
		if err := q.redis.Set(ctx, key, data, jobStatusTTL).Err(); err != nil {
			return fmt.Errorf("set job status: %w", err)
		}
		return nil
	}

	q.localStatus.Store(key, data)
	return nil
}

func (q *Queue) getBookingReader(group string) *kafkago.Reader {
	q.mu.Lock()
	defer q.mu.Unlock()
	if r, ok := q.bookingReaders[group]; ok {
		return r
	}
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        q.brokers,
		GroupID:        group,
		Topic:          q.bookingTopic,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        dequeueBlockTime,
		CommitInterval: 0,
	})
	q.bookingReaders[group] = r
	return r
}

func (q *Queue) getPurchaseReader(group string) *kafkago.Reader {
	q.mu.Lock()
	defer q.mu.Unlock()
	if r, ok := q.purchaseReaders[group]; ok {
		return r
	}
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        q.brokers,
		GroupID:        group,
		Topic:          q.purchaseTopic,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        dequeueBlockTime,
		CommitInterval: 0,
	})
	q.purchaseReaders[group] = r
	return r
}

func parseMessageID(messageID string) (int, int64, error) {
	parts := strings.Split(messageID, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid message id: %s", messageID)
	}
	partition, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid partition in message id: %w", err)
	}
	offset, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid offset in message id: %w", err)
	}
	return partition, offset, nil
}

func (q *Queue) bumpCounter(ctx context.Context, key string, delta int64) {
	if q.redis == nil {
		return
	}
	if delta >= 0 {
		_, _ = q.redis.IncrBy(ctx, key, delta).Result()
		return
	}
	newVal, err := q.redis.DecrBy(ctx, key, -delta).Result()
	if err == nil && newVal < 0 {
		_ = q.redis.Set(ctx, key, 0, 0).Err()
	}
}

func (q *Queue) readCounter(ctx context.Context, key string) int64 {
	if q.redis == nil {
		return 0
	}
	v, err := q.redis.Get(ctx, key).Int64()
	if err != nil {
		return 0
	}
	return v
}
