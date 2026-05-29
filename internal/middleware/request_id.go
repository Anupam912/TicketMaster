package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const requestIDKey = "request_id"

// RequestID injects a request ID in context and response headers.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}

		c.Set(requestIDKey, reqID)
		c.Header("X-Request-ID", reqID)
		c.Next()
	}
}

// GetRequestID returns the request ID from the context.
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if reqID, ok := v.(string); ok {
			return reqID
		}
	}
	return ""
}
