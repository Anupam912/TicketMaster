package handlers

import (
	"net/http"

	"event-ticketing-system/internal/websocket"

	"github.com/gin-gonic/gin"
	gorillaWS "github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	hub *websocket.Hub
	upgrader gorillaWS.Upgrader
}

func NewWebSocketHandler(hub *websocket.Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		upgrader: gorillaWS.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to upgrade connection"})
		return
	}

	h.hub.HandleClient(conn)
}
