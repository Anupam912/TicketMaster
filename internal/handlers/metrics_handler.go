package handlers

import (
	"net/http"

	"event-ticketing-system/internal/telemetry"

	"github.com/gin-gonic/gin"
)

// ServePrometheusMetrics exposes Prometheus text-format metrics.
func ServePrometheusMetrics(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(telemetry.RenderPrometheus()))
}
