package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"

	"event-ticketing-system/internal/config"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const (
	seatUpdateFanoutChannel = "websocket:seat_updates"

	// Buffered hub channels avoid blocking producers when Run is busy fanning out.
	broadcastQueueSize   = 512
	registerQueueSize    = 64
	unregisterQueueSize  = 64
	clientSendBufferSize = 256
)

// Client represents a WebSocket client.
type Client struct {
	ID       uuid.UUID
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *Hub
	EventIDs map[uuid.UUID]bool
	mu       sync.RWMutex
}

// Hub maintains active WebSocket clients and broadcasts messages.
type Hub struct {
	Clients        map[*Client]bool
	Broadcast      chan *Message
	Register       chan *Client
	Unregister     chan *Client
	mu             sync.RWMutex
	wsConfig       config.WebSocketConfig
	Upgrader       websocket.Upgrader
	redis          *redis.Client
	instanceID     string
	publishCtx     context.Context
}

// Message represents a WebSocket message.
type Message struct {
	Type    string      `json:"type"`
	EventID uuid.UUID   `json:"event_id,omitempty"`
	SeatID  uuid.UUID   `json:"seat_id,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type fanoutMessage struct {
	SourceID string   `json:"source_id"`
	Message  *Message `json:"message"`
}

// NewHub creates a new Hub with configurable CORS.
func NewHub() *Hub {
	return NewHubWithConfig(nil)
}

// NewHubWithConfig creates a new Hub with the given configuration.
func NewHubWithConfig(cfg *config.Config) *Hub {
	h := &Hub{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan *Message, broadcastQueueSize),
		Register:   make(chan *Client, registerQueueSize),
		Unregister: make(chan *Client, unregisterQueueSize),
		instanceID: uuid.New().String(),
	}

	if cfg != nil {
		h.wsConfig = cfg.WebSocket
	}

	h.Upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}

	return h
}

func (h *Hub) SetRedis(redisClient *redis.Client) {
	h.redis = redisClient
}

// SetContext binds a cancellable context used for Redis publish (e.g. app shutdown).
// Call before starting Run or any code that calls BroadcastSeatUpdate.
func (h *Hub) SetContext(ctx context.Context) {
	h.publishCtx = ctx
}

func (h *Hub) StartFanout(ctx context.Context) {
	if h.redis == nil {
		return
	}

	pubsub := h.redis.Subscribe(ctx, seatUpdateFanoutChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var envelope fanoutMessage
			if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
				log.Printf("WebSocket fanout unmarshal error: %v", err)
				continue
			}
			if envelope.SourceID == h.instanceID || envelope.Message == nil {
				continue
			}
			h.broadcastLocal(envelope.Message)
		}
	}
}

// checkOrigin validates the request origin using WebSocketConfig.IsOriginAllowed.
func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	if h.wsConfig.IsOriginAllowed(origin) {
		return true
	}

	log.Printf("WebSocket connection rejected: origin %s not allowed", origin)
	return false
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			total := len(h.Clients)
			h.mu.Unlock()
			log.Printf("Client connected: %s (Total: %d)", client.ID, total)

		case client := <-h.Unregister:
			h.removeClient(client)

		case message := <-h.Broadcast:
			h.deliverToClients(message)
		}
	}
}

// removeClient disconnects a client and closes its send channel.
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.Clients[client]; ok {
		delete(h.Clients, client)
		close(client.Send)
		log.Printf("Client disconnected: %s (Total: %d)", client.ID, len(h.Clients))
	}
}

// deliverToClients fans out a message without holding the hub lock during sends.
// Slow clients are dropped (channel closed) when their send buffer is full.
func (h *Hub) deliverToClients(message *Message) {
	payload := h.marshalMessage(message)

	h.mu.RLock()
	recipients := make([]*Client, 0, len(h.Clients))
	for client := range h.Clients {
		if message.EventID == uuid.Nil || client.subscribedTo(message.EventID) {
			recipients = append(recipients, client)
		}
	}
	h.mu.RUnlock()

	var stale []*Client
	for _, client := range recipients {
		select {
		case client.Send <- payload:
		default:
			stale = append(stale, client)
		}
	}

	if len(stale) == 0 {
		return
	}

	h.mu.Lock()
	for _, client := range stale {
		if _, ok := h.Clients[client]; ok {
			delete(h.Clients, client)
			close(client.Send)
			log.Printf("WebSocket client %s dropped: send buffer full", client.ID)
		}
	}
	h.mu.Unlock()
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
	h.broadcastLocal(message)
	h.publishFanout(message)
}

// broadcastLocal enqueues a message for the hub loop. Never blocks callers:
// if the queue is full, the message is dropped (backpressure).
func (h *Hub) broadcastLocal(message *Message) {
	select {
	case h.Broadcast <- message:
	default:
		log.Printf(
			"WebSocket broadcast queue full (cap=%d), dropping type=%s event_id=%s",
			broadcastQueueSize,
			message.Type,
			message.EventID,
		)
	}
}

func (h *Hub) publishFanout(message *Message) {
	if h.redis == nil {
		return
	}

	payload, err := json.Marshal(fanoutMessage{
		SourceID: h.instanceID,
		Message:  message,
	})
	if err != nil {
		log.Printf("WebSocket fanout marshal error: %v", err)
		return
	}

	ctx := h.publishCtx
	if ctx == nil {
		ctx = context.Background()
	}

	if err := h.redis.Publish(ctx, seatUpdateFanoutChannel, payload).Err(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		log.Printf("WebSocket fanout publish error: %v", err)
	}
}

func (h *Hub) HandleClient(conn *websocket.Conn) *Client {
	client := &Client{
		ID:       uuid.New(),
		Conn:     conn,
		Send:     make(chan []byte, clientSendBufferSize),
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
						c.subscribe(eventID)
						log.Printf("Client %s subscribed to event %s", c.ID, eventID)
					}
				}
			}
		}
	}
}

func (c *Client) subscribe(eventID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.EventIDs[eventID] = true
}

func (c *Client) subscribedTo(eventID uuid.UUID) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.EventIDs[eventID]
}

func (c *Client) writePump() {
	defer c.Conn.Close()

	for message := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}
	}
	// Channel closed, send close message
	c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
}
