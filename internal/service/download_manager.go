package service

import (
	"context"
	"encoding/json"
	"sync"

	fiberWS "github.com/gofiber/websocket/v2"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
	"tsuruyomi/internal/websocket"
)

// DownloadManager handles the downloads queue and notifies active WebSocket clients.
type DownloadManager struct {
	mu           sync.Mutex
	status       string // "Started" or "Stopped"
	queue        []models.DownloadQueueItem
	hub          *websocket.Hub
	hubStarted   bool
}

// GlobalDownloadManager is the shared download manager instance.
var GlobalDownloadManager = &DownloadManager{
	status: "Stopped",
	queue:  []models.DownloadQueueItem{},
	hub:    websocket.NewHub(),
}

// StartHub starts the websocket hub event loop.
func (dm *DownloadManager) StartHub() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if !dm.hubStarted {
		go dm.hub.Run()
		dm.hubStarted = true
	}
}

// AddClient registers a new WebSocket connection.
func (dm *DownloadManager) AddClient(conn *fiberWS.Conn) {
	dm.StartHub()
	client := dm.hub.AddClient(conn)
	// Immediately send current status on connect
	statusBytes, err := json.Marshal(dm.GetStatus())
	if err == nil {
		client.Send <- statusBytes
	}
}

// GetStatus returns the current status payload.
func (dm *DownloadManager) GetStatus() models.DownloadStatus {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	return models.DownloadStatus{
		Status: dm.status,
		Queue:  dm.queue,
	}
}

// GetOldStatus converts current queue to DownloadChapter objects (fetching from DB).
func (dm *DownloadManager) GetOldStatus(ctx context.Context) (models.OldDownloadStatus, error) {
	dm.mu.Lock()
	queueCopy := make([]models.DownloadQueueItem, len(dm.queue))
	copy(queueCopy, dm.queue)
	statusVal := dm.status
	dm.mu.Unlock()

	oldQueue := []models.DownloadChapter{}
	for _, item := range queueCopy {
		manga, err := database.GetMangaByID(ctx, item.MangaID)
		if err != nil || manga == nil {
			continue
		}
		chapter, err := database.GetChapterByMangaAndIndex(ctx, item.MangaID, item.ChapterIndex)
		if err != nil || chapter == nil {
			continue
		}

		oldQueue = append(oldQueue, models.DownloadChapter{
			ChapterIndex: item.ChapterIndex,
			MangaID:      item.MangaID,
			Chapter:      *chapter,
			Manga:        *manga,
			Position:     item.Position,
			State:        item.State,
			Progress:     item.Progress,
			Tries:        item.Tries,
		})
	}

	return models.OldDownloadStatus{
		Status: statusVal,
		Queue:  oldQueue,
	}, nil
}

// Start starts queue execution.
func (dm *DownloadManager) Start() {
	dm.mu.Lock()
	dm.status = "Started"
	dm.mu.Unlock()
	dm.BroadcastStatus()
}

// Stop stops/pauses queue execution.
func (dm *DownloadManager) Stop() {
	dm.mu.Lock()
	dm.status = "Stopped"
	dm.mu.Unlock()
	dm.BroadcastStatus()
}

// Clear empties the downloads queue.
func (dm *DownloadManager) Clear() {
	dm.mu.Lock()
	dm.queue = []models.DownloadQueueItem{}
	dm.mu.Unlock()
	dm.BroadcastStatus()
}

// Enqueue adds chapter IDs to the downloads queue.
func (dm *DownloadManager) Enqueue(ctx context.Context, chapterIDs []int) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for _, chID := range chapterIDs {
		// Verify if it is already in queue to avoid duplicates
		exists := false
		for _, item := range dm.queue {
			if item.ChapterID == chID {
				exists = true
				break
			}
		}
		if exists {
			continue
		}

		// Find parent manga ID and index from DB
		var mangaID, index int
		var sourceID int64
		query := `SELECT manga, source_order, (SELECT source FROM suwayomi.manga WHERE id = manga) as src FROM suwayomi.chapter WHERE id = $1 LIMIT 1`
		err := database.DB.QueryRow(ctx, query, chID).Scan(&mangaID, &index, &sourceID)
		if err != nil {
			continue
		}

		dm.queue = append(dm.queue, models.DownloadQueueItem{
			ChapterID:    chID,
			ChapterIndex: index,
			MangaID:      mangaID,
			SourceID:     sourceID,
			Position:     len(dm.queue),
			PageCount:    -1,
			State:        models.DownloadStateQueued,
			Progress:     0,
			Tries:        0,
		})
	}

	go dm.BroadcastStatus()
	return nil
}

// Dequeue removes specific chapter IDs from the downloads queue.
func (dm *DownloadManager) Dequeue(chapterIDs []int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	idMap := make(map[int]bool)
	for _, id := range chapterIDs {
		idMap[id] = true
	}

	newQueue := []models.DownloadQueueItem{}
	for _, item := range dm.queue {
		if !idMap[item.ChapterID] {
			newQueue = append(newQueue, item)
		}
	}

	// Recalculate positions
	for i := range newQueue {
		newQueue[i].Position = i
	}

	dm.queue = newQueue
	go dm.BroadcastStatus()
}

// DequeueByIndex removes an item by its chapter index and manga ID.
func (dm *DownloadManager) DequeueByIndex(ctx context.Context, mangaID, chapterIndex int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	newQueue := []models.DownloadQueueItem{}
	for _, item := range dm.queue {
		if !(item.MangaID == mangaID && item.ChapterIndex == chapterIndex) {
			newQueue = append(newQueue, item)
		}
	}

	// Recalculate positions
	for i := range newQueue {
		newQueue[i].Position = i
	}

	dm.queue = newQueue
	go dm.BroadcastStatus()
}

// Reorder moves an item to another position in queue.
func (dm *DownloadManager) Reorder(mangaID, chapterIndex, to int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	idx := -1
	for i, item := range dm.queue {
		if item.MangaID == mangaID && item.ChapterIndex == chapterIndex {
			idx = i
			break
		}
	}

	if idx == -1 || to < 0 || to >= len(dm.queue) {
		return
	}

	target := dm.queue[idx]
	// Remove from slice
	dm.queue = append(dm.queue[:idx], dm.queue[idx+1:]...)

	// Insert back at 'to' position
	if to == len(dm.queue) {
		dm.queue = append(dm.queue, target)
	} else {
		dm.queue = append(dm.queue[:to], append([]models.DownloadQueueItem{target}, dm.queue[to:]...)...)
	}

	// Recalculate positions
	for i := range dm.queue {
		dm.queue[i].Position = i
	}

	go dm.BroadcastStatus()
}

// BroadcastStatus sends status to all WebSocket clients.
func (dm *DownloadManager) BroadcastStatus() {
	status := dm.GetStatus()
	statusBytes, err := json.Marshal(status)
	if err == nil {
		dm.hub.Broadcast(statusBytes)
	}
}
