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
	member := fmt.Sprintf("%d", now.UnixNano())

	result, err := slidingWindowRateLimitScript.Run(
		ctx,
		rl.redis,
		[]string{redisKey},
		windowStart.Unix(),
		now.Unix(),
		member,
		maxRequests,
		int(window.Seconds()),
	).Int()
	if err != nil {
		return false, err
	}

	return result == 1, nil
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

	eventCounterKey := fmt.Sprintf("admission:event:%s:count", eventID)
	clientCounterKey := fmt.Sprintf("admission:event:%s:client:%s:count", eventID, clientID)

	result, err := eventAdmissionScript.Run(
		ctx,
		rl.redis,
		[]string{eventCounterKey, clientCounterKey},
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

	result, err := fixedWindowRateLimitScript.Run(
		ctx,
		rl.redis,
		[]string{redisKey},
		maxRequests,
		int(window.Seconds()),
	).Int()
	if err != nil {
		return false, err
	}

	return result == 1, nil
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

var slidingWindowRateLimitScript = redis.NewScript(`
local key = KEYS[1]
local window_start = ARGV[1]
local now = ARGV[2]
local member = ARGV[3]
local max_requests = tonumber(ARGV[4])
local ttl_seconds = tonumber(ARGV[5])

redis.call("ZREMRANGEBYSCORE", key, "0", window_start)
local count = redis.call("ZCOUNT", key, window_start, now)

if count >= max_requests then
	redis.call("EXPIRE", key, ttl_seconds)
	return 0
end

redis.call("ZADD", key, now, member)
redis.call("EXPIRE", key, ttl_seconds)
return 1
`)

var fixedWindowRateLimitScript = redis.NewScript(`
local key = KEYS[1]
local max_requests = tonumber(ARGV[1])
local ttl_seconds = tonumber(ARGV[2])

local count = redis.call("INCR", key)
if count == 1 or redis.call("TTL", key) < 0 then
	redis.call("EXPIRE", key, ttl_seconds)
end

if count > max_requests then
	redis.call("EXPIRE", key, ttl_seconds)
	return 0
end
return 1
`)

var eventAdmissionScript = redis.NewScript(`
local event_counter = KEYS[1]
local client_counter = KEYS[2]
local ttl_seconds = tonumber(ARGV[1])
local event_limit = tonumber(ARGV[2])
local client_limit = tonumber(ARGV[3])

local function incrWithTTL(key)
	local count = redis.call("INCR", key)
	if count == 1 or redis.call("TTL", key) < 0 then
		redis.call("EXPIRE", key, ttl_seconds)
	end
	return count
end

local event_count = incrWithTTL(event_counter)
if event_count > event_limit then
	redis.call("DECR", event_counter)
	redis.call("EXPIRE", event_counter, ttl_seconds)
	return 0
end

local client_count = incrWithTTL(client_counter)
if client_count > client_limit then
	redis.call("DECR", event_counter)
	redis.call("DECR", client_counter)
	redis.call("EXPIRE", event_counter, ttl_seconds)
	redis.call("EXPIRE", client_counter, ttl_seconds)
	return 0
end

return 1
`)
