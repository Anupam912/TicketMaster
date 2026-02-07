package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	ID       uuid.UUID
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *Hub
	EventIDs map[uuid.UUID]bool
}

type Hub struct {
	Clients map[*Client]bool
	Broadcast chan *Message
	Register chan *Client
	Unregister chan *Client
	mu sync.RWMutex
}

type Message struct {
	Type    string      `json:"type"`
	EventID uuid.UUID   `json:"event_id,omitempty"`
	SeatID  uuid.UUID   `json:"seat_id,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan *Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			h.mu.Unlock()
			log.Printf("Client connected: %s (Total: %d)", client.ID, len(h.Clients))

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("Client disconnected: %s (Total: %d)", client.ID, len(h.Clients))

		case message := <-h.Broadcast:
			h.mu.RLock()
			for client := range h.Clients {
				if message.EventID != uuid.Nil {
					if client.EventIDs[message.EventID] {
						select {
						case client.Send <- h.marshalMessage(message):
						default:
							close(client.Send)
							delete(h.Clients, client)
						}
					}
				} else {
					select {
					case client.Send <- h.marshalMessage(message):
					default:
						close(client.Send)
						delete(h.Clients, client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) marshalMessage(msg *Message) []byte {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return []byte(`{"type":"error","message":"failed to marshal message"}`)
	}
	return data
}

func (h *Hub) BroadcastSeatUpdate(eventID, seatID uuid.UUID, status string) {
	message := &Message{
		Type:    "seat_update",
		EventID: eventID,
		SeatID:  seatID,
		Data: map[string]interface{}{
			"seat_id": seatID.String(),
			"status":  status,
		},
	}
	h.Broadcast <- message
}

func (h *Hub) HandleClient(conn *websocket.Conn) *Client {
	client := &Client{
		ID:       uuid.New(),
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      h,
		EventIDs: make(map[uuid.UUID]bool),
	}

	client.Hub.Register <- client
	go client.writePump()
	go client.readPump()

	return client
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			if msgType, ok := msg["type"].(string); ok && msgType == "subscribe" {
				if eventIDStr, ok := msg["event_id"].(string); ok {
					if eventID, err := uuid.Parse(eventIDStr); err == nil {
						c.EventIDs[eventID] = true
						log.Printf("Client %s subscribed to event %s", c.ID, eventID)
					}
				}
			}
		}
	}
}

func (c *Client) writePump() {
	defer c.Conn.Close()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}
}
