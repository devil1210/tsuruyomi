package websocket

import (
	"sync"

	"github.com/gofiber/websocket/v2"
)

// Client represents a connected websocket client.
type Client struct {
	Conn *websocket.Conn
	Send chan []byte
}

// Hub manages active connections and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

// GlobalHub is the default shared Hub for application-wide broadcasts.
var GlobalHub = NewHub()

func init() {
	go GlobalHub.Run()
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 64),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
	}
}

// Run starts the event loop for the Hub.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast sends a message payload to all registered clients without blocking.
func (h *Hub) Broadcast(message []byte) {
	if h == nil || h.broadcast == nil {
		return
	}
	select {
	case h.broadcast <- message:
	default:
	}
}

// AddClient registers a new websocket client connection with the hub.
func (h *Hub) AddClient(conn *websocket.Conn) *Client {
	client := &Client{
		Conn: conn,
		Send: make(chan []byte, 256),
	}
	h.register <- client

	// Start writer goroutine
	go func() {
		defer func() {
			h.unregister <- client
			conn.Close()
		}()
		for message := range client.Send {
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				break
			}
		}
	}()

	return client
}
