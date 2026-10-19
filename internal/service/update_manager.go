package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	fiberWS "github.com/gofiber/websocket/v2"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
	"tsuruyomi/internal/sources"
	"tsuruyomi/internal/websocket"
)

// UpdateManager controls library update processes, manages the periodic schedule,
// and broadcasts updates state via WebSockets and GraphQL.
type UpdateManager struct {
	mu                     sync.RWMutex
	running                bool
	totalJobs              int
	finishedJobs           int
	skippedCategoriesCount int
	skippedMangasCount     int
	lastUpdateTimestamp    int64
	hub                    *websocket.Hub
	hubStarted             bool
	cancelFunc             context.CancelFunc
	schedulerStop          chan struct{}
	currentInterval        float64
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

	// Send initial status immediately on connect
	statusBytes, err := json.Marshal(um.GetStatusGraphQL())
	if err == nil {
		client.Send <- statusBytes
	}
}

// IsRunning returns true if an update is actively running.
func (um *UpdateManager) IsRunning() bool {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.running
}

// GetStatusGraphQL returns the update status formatted for GraphQL LibraryUpdateStatus.
func (um *UpdateManager) GetStatusGraphQL() map[string]interface{} {
	um.mu.RLock()
	defer um.mu.RUnlock()

	return map[string]interface{}{
		"jobsInfo": map[string]interface{}{
			"isRunning":              um.running,
			"totalJobs":              um.totalJobs,
			"finishedJobs":           um.finishedJobs,
			"skippedCategoriesCount": um.skippedCategoriesCount,
			"skippedMangasCount":     um.skippedMangasCount,
			"__typename":             "UpdaterJobsInfoType",
		},
		"categoryUpdates": []interface{}{},
		"mangaUpdates":    []interface{}{},
		"__typename":      "LibraryUpdateStatus",
	}
}

// GetLastUpdateTimestamp returns the UNIX timestamp of the last library update.
func (um *UpdateManager) GetLastUpdateTimestamp() int64 {
	um.mu.RLock()
	ts := um.lastUpdateTimestamp
	um.mu.RUnlock()

	if ts > 0 {
		return ts
	}

	if database.DB != nil {
		var maxFetched sql.NullInt64
		_ = database.DB.QueryRow(context.Background(), "SELECT MAX(chapters_last_fetched_at) FROM suwayomi.manga WHERE in_library = true").Scan(&maxFetched)
		if maxFetched.Valid && maxFetched.Int64 > 0 {
			return maxFetched.Int64
		}
	}
	return time.Now().Unix()
}

// BroadcastState broadcasts current status to all WebSocket clients.
func (um *UpdateManager) BroadcastState() {
	if um.hub == nil {
		return
	}
	status := um.GetStatusGraphQL()
	stateBytes, err := json.Marshal(status)
	if err == nil {
		um.hub.Broadcast(stateBytes)
	}
}

func (um *UpdateManager) broadcastProgress(mangaID int, title, state string, progress float32) {
	if um.hub == nil {
		return
	}
	msg, err := json.Marshal(map[string]interface{}{
		"mangaId":  mangaID,
		"progress": progress,
		"state":    state,
		"title":    title,
	})
	if err == nil {
		um.hub.Broadcast(msg)
	}
}

type mangaToUpdate struct {
	ID          int
	SourceID    int64
	URL         string
	Title       string
	Status      int
	UnreadCount int
	ReadCount   int
	Categories  []int64
}

// StartUpdate initiates a full library update in the background.
func (um *UpdateManager) StartUpdate() {
	um.mu.Lock()
	if um.running {
		um.mu.Unlock()
		log.Printf("[UPDATER] Update already running, skipping request")
		return
	}
	um.running = true
	um.totalJobs = 0
	um.finishedJobs = 0
	um.skippedCategoriesCount = 0
	um.skippedMangasCount = 0

	ctx, cancel := context.WithCancel(context.Background())
	um.cancelFunc = cancel
	um.mu.Unlock()

	um.BroadcastState()

	go func() {
		defer func() {
			um.mu.Lock()
			um.running = false
			um.lastUpdateTimestamp = time.Now().Unix()
			if um.cancelFunc != nil {
				um.cancelFunc()
				um.cancelFunc = nil
			}
			finished := um.finishedJobs
			total := um.totalJobs
			skipped := um.skippedMangasCount
			um.mu.Unlock()

			um.BroadcastState()
			log.Printf("[UPDATER] Library update completed. Processed %d/%d (skipped: %d).", finished, total, skipped)
		}()

		if database.DB == nil {
			log.Printf("[UPDATER] No database connection available for library update.")
			return
		}

		// 1. Fetch server settings to apply update filters
		settings, err := database.GetAllServerSettings(ctx)
		if err != nil || settings == nil {
			settings = make(map[string]string)
		}

		excludeUnread := parseBoolSetting(settings["excludeUnreadChapters"], true)
		excludeNotStarted := parseBoolSetting(settings["excludeNotStarted"], true)
		excludeCompleted := parseBoolSetting(settings["excludeCompleted"], true)
		autoDownload := parseBoolSetting(settings["autoDownloadNewChapters"], false)
		autoDownloadLimit := parseIntSetting(settings["autoDownloadNewChaptersLimit"], 1)

		// 2. Fetch category inclusion/exclusion rules
		cats, _ := database.GetCategoryList(ctx)
		includedCats := make(map[int]bool)
		excludedCats := make(map[int]bool)
		for _, c := range cats {
			if c.IncludeInUpdate == models.Include {
				includedCats[c.ID] = true
			} else if c.IncludeInUpdate == models.Exclude {
				excludedCats[c.ID] = true
			}
		}

		// 3. Query all library mangas with stats and categories
		query := `
			SELECT 
				m.id, 
				m.source, 
				m.url, 
				m.title, 
				COALESCE(m.status, 0),
				COALESCE(stat.unread_count, 0), 
				COALESCE(stat.read_count, 0),
				COALESCE(ARRAY_AGG(cm.category) FILTER (WHERE cm.category IS NOT NULL), '{}') as categories
			FROM suwayomi.manga m
			LEFT JOIN suwayomi.categorymanga cm ON m.id = cm.manga AND cm.user_id = 1
			LEFT JOIN (
				SELECT 
					c.manga,
					COUNT(CASE WHEN cu.read = false OR cu.read IS NULL THEN 1 END) as unread_count,
					COUNT(CASE WHEN cu.read = true THEN 1 END) as read_count
				FROM suwayomi.chapter c
				LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter AND cu.user_id = 1
				GROUP BY c.manga
			) stat ON m.id = stat.manga
			WHERE m.in_library = true
			GROUP BY m.id, m.source, m.url, m.title, m.status, stat.unread_count, stat.read_count
			ORDER BY m.id ASC
		`

		rows, err := database.DB.Query(ctx, query)
		if err != nil {
			log.Printf("[UPDATER] Error querying library mangas: %v", err)
			return
		}
		defer rows.Close()

		var library []mangaToUpdate
		for rows.Next() {
			var m mangaToUpdate
			var catArr []int64
			if err := rows.Scan(&m.ID, &m.SourceID, &m.URL, &m.Title, &m.Status, &m.UnreadCount, &m.ReadCount, &catArr); err == nil {
				m.Categories = catArr
				library = append(library, m)
			}
		}

		// 4. Filter mangas according to library settings and native Go source availability
		var eligibleMangas []mangaToUpdate
		for _, m := range library {
			// Only update native Go sources (discard non-native APK extensions)
			if !sources.Has(m.SourceID) {
				um.mu.Lock()
				um.skippedMangasCount++
				um.mu.Unlock()
				continue
			}

			// Check Category Filters
			isExcluded := false
			hasIncluded := false
			if len(m.Categories) == 0 {
				if excludedCats[0] {
					isExcluded = true
				}
				if includedCats[0] {
					hasIncluded = true
				}
			} else {
				for _, catID := range m.Categories {
					if excludedCats[int(catID)] {
						isExcluded = true
					}
					if includedCats[int(catID)] {
						hasIncluded = true
					}
				}
			}

			if isExcluded {
				um.mu.Lock()
				um.skippedMangasCount++
				um.mu.Unlock()
				continue
			}

			if len(includedCats) > 0 && !hasIncluded {
				um.mu.Lock()
				um.skippedMangasCount++
				um.mu.Unlock()
				continue
			}

			// Check Manga Status / Reading Filters
			if excludeCompleted && (m.Status == 2 || m.Status == 3) { // 2: Completed, 3: Licensed/Finished
				um.mu.Lock()
				um.skippedMangasCount++
				um.mu.Unlock()
				continue
			}

			if excludeUnread && m.UnreadCount > 0 {
				um.mu.Lock()
				um.skippedMangasCount++
				um.mu.Unlock()
				continue
			}

			if excludeNotStarted && m.ReadCount == 0 {
				um.mu.Lock()
				um.skippedMangasCount++
				um.mu.Unlock()
				continue
			}

			eligibleMangas = append(eligibleMangas, m)
		}

		total := len(eligibleMangas)
		um.mu.Lock()
		um.totalJobs = total
		um.mu.Unlock()
		um.BroadcastState()

		log.Printf("[UPDATER] Found %d library mangas. %d eligible for update, %d skipped by settings/source availability.", len(library), total, len(library)-total)

		if total == 0 {
			return
		}

		// 5. Sequentially update each eligible manga with polite interval
		for _, m := range eligibleMangas {
			select {
			case <-ctx.Done():
				log.Printf("[UPDATER] Library update canceled by user")
				return
			default:
			}

			um.mu.RLock()
			currentFinished := um.finishedJobs
			currentTotal := max(um.totalJobs, 1)
			um.mu.RUnlock()

			prog := float32(currentFinished) / float32(currentTotal)
			um.broadcastProgress(m.ID, m.Title, "Updating", prog)

			var countBefore int
			_ = database.DB.QueryRow(ctx, "SELECT COUNT(*) FROM suwayomi.chapter WHERE manga = $1", m.ID).Scan(&countBefore)

			log.Printf("[UPDATER] [%d/%d] Fetching chapters for %q (manga %d, source %d)...", currentFinished+1, currentTotal, m.Title, m.ID, m.SourceID)
			_, fetchErr := sources.FetchAndSaveMangaChapters(ctx, m.SourceID, int64(m.ID), m.URL)
			if fetchErr != nil {
				log.Printf("[UPDATER] Warning: failed to fetch chapters for %q: %v", m.Title, fetchErr)
			} else {
				var countAfter int
				_ = database.DB.QueryRow(ctx, "SELECT COUNT(*) FROM suwayomi.chapter WHERE manga = $1", m.ID).Scan(&countAfter)
				if countAfter > countBefore {
					newChapters := countAfter - countBefore
					log.Printf("[UPDATER] New chapters found for %q: +%d chapters! (Total: %d)", m.Title, newChapters, countAfter)

					// Auto-download new chapters if enabled
					if autoDownload && newChapters > 0 {
						limit := min(newChapters, autoDownloadLimit)
						chRows, err := database.DB.Query(ctx, "SELECT id FROM suwayomi.chapter WHERE manga = $1 ORDER BY source_order DESC LIMIT $2", m.ID, limit)
						if err == nil {
							var chIDs []int
							for chRows.Next() {
								var cid int
								if err := chRows.Scan(&cid); err == nil {
									chIDs = append(chIDs, cid)
								}
							}
							chRows.Close()
							if len(chIDs) > 0 {
								_ = GlobalDownloadManager.Enqueue(ctx, chIDs)
								log.Printf("[UPDATER] Auto-download: Enqueued %d new chapter(s) for %q", len(chIDs), m.Title)
							}
						}
					}
				}
			}

			um.mu.Lock()
			um.finishedJobs++
			um.mu.Unlock()
			um.BroadcastState()

			// Polite delay between mangas (500ms) to respect source servers
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
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

// StartScheduler initializes the background periodic updater.
func (um *UpdateManager) StartScheduler(ctx context.Context) {
	um.mu.Lock()
	defer um.mu.Unlock()

	hours := 12.0
	if dbSettings, err := database.GetAllServerSettings(ctx); err == nil {
		if v, ok := dbSettings["globalUpdateInterval"]; ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				hours = f
			}
		}
	}
	um.currentInterval = hours
	um.restartSchedulerLocked(ctx)
}

// SetInterval dynamically updates the background update interval.
func (um *UpdateManager) SetInterval(hours float64) {
	um.mu.Lock()
	defer um.mu.Unlock()

	if um.currentInterval == hours {
		return
	}
	um.currentInterval = hours
	um.restartSchedulerLocked(context.Background())
}

func (um *UpdateManager) restartSchedulerLocked(ctx context.Context) {
	if um.schedulerStop != nil {
		close(um.schedulerStop)
		um.schedulerStop = nil
	}

	if um.currentInterval <= 0 {
		log.Printf("[UPDATER] Background periodic library update scheduler disabled (interval <= 0)")
		return
	}

	duration := time.Duration(um.currentInterval * float64(time.Hour))
	log.Printf("[UPDATER] Background periodic library update scheduler started (every %.1f hours / %v)", um.currentInterval, duration)

	stopChan := make(chan struct{})
	um.schedulerStop = stopChan

	go func(d time.Duration, stop chan struct{}) {
		ticker := time.NewTicker(d)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				log.Printf("[UPDATER] Periodic library update triggered by scheduler")
				um.StartUpdate()
			}
		}
	}(duration, stopChan)
}

func parseBoolSetting(val string, fallback bool) bool {
	if val == "" {
		return fallback
	}
	return strings.ToLower(val) == "true" || val == "1"
}

func parseIntSetting(val string, fallback int) int {
	if val == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscan(val, &n); err == nil {
		return n
	}
	return fallback
}
