package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type BookingJob struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	EventID   uuid.UUID `json:"event_id"`
	SeatNumber string   `json:"seat_number"`
	CreatedAt time.Time `json:"created_at"`
}

type PurchaseJob struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	BookingID uuid.UUID `json:"booking_id"`
	IdempotencyKey string `json:"idempotency_key"`
	CreatedAt time.Time `json:"created_at"`
}

type Queue struct {
	redis *redis.Client
}

func NewQueue(redis *redis.Client) *Queue {
	return &Queue{redis: redis}
}

func (q *Queue) EnqueueBookingJob(ctx context.Context, job *BookingJob) error {
	if q.redis == nil {
		return fmt.Errorf("redis not available")
	}

	job.ID = uuid.New()
	job.CreatedAt = time.Now()

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	_, err = q.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: "booking:queue",
		Values: map[string]interface{}{
			"job": string(data),
		},
	}).Result()

	return err
}

func (q *Queue) DequeueBookingJob(ctx context.Context, consumerGroup string) (*BookingJob, error) {
	if q.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	_ = q.redis.XGroupCreateMkStream(ctx, "booking:queue", consumerGroup, "0").Err()

	streams, err := q.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: fmt.Sprintf("worker-%s", uuid.New().String()[:8]),
		Streams:  []string{"booking:queue", ">"},
		Count:    1,
		Block:    time.Second * 5,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	message := streams[0].Messages[0]
	jobData := message.Values["job"].(string)

	var job BookingJob
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, err
	}

	q.redis.XAck(ctx, "booking:queue", consumerGroup, message.ID)

	return &job, nil
}

func (q *Queue) EnqueuePurchaseJob(ctx context.Context, job *PurchaseJob) error {
	if q.redis == nil {
		return fmt.Errorf("redis not available")
	}

	job.ID = uuid.New()
	job.CreatedAt = time.Now()

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	_, err = q.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: "purchase:queue",
		Values: map[string]interface{}{
			"job": string(data),
		},
	}).Result()

	return err
}

func (q *Queue) DequeuePurchaseJob(ctx context.Context, consumerGroup string) (*PurchaseJob, error) {
	if q.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	_ = q.redis.XGroupCreateMkStream(ctx, "purchase:queue", consumerGroup, "0").Err()

	streams, err := q.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: fmt.Sprintf("worker-%s", uuid.New().String()[:8]),
		Streams:  []string{"purchase:queue", ">"},
		Count:    1,
		Block:    time.Second * 5,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	message := streams[0].Messages[0]
	jobData := message.Values["job"].(string)

	var job PurchaseJob
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, err
	}

	q.redis.XAck(ctx, "purchase:queue", consumerGroup, message.ID)

	return &job, nil
}
