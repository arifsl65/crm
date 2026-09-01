// Package websocket provides real-time WebSocket communication.
package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/cache"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// Client represents a WebSocket client connection.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	tenantID uuid.UUID
	userID   uuid.UUID
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	// Registered clients grouped by tenant
	clients map[uuid.UUID]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Redis client for Pub/Sub
	redis *cache.Client

	// Mutex for thread-safe client management
	mu sync.RWMutex

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewHub creates a new Hub instance.
func NewHub(redisClient *cache.Client) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:    make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		redis:      redisClient,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.tenantID] == nil {
				h.clients[client.tenantID] = make(map[*Client]bool)
				// Start Redis subscription for this tenant
				go h.subscribeToTenant(client.tenantID)
			}
			h.clients[client.tenantID][client] = true
			h.mu.Unlock()

			log.Debug().
				Str("tenant_id", client.tenantID.String()).
				Str("user_id", client.userID.String()).
				Msg("WebSocket client registered")

		case client := <-h.unregister:
			h.mu.Lock()
			if tenantClients, ok := h.clients[client.tenantID]; ok {
				if _, ok := tenantClients[client]; ok {
					delete(tenantClients, client)
					close(client.send)

					// Clean up tenant map if empty
					if len(tenantClients) == 0 {
						delete(h.clients, client.tenantID)
					}
				}
			}
			h.mu.Unlock()

			log.Debug().
				Str("tenant_id", client.tenantID.String()).
				Str("user_id", client.userID.String()).
				Msg("WebSocket client unregistered")

		case <-h.ctx.Done():
			return
		}
	}
}

// subscribeToTenant subscribes to Redis events for a tenant.
func (h *Hub) subscribeToTenant(tenantID uuid.UUID) {
	eventChan, cleanup, err := h.redis.Subscribe(h.ctx, tenantID)
	if err != nil {
		log.Error().Err(err).Str("tenant_id", tenantID.String()).Msg("Failed to subscribe to tenant events")
		return
	}
	defer cleanup()

	log.Debug().Str("tenant_id", tenantID.String()).Msg("Started Redis subscription for tenant")

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			h.broadcastToTenant(tenantID, event)

		case <-h.ctx.Done():
			return
		}
	}
}

// broadcastToTenant sends an event to all clients of a tenant.
func (h *Hub) broadcastToTenant(tenantID uuid.UUID, event *cache.Event) {
	h.mu.RLock()
	clients := h.clients[tenantID]
	h.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	message, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal event for broadcast")
		return
	}

	for client := range clients {
		select {
		case client.send <- message:
		default:
			// Client's send channel is full, skip
			log.Warn().
				Str("user_id", client.userID.String()).
				Msg("Client send channel full, skipping message")
		}
	}

	log.Debug().
		Str("tenant_id", tenantID.String()).
		Int("client_count", len(clients)).
		Str("event_type", string(event.Type)).
		Msg("Broadcast event to tenant clients")
}

// BroadcastToUser sends an event to a specific user.
func (h *Hub) BroadcastToUser(tenantID, userID uuid.UUID, event *cache.Event) {
	h.mu.RLock()
	clients := h.clients[tenantID]
	h.mu.RUnlock()

	message, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal event for user broadcast")
		return
	}

	for client := range clients {
		if client.userID == userID {
			select {
			case client.send <- message:
			default:
				log.Warn().Str("user_id", userID.String()).Msg("User's send channel full")
			}
		}
	}
}

// Shutdown gracefully shuts down the hub.
func (h *Hub) Shutdown() {
	h.cancel()

	h.mu.Lock()
	defer h.mu.Unlock()

	for tenantID, clients := range h.clients {
		for client := range clients {
			close(client.send)
		}
		delete(h.clients, tenantID)
	}

	log.Info().Msg("WebSocket hub shut down")
}

// Stats returns hub statistics.
func (h *Hub) Stats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalClients := 0
	tenantStats := make(map[string]int)

	for tenantID, clients := range h.clients {
		count := len(clients)
		totalClients += count
		tenantStats[tenantID.String()] = count
	}

	return map[string]interface{}{
		"total_clients":   totalClients,
		"total_tenants":   len(h.clients),
		"clients_by_tenant": tenantStats,
	}
}

// RegisterClient registers a new client with the hub.
func (h *Hub) RegisterClient(conn *websocket.Conn, tenantID, userID uuid.UUID) *Client {
	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		tenantID: tenantID,
		userID:   userID,
	}
	h.register <- client
	return client
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("WebSocket read error")
			}
			break
		}
		// We don't process incoming messages for now (read-only WebSocket)
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Batch pending messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
