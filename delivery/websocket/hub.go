package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Message represents a WebSocket message
type Message struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// Client represents a WebSocket client
type Client struct {
	ID     string
	Conn   *websocket.Conn
	Send   chan []byte
	Hub    *Hub
 UserID string // Optional: for future authentication
}

// Hub maintains active client set and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mutex      sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
	}
}

// Run starts the hub's event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			log.Printf("[WebSocket] Client registered: %s (Total: %d)", client.ID, len(h.clients))

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mutex.Unlock()
			log.Printf("[WebSocket] Client unregistered: %s (Total: %d)", client.ID, len(h.clients))

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
					// Message sent
				default:
					// Client channel is full, close it
					log.Printf("[WebSocket] Client %s send buffer full, closing", client.ID)
					delete(h.clients, client)
					close(client.Send)
				}
			}
			h.mutex.RLock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(messageType string, data map[string]interface{}) {
	message := Message{
		Type: messageType,
		Data: data,
	}

	bytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("[WebSocket] Failed to marshal message: %v", err)
		return
	}

	select {
	case h.broadcast <- bytes:
		log.Printf("[WebSocket] Broadcasted %s message", messageType)
	default:
		log.Printf("[WebSocket] Broadcast buffer full, message dropped")
	}
}

// BroadcastTaskCreated broadcasts a task creation event
func (h *Hub) BroadcastTaskCreated(taskID, taskName string, payload map[string]interface{}) {
	h.Broadcast("task_created", map[string]interface{}{
		"task_id":   taskID,
		"name":       taskName,
		"payload":    payload,
		"status":     "pending",
		"created_at": dataToString(payload["created_at"]),
	})
}

// BroadcastTaskUpdated broadcasts a task update event
func (h *Hub) BroadcastTaskUpdated(taskID, status string) {
	h.Broadcast("task_updated", map[string]interface{}{
		"task_id":    taskID,
		"status":     status,
		"updated_at": dataToString(nil),
	})
}

// BroadcastTaskDeleted broadcasts a task deletion event
func (h *Hub) BroadcastTaskDeleted(taskID string) {
	h.Broadcast("task_deleted", map[string]interface{}{
		"task_id":    taskID,
		"deleted_at": dataToString(nil),
	})
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}

// Upgrader is used to upgrade HTTP to WebSocket
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// HandleWebSocket handles WebSocket connection requests
func (h *Hub) HandleWebSocket(c *gin.Context) {
	conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WebSocket] Failed to upgrade connection: %v", err)
		return
	}

	client := &Client{
		ID:   generateClientID(),
		Conn: conn,
		Send: make(chan []byte, 256),
		Hub:  h,
	}

	// Register client
	h.register <- client

	// Start client goroutines
	go client.readPump()
	go client.writePump()
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WebSocket] Unexpected close error: %v", err)
			}
			break
		}
		log.Printf("[WebSocket] Received message from client %s: %s", c.ID, string(message))
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				return
			}

			err := c.Conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("[WebSocket] Failed to write message: %v", err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return "client_" + randomString(8)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// dataToString converts interface{} to string
func dataToString(data interface{}) string {
	if data == nil {
		return time.Now().Format(time.RFC3339)
	}
	if str, ok := data.(string); ok {
		return str
	}
	return time.Now().Format(time.RFC3339)
}
