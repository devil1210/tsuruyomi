# WebSocket Connection Hub (`internal/websocket`)

## Overview
The `internal/websocket` package implements an in-memory WebSocket connection manager and broadcast message hub based on `github.com/gofiber/websocket/v2`. It handles concurrent client socket registration, event-driven message distribution, non-blocking broadcasts with automatic slow-client disconnection, and automatically runs a singleton global hub instance (`GlobalHub`).

## Interface
- `[hub.go:Client]`: Active client connection wrapper holding socket connection (`Conn`) and buffered outgoing channel (`Send`).
- `[hub.go:Hub]`: Central connection registry and broadcast event coordinator.
- `[hub.go:GlobalHub]`: Package-level singleton instance of `*Hub`.
- `[hub.go:NewHub]`: `func NewHub() *Hub` constructor initializing registration, unregistration, and broadcast channels.
- `[hub.go:Hub.Run]`: `func (h *Hub) Run()` event loop processing client registration, disconnection, and broadcasts.
- `[hub.go:Hub.Broadcast]`: `func (h *Hub) Broadcast(message []byte)` non-blocking message dissemination.
- `[hub.go:Hub.AddClient]`: `func (h *Hub) AddClient(conn *websocket.Conn) *Client` registers a new connection and spawns its writer goroutine.

## Invariants
- `[hub.go:Hub.Broadcast]`: Broadcast is a safe no-op if receiver hub or broadcast channel is nil (`h == nil || h.broadcast == nil`).
- `[hub.go:Hub.Broadcast]`: Broadcast operation is strictly non-blocking; drops message via select default if broadcast channel buffer (capacity 64) is full.
- `[hub.go:Hub.Run]`: `Hub.mu` mutex must be held whenever accessing or mutating `Hub.clients` map during registration, unregistration, or broadcast cycles.
- `[hub.go:Hub.Run]`: Slow client eviction invariant: if a client's outgoing channel buffer (capacity 256) is full or blocked during broadcast, the channel is closed and the client is automatically evicted from `Hub.clients`.
- `[hub.go:Hub.AddClient]`: Writer goroutine defer ensures automatic client unregistration and socket closure via `conn.Close()` on loop termination.

## Side Effects
- `[hub.go:init]`: Spawns a background goroutine executing `GlobalHub.Run()` during package initialization.
- `[hub.go:Hub.Run]`: Executes an infinite event loop blocking on channels, acquiring/releasing mutexes, and closing channels.
- `[hub.go:Hub.Broadcast]`: Enqueues message bytes into `Hub.broadcast` channel.
- `[hub.go:Hub.AddClient]`: Writes new client into `Hub.register` channel and launches background socket writer goroutine.

## Verification
- Run: `go test ./...`
