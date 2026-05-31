package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ReservationExpiryKey         = "reservation:expiry"
	ReservationExpiryInflightKey = "reservation:expiry:inflight"
	ExpiryPollBatchSize          = 100
	ExpiryClaimVisibilityTimeout = 2 * time.Minute
	// Overscan due ZSET when filtering by partition so each poller can fill its batch.
	ExpiryClaimScanMultiplier = 10
	ExpiryClaimScanMax        = 1000
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
	return execRedisPipeline(pipe.Exec(ctx))
}

func execRedisPipeline(cmds []redis.Cmder, err error) error {
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	for _, cmd := range cmds {
		if cmdErr := cmd.Err(); cmdErr != nil && !errors.Is(cmdErr, redis.Nil) {
			return fmt.Errorf("redis %s: %w", cmd.Name(), cmdErr)
		}
	}
	return nil
}

// PollDue claims due expiry members for a single partition. Each partition index must
// be owned by at most one active worker process to avoid duplicate releases.
func (q *ExpiryQueue) PollDue(
	ctx context.Context,
	now time.Time,
	limit int,
	partitionIndex int,
	partitionCount int,
) ([]string, error) {
	if q.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	if limit <= 0 {
		limit = ExpiryPollBatchSize
	}
	if partitionCount <= 0 {
		return nil, fmt.Errorf("partition count must be positive")
	}
	if partitionIndex < 0 || partitionIndex >= partitionCount {
		return nil, fmt.Errorf("partition index %d out of range for count %d", partitionIndex, partitionCount)
	}

	ids, err := claimExpiryScript.Run(
		ctx,
		q.redis,
		[]string{ReservationExpiryKey, ReservationExpiryInflightKey},
		now.Unix(),
		now.Add(-ExpiryClaimVisibilityTimeout).Unix(),
		now.Add(ExpiryClaimVisibilityTimeout).Unix(),
		limit,
		partitionIndex,
		partitionCount,
		ExpiryClaimScanMultiplier,
		ExpiryClaimScanMax,
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
local partition_index = tonumber(ARGV[5])
local partition_count = tonumber(ARGV[6])
local scan_multiplier = tonumber(ARGV[7])
local scan_max = tonumber(ARGV[8])

local function expiry_partition_index(id)
	local h = 0
	for i = 1, #id do
		h = (h * 31 + string.byte(id, i)) % 2147483647
	end
	return h % partition_count
end

local function belongs_to_partition(id)
	return expiry_partition_index(id) == partition_index
end

local scan_limit = limit * scan_multiplier
if scan_limit > scan_max then
	scan_limit = scan_max
end
if scan_limit < limit then
	scan_limit = limit
end

local stale = redis.call("ZRANGEBYSCORE", inflight_key, "-inf", stale_before, "LIMIT", 0, scan_limit)
for _, id in ipairs(stale) do
	if belongs_to_partition(id) then
		redis.call("ZREM", inflight_key, id)
		redis.call("ZADD", pending_key, now, id)
	end
end

local claimed = {}
local candidates = redis.call("ZRANGEBYSCORE", pending_key, "-inf", now, "LIMIT", 0, scan_limit)
for _, id in ipairs(candidates) do
	if belongs_to_partition(id) then
		redis.call("ZREM", pending_key, id)
		redis.call("ZADD", inflight_key, claim_until, id)
		table.insert(claimed, id)
		if #claimed >= limit then
			break
		end
	end
end

return claimed
`)
