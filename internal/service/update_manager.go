package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	fiberWS "github.com/gofiber/websocket/v2"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/websocket"
)

// UpdateManager controls library update processes and broadcasts updates state via WebSockets.
type UpdateManager struct {
	mu         sync.Mutex
	running    bool
	hub        *websocket.Hub
	hubStarted bool
	cancelFunc context.CancelFunc
}

// GlobalUpdateManager is the shared update manager instance.
var GlobalUpdateManager = &UpdateManager{
	running: false,
	hub:     websocket.NewHub(),
}

// StartHub starts the updates WebSocket hub event loop.
func (um *UpdateManager) StartHub() {
	um.mu.Lock()
	defer um.mu.Unlock()
	if !um.hubStarted {
		go um.hub.Run()
		um.hubStarted = true
	}
}

// AddClient registers a new WebSocket client for updates events.
func (um *UpdateManager) AddClient(conn *fiberWS.Conn) {
	um.StartHub()
	client := um.hub.AddClient(conn)
	// Send initial summary status
	statusBytes, err := json.Marshal(map[string]interface{}{
		"running": um.IsRunning(),
	})
	if err == nil {
		client.Send <- statusBytes
	}
}

// IsRunning returns true if an update is actively running.
func (um *UpdateManager) IsRunning() bool {
	um.mu.Lock()
	defer um.mu.Unlock()
	return um.running
}

// StartUpdate initiates a background scanning task.
func (um *UpdateManager) StartUpdate() {
	um.mu.Lock()
	if um.running {
		um.mu.Unlock()
		return
	}
	um.running = true
	ctx, cancel := context.WithCancel(context.Background())
	um.cancelFunc = cancel
	um.mu.Unlock()

	um.BroadcastState()

	// Spawn background scanning routine
	go func() {
		defer func() {
			um.mu.Lock()
			um.running = false
			um.mu.Unlock()
			um.BroadcastState()
		}()

		// Find mangas in library to update
		if database.DB == nil {
			time.Sleep(1 * time.Second) // Mock wait
			return
		}

		rows, err := database.DB.Query(ctx, "SELECT id, title FROM suwayomi.manga WHERE in_library = true")
		if err != nil {
			return
		}
		defer rows.Close()

		type mangaInfo struct {
			ID    int
			Title string
		}
		var library []mangaInfo
		for rows.Next() {
			var m mangaInfo
			if err := rows.Scan(&m.ID, &m.Title); err == nil {
				library = append(library, m)
			}
		}

		total := len(library)
		if total == 0 {
			time.Sleep(1 * time.Second)
			return
		}

		for idx, m := range library {
			select {
			case <-ctx.Done():
				return
			default:
				// Emulate scanning process delay
				time.Sleep(200 * time.Millisecond)

				// Broadcast progress for current manga
				progress := float32(idx+1) / float32(total)
				progressBytes, err := json.Marshal(map[string]interface{}{
					"mangaId":  m.ID,
					"progress": progress,
					"state":    "Scanning",
					"title":    m.Title,
				})
				if err == nil {
					um.hub.Broadcast(progressBytes)
				}
			}
		}
	}()
}

// CancelUpdate aborts any currently running updates task.
func (um *UpdateManager) CancelUpdate() {
	um.mu.Lock()
	if um.cancelFunc != nil {
		um.cancelFunc()
		um.cancelFunc = nil
	}
	um.running = false
	um.mu.Unlock()
	um.BroadcastState()
}

// BroadcastState broadcasts current runner state.
func (um *UpdateManager) BroadcastState() {
	stateBytes, err := json.Marshal(map[string]interface{}{
		"running": um.IsRunning(),
	})
	if err == nil {
		um.hub.Broadcast(stateBytes)
	}
}
