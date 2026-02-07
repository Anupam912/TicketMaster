package queue

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ReservationExpiryKey = "reservation:expiry"
	ExpiryPollBatchSize = 100
)
 
type ExpiryQueue struct {
	redis *redis.Client
}

func NewExpiryQueue(redis *redis.Client) *ExpiryQueue {
	return &ExpiryQueue{redis: redis}
}

func (q *ExpiryQueue) Add(ctx context.Context, bookingID string, releaseAt time.Time) error {
	if q.redis == nil {
		return fmt.Errorf("redis not available")
	}
	score := float64(releaseAt.Unix())
	return q.redis.ZAdd(ctx, ReservationExpiryKey, redis.Z{Score: score, Member: bookingID}).Err()
}

func (q *ExpiryQueue) Remove(ctx context.Context, bookingID string) error {
	if q.redis == nil {
		return nil
	}
	return q.redis.ZRem(ctx, ReservationExpiryKey, bookingID).Err()
}

func (q *ExpiryQueue) PollDue(ctx context.Context, now time.Time, limit int) ([]string, error) {
	if q.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}
	
	if limit <= 0 {
		limit = ExpiryPollBatchSize
	}

	nowFloat := float64(now.Unix())
	ids, err := q.redis.ZRangeByScore(ctx, ReservationExpiryKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatFloat(nowFloat, 'f', -1, 64),
		Count: int64(limit),
	}).Result()

	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	args := make([]interface{}, len(ids))

	for i, id := range ids {
		args[i] = id
	}

	if err := q.redis.ZRem(ctx, ReservationExpiryKey, args...).Err(); err != nil {
		return nil, err
	}
	
	return ids, nil
}
