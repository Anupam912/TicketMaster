package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"event-ticketing-system/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	redis  *redis.Client
	config *config.Config
	// In-memory rate limiter as fallback
	limiter *rate.Limiter

	eventLimitersMu sync.Mutex
	eventLimiters   map[string]*rate.Limiter
}

func NewRateLimiter(redis *redis.Client, cfg *config.Config) *RateLimiter {
	limiter := rate.NewLimiter(rate.Every(time.Second/10), 10)

	return &RateLimiter{
		redis:   redis,
		config:  cfg,
		limiter: limiter,

		eventLimiters: make(map[string]*rate.Limiter),
	}
}

type admissionRequest struct {
	EventID uuid.UUID `json:"event_id"`
}

func (rl *RateLimiter) EventAdmissionControl(maxEventRequests, maxClientRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID, err := readEventID(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event_id"})
			c.Abort()
			return
		}

		clientID := c.ClientIP()
		if userID, ok := c.Get("user_id"); ok {
			clientID = fmt.Sprintf("%v", userID)
		}

		if rl.redis != nil {
			allowed, err := rl.checkRedisEventAdmission(
				c.Request.Context(),
				eventID.String(),
				clientID,
				maxEventRequests,
				maxClientRequests,
				window,
			)
			if err == nil {
				if !allowed {
					writeAdmissionRejected(c)
					return
				}
				c.Next()
				return
			}
		}

		if !rl.allowLocalEvent(eventID.String(), maxEventRequests, window) {
			writeAdmissionRejected(c)
			return
		}

		c.Next()
	}
}

func (rl *RateLimiter) VirtualWaitingRoom(maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if rl.redis != nil {
			allowed, err := rl.checkRedisRateLimit(c.Request.Context(), clientIP, maxRequests, window)
			if err == nil {
				if !allowed {
					c.JSON(http.StatusTooManyRequests, gin.H{
						"error":   "too many requests, please wait",
						"message": "You are in the virtual waiting room. Please try again in a moment.",
					})
					c.Abort()
					return
				}
				c.Next()
				return
			}
		}

		if !rl.limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "too many requests, please wait",
				"message": "You are in the virtual waiting room. Please try again in a moment.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (rl *RateLimiter) checkRedisRateLimit(ctx context.Context, key string, maxRequests int, window time.Duration) (bool, error) {
	redisKey := fmt.Sprintf("ratelimit:%s", key)
	now := time.Now()
	windowStart := now.Add(-window)

	count, err := rl.redis.ZCount(ctx, redisKey, fmt.Sprintf("%d", windowStart.Unix()), fmt.Sprintf("%d", now.Unix())).Result()
	if err != nil {
		return false, err
	}

	if count < int64(maxRequests) {
		member := fmt.Sprintf("%d", now.UnixNano())
		pipe := rl.redis.Pipeline()
		pipe.ZAdd(ctx, redisKey, redis.Z{
			Score:  float64(now.Unix()),
			Member: member,
		})
		pipe.Expire(ctx, redisKey, window)
		pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart.Unix()))
		_, err := pipe.Exec(ctx)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func (rl *RateLimiter) checkRedisEventAdmission(
	ctx context.Context,
	eventID string,
	clientID string,
	maxEventRequests int,
	maxClientRequests int,
	window time.Duration,
) (bool, error) {
	if maxEventRequests <= 0 || maxClientRequests <= 0 {
		return true, nil
	}

	now := time.Now()
	windowStart := now.Add(-window)
	member := fmt.Sprintf("%d:%s", now.UnixNano(), clientID)
	eventKey := fmt.Sprintf("admission:event:%s", eventID)
	clientKey := fmt.Sprintf("admission:event:%s:client:%s", eventID, clientID)

	result, err := eventAdmissionScript.Run(
		ctx,
		rl.redis,
		[]string{eventKey, clientKey},
		windowStart.UnixNano(),
		now.UnixNano(),
		member,
		int(window.Seconds()),
		maxEventRequests,
		maxClientRequests,
	).Int()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}

func (rl *RateLimiter) allowLocalEvent(eventID string, maxEventRequests int, window time.Duration) bool {
	if maxEventRequests <= 0 {
		return true
	}

	rl.eventLimitersMu.Lock()
	defer rl.eventLimitersMu.Unlock()

	limiter, ok := rl.eventLimiters[eventID]
	if !ok {
		refillRate := rate.Every(window / time.Duration(maxEventRequests))
		limiter = rate.NewLimiter(refillRate, maxEventRequests)
		rl.eventLimiters[eventID] = limiter
	}

	return limiter.Allow()
}

func (rl *RateLimiter) SimpleRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if rl.redis != nil {
			// Higher limit (1000/min) to support load testing while still providing protection
			allowed, err := rl.checkRedisSimpleLimit(c.Request.Context(), clientIP, 1000, time.Minute)
			if err == nil {
				if !allowed {
					c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
					c.Abort()
					return
				}
				c.Next()
				return
			}
		}

		if !rl.limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (rl *RateLimiter) checkRedisSimpleLimit(ctx context.Context, key string, maxRequests int, window time.Duration) (bool, error) {
	redisKey := fmt.Sprintf("ratelimit:simple:%s", key)

	count, err := rl.redis.Get(ctx, redisKey).Int()
	if err != nil && err != redis.Nil {
		return false, err
	}

	if count >= maxRequests {
		return false, nil
	}

	pipe := rl.redis.Pipeline()
	pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, window)
	_, err = pipe.Exec(ctx)

	return err == nil, err
}

func readEventID(c *gin.Context) (uuid.UUID, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return uuid.Nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var req admissionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return uuid.Nil, err
	}
	if req.EventID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("missing event_id")
	}

	return req.EventID, nil
}

func writeAdmissionRejected(c *gin.Context) {
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error":   "event admission limit reached",
		"message": "This event is receiving high demand. Please retry shortly.",
	})
	c.Abort()
}

var eventAdmissionScript = redis.NewScript(`
local event_key = KEYS[1]
local client_key = KEYS[2]
local window_start = ARGV[1]
local now = ARGV[2]
local member = ARGV[3]
local ttl_seconds = tonumber(ARGV[4])
local event_limit = tonumber(ARGV[5])
local client_limit = tonumber(ARGV[6])

redis.call("ZREMRANGEBYSCORE", event_key, 0, window_start)
redis.call("ZREMRANGEBYSCORE", client_key, 0, window_start)

local event_count = redis.call("ZCARD", event_key)
local client_count = redis.call("ZCARD", client_key)

if event_count >= event_limit or client_count >= client_limit then
	return 0
end

redis.call("ZADD", event_key, now, member)
redis.call("ZADD", client_key, now, member)
redis.call("EXPIRE", event_key, ttl_seconds)
redis.call("EXPIRE", client_key, ttl_seconds)

return 1
`)
