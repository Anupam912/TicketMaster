package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Stream and key configuration constants.
const (
	bookingStream      = "booking:queue"
	purchaseStream     = "purchase:queue"
	jobStatusKeyPrefix = "job:status:"
	jobStatusTTL       = 24 * time.Hour
	dequeueBlockTime   = 5 * time.Second
)

// Job status constants.
const (
	JobStatusPending   = "pending"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
)

// Sentinel errors for queue operations.
var (
	ErrRedisUnavailable = errors.New("redis not available")
	ErrJobNotFound      = errors.New("job not found")
)

// BookingJob represents an async seat reservation request.
type BookingJob struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	EventID    uuid.UUID `json:"event_id"`
	SeatNumber string    `json:"seat_number"`
	CreatedAt  time.Time `json:"created_at"`
}

// PurchaseJob represents an async booking purchase request.
type PurchaseJob struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	BookingID      uuid.UUID `json:"booking_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}

// JobStatus represents the current status of an async job.
type JobStatus struct {
	JobID     uuid.UUID  `json:"job_id"`
	Status    string     `json:"status"`
	BookingID *uuid.UUID `json:"booking_id,omitempty"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Queue manages async job processing via Redis Streams.
type Queue struct {
	redis *redis.Client
}

// NewQueue creates a new Queue instance.
func NewQueue(redisClient *redis.Client) *Queue {
	return &Queue{redis: redisClient}
}

// EnqueueBookingJob adds a booking job to the queue and initializes its status.
func (q *Queue) EnqueueBookingJob(ctx context.Context, job *BookingJob) error {
	if q.redis == nil {
		return ErrRedisUnavailable
	}

	job.ID = uuid.New()
	job.CreatedAt = time.Now()

	status := JobStatus{
		JobID:     job.ID,
		Status:    JobStatusPending,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.CreatedAt,
	}
	if err := q.setJobStatus(ctx, &status); err != nil {
		return fmt.Errorf("set initial job status: %w", err)
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal booking job: %w", err)
	}

	if _, err := q.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: bookingStream,
		Values: map[string]interface{}{"job": string(data)},
	}).Result(); err != nil {
		return fmt.Errorf("add to booking stream: %w", err)
	}

	return nil
}

// DequeueBookingJob retrieves the next booking job from the queue.
// Returns nil, nil if no jobs are available within the timeout.
func (q *Queue) DequeueBookingJob(ctx context.Context, consumerGroup string) (*BookingJob, error) {
	if q.redis == nil {
		return nil, ErrRedisUnavailable
	}

	_ = q.redis.XGroupCreateMkStream(ctx, bookingStream, consumerGroup, "0").Err()

	consumerID := fmt.Sprintf("worker-%s", uuid.New().String()[:8])
	streams, err := q.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerID,
		Streams:  []string{bookingStream, ">"},
		Count:    1,
		Block:    dequeueBlockTime,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("read from booking stream: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	message := streams[0].Messages[0]
	jobData, ok := message.Values["job"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid job data type")
	}

	var job BookingJob
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("unmarshal booking job: %w", err)
	}

	_ = q.redis.XAck(ctx, bookingStream, consumerGroup, message.ID).Err()

	return &job, nil
}

// EnqueuePurchaseJob adds a purchase job to the queue and initializes its status.
func (q *Queue) EnqueuePurchaseJob(ctx context.Context, job *PurchaseJob) error {
	if q.redis == nil {
		return ErrRedisUnavailable
	}

	job.ID = uuid.New()
	job.CreatedAt = time.Now()

	status := JobStatus{
		JobID:     job.ID,
		Status:    JobStatusPending,
		BookingID: &job.BookingID,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.CreatedAt,
	}
	if err := q.setJobStatus(ctx, &status); err != nil {
		return fmt.Errorf("set initial job status: %w", err)
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal purchase job: %w", err)
	}

	if _, err := q.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: purchaseStream,
		Values: map[string]interface{}{"job": string(data)},
	}).Result(); err != nil {
		return fmt.Errorf("add to purchase stream: %w", err)
	}

	return nil
}

// DequeuePurchaseJob retrieves the next purchase job from the queue.
// Returns nil, nil if no jobs are available within the timeout.
func (q *Queue) DequeuePurchaseJob(ctx context.Context, consumerGroup string) (*PurchaseJob, error) {
	if q.redis == nil {
		return nil, ErrRedisUnavailable
	}

	_ = q.redis.XGroupCreateMkStream(ctx, purchaseStream, consumerGroup, "0").Err()

	consumerID := fmt.Sprintf("worker-%s", uuid.New().String()[:8])
	streams, err := q.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerID,
		Streams:  []string{purchaseStream, ">"},
		Count:    1,
		Block:    dequeueBlockTime,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("read from purchase stream: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	message := streams[0].Messages[0]
	jobData, ok := message.Values["job"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid job data type")
	}

	var job PurchaseJob
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("unmarshal purchase job: %w", err)
	}

	_ = q.redis.XAck(ctx, purchaseStream, consumerGroup, message.ID).Err()

	return &job, nil
}

// GetJobStatus retrieves the current status of a job by its ID.
func (q *Queue) GetJobStatus(ctx context.Context, jobID uuid.UUID) (*JobStatus, error) {
	if q.redis == nil {
		return nil, ErrRedisUnavailable
	}

	key := jobStatusKeyPrefix + jobID.String()
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

// CompleteJob marks a job as completed with the resulting booking ID.
func (q *Queue) CompleteJob(ctx context.Context, jobID, bookingID uuid.UUID) error {
	status := &JobStatus{
		JobID:     jobID,
		Status:    JobStatusCompleted,
		BookingID: &bookingID,
	}
	return q.setJobStatus(ctx, status)
}

// FailJob marks a job as failed with an error message.
func (q *Queue) FailJob(ctx context.Context, jobID uuid.UUID, errMsg string) error {
	status := &JobStatus{
		JobID:  jobID,
		Status: JobStatusFailed,
		Error:  errMsg,
	}
	return q.setJobStatus(ctx, status)
}

// setJobStatus stores the job status in Redis with TTL.
func (q *Queue) setJobStatus(ctx context.Context, status *JobStatus) error {
	if q.redis == nil {
		return ErrRedisUnavailable
	}

	status.UpdatedAt = time.Now()
	if status.CreatedAt.IsZero() {
		status.CreatedAt = status.UpdatedAt
	}

	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal job status: %w", err)
	}

	key := jobStatusKeyPrefix + status.JobID.String()
	if err := q.redis.Set(ctx, key, data, jobStatusTTL).Err(); err != nil {
		return fmt.Errorf("set job status: %w", err)
	}

	return nil
}
