package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"event-ticketing-system/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	redis  *redis.Client
	config *config.Config
	// In-memory rate limiter as fallback
	limiter *rate.Limiter
}

func NewRateLimiter(redis *redis.Client, cfg *config.Config) *RateLimiter {
	limiter := rate.NewLimiter(rate.Every(time.Second/10), 10)

	return &RateLimiter{
		redis:   redis,
		config: cfg,
		limiter: limiter,
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
						"error": "too many requests, please wait",
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
				"error": "too many requests, please wait",
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

func (rl *RateLimiter) SimpleRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if rl.redis != nil {
			allowed, err := rl.checkRedisSimpleLimit(c.Request.Context(), clientIP, 100, time.Minute)
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
