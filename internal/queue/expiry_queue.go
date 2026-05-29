package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ReservationExpiryKey         = "reservation:expiry"
	ReservationExpiryInflightKey = "reservation:expiry:inflight"
	ExpiryPollBatchSize          = 100
	ExpiryClaimVisibilityTimeout = 2 * time.Minute
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
	pipe := q.redis.Pipeline()
	pipe.ZRem(ctx, ReservationExpiryKey, bookingID)
	pipe.ZRem(ctx, ReservationExpiryInflightKey, bookingID)
	_, err := pipe.Exec(ctx)
	return err
}

func (q *ExpiryQueue) PollDue(ctx context.Context, now time.Time, limit int) ([]string, error) {
	if q.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	if limit <= 0 {
		limit = ExpiryPollBatchSize
	}

	ids, err := claimExpiryScript.Run(
		ctx,
		q.redis,
		[]string{ReservationExpiryKey, ReservationExpiryInflightKey},
		now.Unix(),
		now.Add(-ExpiryClaimVisibilityTimeout).Unix(),
		now.Add(ExpiryClaimVisibilityTimeout).Unix(),
		limit,
	).StringSlice()
	if err != nil {
		return nil, err
	}
	return ids, nil
}

var claimExpiryScript = redis.NewScript(`
local pending_key = KEYS[1]
local inflight_key = KEYS[2]
local now = tonumber(ARGV[1])
local stale_before = tonumber(ARGV[2])
local claim_until = tonumber(ARGV[3])
local limit = tonumber(ARGV[4])

local stale = redis.call("ZRANGEBYSCORE", inflight_key, "-inf", stale_before, "LIMIT", 0, limit)
for _, id in ipairs(stale) do
	redis.call("ZREM", inflight_key, id)
	redis.call("ZADD", pending_key, now, id)
end

local ids = redis.call("ZRANGEBYSCORE", pending_key, "-inf", now, "LIMIT", 0, limit)
for _, id in ipairs(ids) do
	redis.call("ZREM", pending_key, id)
	redis.call("ZADD", inflight_key, claim_until, id)
end

return ids
`)
