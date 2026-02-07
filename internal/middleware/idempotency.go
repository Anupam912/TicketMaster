package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"event-ticketing-system/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type IdempotencyMiddleware struct {
	redis  *redis.Client
	config *config.Config
}

func NewIdempotencyMiddleware(redis *redis.Client, cfg *config.Config) *IdempotencyMiddleware {
	return &IdempotencyMiddleware{
		redis:  redis,
		config: cfg,
	}
}

func (m *IdempotencyMiddleware) IdempotencyKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost && 
		   c.Request.Method != http.MethodPut && 
		   c.Request.Method != http.MethodPatch {
			c.Next()
			return
		}

		idempotencyKey := c.GetHeader("Idempotency-Key")
		if idempotencyKey == "" {
			c.Next()
			return
		}

		cacheKey := fmt.Sprintf("idempotency:%s", idempotencyKey)
		
		if m.redis != nil {
			cachedResponse, err := m.redis.Get(c.Request.Context(), cacheKey).Result()
			if err == nil {
				var response CachedResponse
				if err := json.Unmarshal([]byte(cachedResponse), &response); err == nil {
					c.Status(response.StatusCode)
					
					for k, v := range response.Headers {
						c.Header(k, v)
					}
					
					c.Writer.Write(response.Body)
					c.Abort()
					return
				}
			}
		}

		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		responseWriter := &responseRecorder{
			ResponseWriter: c.Writer,
			body:          &bytes.Buffer{},
		}

		c.Writer = responseWriter
		c.Next()

		if responseWriter.statusCode >= 200 && responseWriter.statusCode < 300 {
			if m.redis != nil {
				cachedResponse := CachedResponse{
					StatusCode: responseWriter.statusCode,
					Headers:    make(map[string]string),
					Body:       responseWriter.body.Bytes(),
				}

				for k, v := range c.Writer.Header() {
					if len(v) > 0 {
						cachedResponse.Headers[k] = v[0]
					}
				}

				data, err := json.Marshal(cachedResponse)
				if err == nil {
					m.redis.Set(c.Request.Context(), cacheKey, data, 24*time.Hour)
				}
			}
		}
	}
}

type CachedResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
}

type responseRecorder struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
