package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLog logs per-request latency and metadata with request IDs.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		reqID := GetRequestID(c)
		log.Printf(
			"request_id=%s method=%s path=%s status=%d latency_ms=%d ip=%s",
			reqID,
			c.Request.Method,
			c.FullPath(),
			c.Writer.Status(),
			time.Since(start).Milliseconds(),
			c.ClientIP(),
		)
	}
}
