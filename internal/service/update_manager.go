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
	baseHour               int
	baseMinute             int
}

// GlobalUpdateManager is the shared update manager instance.
var GlobalUpdateManager = &UpdateManager{
	running:  false,
	hub:      websocket.NewHub(),
	baseHour: 4,
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
		_ = database.DB.QueryRow(context.Background(), `
			SELECT MAX(m.chapters_last_fetched_at) 
			FROM suwayomi.manga m 
			JOIN suwayomi.mangauser mu ON m.id = mu.manga AND mu.user_id = 1 
			WHERE mu.in_library = true
		`).Scan(&maxFetched)
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

		excludeUnread := parseBoolSetting(settings["excludeUnreadChapters"], false)
		excludeNotStarted := parseBoolSetting(settings["excludeNotStarted"], false)
		excludeCompleted := parseBoolSetting(settings["excludeCompleted"], false)
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
			JOIN suwayomi.mangauser mu ON m.id = mu.manga AND mu.user_id = 1 AND mu.in_library = true
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
			// Check if source is registered and supports chapter listing
			handler, hasHandler := sources.Get(m.SourceID)
			if !hasHandler {
				um.mu.Lock()
				um.skippedMangasCount++
				um.mu.Unlock()
				continue
			}
			if _, canList := handler.(sources.ChapterLister); !canList {
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

// CalculateNextWallClockRun calculates the next time an update should run based on the interval and base clock time (baseHour, baseMinute).
func CalculateNextWallClockRun(now time.Time, intervalHours float64, baseHour, baseMinute int) time.Time {
	if intervalHours <= 0 {
		return time.Time{}
	}

	loc := now.Location()
	anchor := time.Date(now.Year(), now.Month(), now.Day(), baseHour, baseMinute, 0, 0, loc)

	if intervalHours >= 24 {
		// Run once daily at baseHour:baseMinute
		if now.Before(anchor) {
			return anchor
		}
		return anchor.AddDate(0, 0, 1)
	}

	// For sub-daily intervals (e.g. 12h, 6h, 8h, 4h, 2h, 1h)
	step := time.Duration(intervalHours * float64(time.Hour))
	if step < time.Minute {
		step = time.Minute
	}

	// Start from anchor 24h in the past and step forward until we strictly pass 'now'
	t := anchor.Add(-24 * time.Hour)
	for !t.After(now) {
		t = t.Add(step)
	}
	return t
}

// StartScheduler initializes the background periodic updater with wall-clock alignment.
func (um *UpdateManager) StartScheduler(ctx context.Context) {
	um.mu.Lock()
	defer um.mu.Unlock()

	hours := 12.0
	baseHour := 4
	baseMinute := 0

	if dbSettings, err := database.GetAllServerSettings(ctx); err == nil {
		if v, ok := dbSettings["globalUpdateInterval"]; ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				hours = f
			}
		}
		if v, ok := dbSettings["globalUpdateTime"]; ok && v != "" {
			parts := strings.Split(strings.TrimSpace(v), ":")
			if len(parts) >= 1 {
				if h, err := strconv.Atoi(parts[0]); err == nil && h >= 0 && h < 24 {
					baseHour = h
				}
			}
			if len(parts) >= 2 {
				if m, err := strconv.Atoi(parts[1]); err == nil && m >= 0 && m < 60 {
					baseMinute = m
				}
			}
		}
	}
	um.currentInterval = hours
	um.baseHour = baseHour
	um.baseMinute = baseMinute
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

// SetBaseTime updates the base wall-clock time for library updates.
func (um *UpdateManager) SetBaseTime(hour, minute int) {
	um.mu.Lock()
	defer um.mu.Unlock()

	if um.baseHour == hour && um.baseMinute == minute {
		return
	}
	um.baseHour = hour
	um.baseMinute = minute
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

	interval := um.currentInterval
	bHour := um.baseHour
	bMinute := um.baseMinute

	log.Printf("[UPDATER] Wall-clock library update scheduler active (every %.1f hours, anchored at %02d:%02d)", interval, bHour, bMinute)

	stopChan := make(chan struct{})
	um.schedulerStop = stopChan

	go func(intervalHours float64, baseH, baseM int, stop chan struct{}) {
		for {
			now := time.Now()
			nextRun := CalculateNextWallClockRun(now, intervalHours, baseH, baseM)
			if nextRun.IsZero() {
				return
			}

			delay := time.Until(nextRun)
			log.Printf("[UPDATER] Next library update scheduled for %s (in %v)",
				nextRun.Format("2006-01-02 15:04:05 MST"), delay.Round(time.Second))

			timer := time.NewTimer(delay)
			select {
			case <-stop:
				timer.Stop()
				return
			case <-timer.C:
				log.Printf("[UPDATER] Wall-clock scheduled library update triggered at %s", time.Now().Format("15:04:05 MST"))
				um.StartUpdate()
				// Sleep briefly to ensure time.Now() moves past the trigger second
				time.Sleep(2 * time.Second)
			}
		}
	}(interval, bHour, bMinute, stopChan)
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
