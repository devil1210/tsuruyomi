package service

import (
	"context"
	"testing"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/models"
	"tsuruyomi/internal/websocket"
)

func TestDownloadManager_LifecycleAndQueue(t *testing.T) {
	dm := &DownloadManager{
		status: "Stopped",
		queue:  []models.DownloadQueueItem{},
		hub:    websocket.NewHub(),
	}

	// 1. Initial status
	st := dm.GetStatus()
	if st.Status != "Stopped" {
		t.Fatalf("expected Stopped, got %s", st.Status)
	}
	if len(st.Queue) != 0 {
		t.Fatalf("expected empty queue")
	}

	// 2. Set config
	cfg := &config.Config{}
	cfg.Data.RootDir = "/tmp/tsuruyomi_test"
	dm.SetConfig(cfg)
	if dm.cfg != cfg {
		t.Fatalf("expected cfg to match")
	}

	// 3. Populate queue manually
	dm.queue = append(dm.queue,
		models.DownloadQueueItem{
			ChapterID:    101,
			ChapterIndex: 1,
			MangaID:      1,
			Position:     0,
			State:        models.DownloadStateQueued,
		},
		models.DownloadQueueItem{
			ChapterID:    102,
			ChapterIndex: 2,
			MangaID:      1,
			Position:     1,
			State:        models.DownloadStateQueued,
		},
		models.DownloadQueueItem{
			ChapterID:    103,
			ChapterIndex: 3,
			MangaID:      1,
			Position:     2,
			State:        models.DownloadStateQueued,
		},
	)

	// 4. Test ReorderByID
	// Move chapter 103 (at index 2) to position 0
	dm.ReorderByID(103, 0)
	if dm.queue[0].ChapterID != 103 {
		t.Fatalf("expected chapter 103 to be at position 0, got %d", dm.queue[0].ChapterID)
	}
	if dm.queue[1].ChapterID != 101 || dm.queue[2].ChapterID != 102 {
		t.Fatalf("queue order incorrect after reorder: %v", dm.queue)
	}

	// 5. Test Dequeue
	dm.Dequeue([]int{101})
	if len(dm.queue) != 2 {
		t.Fatalf("expected queue length 2 after dequeue, got %d", len(dm.queue))
	}
	if dm.queue[0].ChapterID != 103 || dm.queue[1].ChapterID != 102 {
		t.Fatalf("queue contents unexpected: %v", dm.queue)
	}

	// 6. Test Start / Stop
	dm.Stop()
	if dm.status != "Stopped" {
		t.Fatalf("expected status Stopped")
	}

	// 7. Test Clear
	dm.Clear()
	if len(dm.queue) != 0 {
		t.Fatalf("expected queue to be empty after Clear")
	}

	// 8. Test OldStatus on empty queue
	oldSt, err := dm.GetOldStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on GetOldStatus: %v", err)
	}
	if len(oldSt.Queue) != 0 {
		t.Fatalf("expected empty old queue")
	}
}
