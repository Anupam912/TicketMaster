package handlers

import (
	"net/http"

	"event-ticketing-system/internal/websocket"

	"github.com/gin-gonic/gin"
)

// WebSocketHandler handles WebSocket connections.
type WebSocketHandler struct {
	hub *websocket.Hub
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(hub *websocket.Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
	}
}

// HandleWebSocket upgrades HTTP connection to WebSocket.
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	conn, err := h.hub.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to upgrade connection"})
		return
	}

	h.hub.HandleClient(conn)
}
