package service

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"tsuruyomi/internal/config"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
	"tsuruyomi/internal/websocket"
)

// RestoreStatus represents the live status of an asynchronous backup restoration job.
type RestoreStatus struct {
	ID            string `json:"id"`
	State         string `json:"state"` // PENDING, RUNNING, COMPLETED, FAILED
	MangaProgress int    `json:"mangaProgress"`
	TotalManga    int    `json:"totalManga"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
}

type RestoreManager struct {
	mu          sync.RWMutex
	jobs        map[string]*RestoreStatus
	latestJobID string
}

var GlobalRestoreManager = NewRestoreManager()

func NewRestoreManager() *RestoreManager {
	return &RestoreManager{
		jobs: make(map[string]*RestoreStatus),
	}
}

// StartAsyncRestore begins an asynchronous backup restoration in a background goroutine and returns job ID and total manga count.
func (rm *RestoreManager) StartAsyncRestore(data []byte) (string, int, error) {
	if database.DB == nil {
		return "", 0, fmt.Errorf("database not initialized")
	}
	if len(data) == 0 {
		return "", 0, fmt.Errorf("empty backup payload")
	}

	// 1. Decompress if gzipped (.tachibk / .proto.gz)
	raw := data
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return "", 0, fmt.Errorf("decompressing gzip backup: %w", err)
		}
		defer zr.Close()
		decompressed, err := io.ReadAll(zr)
		if err != nil {
			return "", 0, fmt.Errorf("reading decompressed backup data: %w", err)
		}
		raw = decompressed
	}

	trimmed := bytes.TrimSpace(raw)
	var pb *ProtoBackup
	var err error
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var payload BackupPayload
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return "", 0, fmt.Errorf("decoding legacy backup JSON: %w", err)
		}
		pb = jsonPayloadToProto(&payload)
	} else {
		pb, err = DecodeProtoBackup(raw)
		if err != nil {
			return "", 0, fmt.Errorf("decoding protobuf backup: %w", err)
		}
	}

	totalManga := len(pb.BackupManga)
	jobID := fmt.Sprintf("restore-%d", time.Now().UnixMilli())

	status := &RestoreStatus{
		ID:            jobID,
		State:         "RUNNING",
		MangaProgress: 0,
		TotalManga:    totalManga,
	}

	rm.mu.Lock()
	rm.jobs[jobID] = status
	rm.latestJobID = jobID
	rm.mu.Unlock()

	log.Printf("[BACKUP] Starting async restore %s with %d mangas...", jobID, totalManga)

	// Launch background restoration
	go func() {
		ctx := context.Background()
		errRestore := RestoreFromProtoBackupWithProgress(ctx, pb, func(current int, total int) {
			rm.mu.Lock()
			if st, ok := rm.jobs[jobID]; ok {
				st.MangaProgress = current
				st.TotalManga = total
			}
			rm.mu.Unlock()

			// Broadcast progress via WebSocket if available
			if websocket.GlobalHub != nil {
				msg, _ := json.Marshal(map[string]interface{}{
					"event": "BACKUP_RESTORE_PROGRESS",
					"data": map[string]interface{}{
						"id":            jobID,
						"state":         "RUNNING",
						"mangaProgress": current,
						"totalManga":    total,
					},
				})
				websocket.GlobalHub.Broadcast(msg)
			}
		})

		rm.mu.Lock()
		defer rm.mu.Unlock()
		if st, ok := rm.jobs[jobID]; ok {
			if errRestore != nil {
				st.State = "FAILED"
				st.ErrorMessage = errRestore.Error()
				log.Printf("[BACKUP] Restore %s failed: %v", jobID, errRestore)
			} else {
				st.State = "COMPLETED"
				st.MangaProgress = totalManga
				log.Printf("[BACKUP] Restore %s completed successfully (%d mangas)", jobID, totalManga)
			}
		}

		if websocket.GlobalHub != nil {
			state := "COMPLETED"
			if errRestore != nil {
				state = "FAILED"
			}
			msg, _ := json.Marshal(map[string]interface{}{
				"event": "BACKUP_RESTORE_FINISHED",
				"data": map[string]interface{}{
					"id":            jobID,
					"state":         state,
					"mangaProgress": totalManga,
					"totalManga":    totalManga,
				},
			})
			websocket.GlobalHub.Broadcast(msg)
		}
	}()

	return jobID, totalManga, nil
}

// GetStatus returns the current live status for a restore job ID (or the latest job if empty).
func (rm *RestoreManager) GetStatus(jobID string) RestoreStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if jobID != "" {
		if st, ok := rm.jobs[jobID]; ok {
			return *st
		}
	}
	if rm.latestJobID != "" {
		if st, ok := rm.jobs[rm.latestJobID]; ok {
			return *st
		}
	}
	return RestoreStatus{
		ID:            "1",
		State:         "IDLE",
		MangaProgress: 0,
		TotalManga:    0,
	}
}

// MissingSourceInfo represents a source present in a backup whose extension is not installed.
type MissingSourceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ValidateBackupResult contains the results of validating a backup against installed sources and extensions.
type ValidateBackupResult struct {
	MissingSources  []MissingSourceInfo `json:"missingSources"`
	MissingTrackers []interface{}       `json:"missingTrackers"`
}

// BackupPayload is the legacy backup format structure for exporting and importing JSON data.
type BackupPayload struct {
	Categories    []models.CategoryDataClass `json:"categories"`
	Mangas        []models.MangaDataClass    `json:"mangas"`
	Chapters      []models.ChapterDataClass  `json:"chapters"`
	CategoryManga []CategoryMangaLink        `json:"categoryManga"`
}

// CategoryMangaLink represents a binding between a category and a manga.
type CategoryMangaLink struct {
	CategoryID int `json:"categoryId"`
	MangaID    int `json:"mangaId"`
}

// ExportBackup exports database contents as standard Tachiyomi/Suwayomi .tachibk format (gzipped protobuf).
func ExportBackup(ctx context.Context) ([]byte, error) {
	return ExportTachibk(ctx)
}

// ImportBackup parses the payload and restores categories, library mangas, and chapters history.
// It supports standard Tachiyomi/Suwayomi Protobuf (.tachibk / .proto.gz) as well as legacy JSON backups.
func ImportBackup(ctx context.Context, data []byte) error {
	if database.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if len(data) == 0 {
		return fmt.Errorf("empty backup payload")
	}

	// 1. Decompress if gzipped (.tachibk / .proto.gz)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("decompressing gzip backup: %w", err)
		}
		defer zr.Close()
		decompressed, err := io.ReadAll(zr)
		if err != nil {
			return fmt.Errorf("reading decompressed backup data: %w", err)
		}
		data = decompressed
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var payload BackupPayload
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return fmt.Errorf("decoding legacy backup JSON: %w", err)
		}
		pb := jsonPayloadToProto(&payload)
		return RestoreFromProtoBackup(ctx, pb)
	}

	// 2. Decode standard Tachiyomi/Suwayomi Protobuf wire bytes
	pb, err := DecodeProtoBackup(data)
	if err != nil {
		return fmt.Errorf("decoding protobuf backup: %w", err)
	}

	return RestoreFromProtoBackup(ctx, pb)
}

func jsonPayloadToProto(p *BackupPayload) *ProtoBackup {
	pb := &ProtoBackup{}
	for _, c := range p.Categories {
		pb.BackupCategories = append(pb.BackupCategories, ProtoBackupCategory{
			Name:  c.Name,
			Order: int64(c.Order),
			Flags: 0,
		})
	}

	for _, m := range p.Mangas {
		sourceID, _ := strconv.ParseInt(m.SourceID, 10, 64)
		var cats []int64
		for _, link := range p.CategoryManga {
			if link.MangaID == m.ID {
				cats = append(cats, int64(link.CategoryID))
			}
		}

		var chs []ProtoBackupChapter
		var history []ProtoBackupHistory
		for _, ch := range p.Chapters {
			if ch.MangaID == m.ID {
				scan := ""
				if ch.Scanlator != nil {
					scan = *ch.Scanlator
				}
				chs = append(chs, ProtoBackupChapter{
					URL:           ch.URL,
					Name:          ch.Name,
					Scanlator:     scan,
					Read:          ch.Read,
					Bookmark:      ch.Bookmarked,
					LastPageRead:  int64(ch.LastPageRead),
					DateUpload:    ch.UploadDate,
					ChapterNumber: float32(ch.ChapterNumber),
					SourceOrder:   int64(ch.Index),
				})
				if ch.LastReadAt > 0 {
					history = append(history, ProtoBackupHistory{
						URL:      ch.URL,
						LastRead: ch.LastReadAt,
					})
				}
			}
		}

		thumb := ""
		if m.ThumbnailURL != nil {
			thumb = *m.ThumbnailURL
		}
		artist := ""
		if m.Artist != nil {
			artist = *m.Artist
		}
		author := ""
		if m.Author != nil {
			author = *m.Author
		}
		desc := ""
		if m.Description != nil {
			desc = *m.Description
		}

		pb.BackupManga = append(pb.BackupManga, ProtoBackupManga{
			Source:       sourceID,
			URL:          m.URL,
			Title:        m.Title,
			Artist:       artist,
			Author:       author,
			Description:  desc,
			Genre:        m.Genre,
			Status:       models.MangaStatusToInt(m.Status),
			ThumbnailURL: thumb,
			DateAdded:    m.InLibraryAt,
			Chapters:     chs,
			Categories:   cats,
			Favorite:     true,
			History:      history,
		})
	}

	return pb
}

// ValidateBackup inspects the backup data and identifies sources whose extensions are missing or not installed.
func ValidateBackup(ctx context.Context, data []byte) (*ValidateBackupResult, error) {
	res := &ValidateBackupResult{
		MissingSources:  []MissingSourceInfo{},
		MissingTrackers: []interface{}{},
	}
	if len(data) == 0 {
		return res, nil
	}

	// 1. Decompress if gzipped (.tachibk / .proto.gz)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decompressing gzip backup: %w", err)
		}
		defer zr.Close()
		decompressed, err := io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("reading decompressed backup data: %w", err)
		}
		data = decompressed
	}

	var pb *ProtoBackup
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var payload BackupPayload
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return nil, fmt.Errorf("decoding legacy backup JSON: %w", err)
		}
		pb = jsonPayloadToProto(&payload)
	} else {
		var err error
		pb, err = DecodeProtoBackup(data)
		if err != nil {
			return nil, fmt.Errorf("decoding protobuf backup: %w", err)
		}
	}

	sourceMap := make(map[int64]string)
	for _, s := range pb.BackupSources {
		if s.SourceID > 0 {
			sourceMap[s.SourceID] = s.Name
		}
	}
	for _, m := range pb.BackupManga {
		if m.Source > 0 {
			if _, ok := sourceMap[m.Source]; !ok {
				sourceMap[m.Source] = ""
			}
		}
	}

	if database.DB == nil {
		return res, nil
	}

	for srcID, srcName := range sourceMap {
		if srcID == 0 { // Local source is always available
			continue
		}

		var isInstalled bool
		var pkgName, dbName string
		err := database.DB.QueryRow(ctx, `
			SELECT COALESCE(e.is_installed, false), COALESCE(e.pkg_name, ''), COALESCE(s.name, '')
			FROM suwayomi.source s
			LEFT JOIN suwayomi.extension e ON s.extension = e.id
			WHERE s.id = $1
		`, srcID).Scan(&isInstalled, &pkgName, &dbName)

		if err != nil || !isInstalled || strings.HasPrefix(pkgName, "imported.source.") {
			finalName := srcName
			if finalName == "" && dbName != "" {
				finalName = dbName
			}
			if finalName == "" {
				if known, ok := database.KnownSourcesByID[srcID]; ok && known.Name != "" {
					finalName = known.Name
				}
			}
			if finalName == "" {
				finalName = fmt.Sprintf("Source %d", srcID)
			}

			res.MissingSources = append(res.MissingSources, MissingSourceInfo{
				ID:   strconv.FormatInt(srcID, 10),
				Name: finalName,
			})
		}
	}

	return res, nil
}


var (
	backupSchedulerStop chan struct{}
	backupSchedulerMu   sync.Mutex
)

// StartBackupScheduler initializes the automated background backup scheduler.
func StartBackupScheduler(ctx context.Context, cfg *config.Config) {
	backupSchedulerMu.Lock()
	defer backupSchedulerMu.Unlock()
	restartBackupSchedulerLocked(ctx, cfg)
}

// RestartBackupScheduler dynamically restarts the backup scheduler when settings change.
func RestartBackupScheduler(ctx context.Context, cfg *config.Config) {
	backupSchedulerMu.Lock()
	defer backupSchedulerMu.Unlock()
	restartBackupSchedulerLocked(ctx, cfg)
}

func restartBackupSchedulerLocked(ctx context.Context, cfg *config.Config) {
	if backupSchedulerStop != nil {
		close(backupSchedulerStop)
		backupSchedulerStop = nil
	}

	intervalDays := 1
	backupTimeStr := "00:00"
	backupPath := ""
	backupTTL := 14

	if database.DB != nil {
		if sMap, err := database.GetAllServerSettings(ctx); err == nil && sMap != nil {
			if v, ok := sMap["backupInterval"]; ok && v != "" {
				if parsed, err := strconv.Atoi(v); err == nil {
					intervalDays = parsed
				}
			}
			if v, ok := sMap["backupTime"]; ok && v != "" {
				backupTimeStr = v
			}
			if v, ok := sMap["backupPath"]; ok && v != "" {
				backupPath = v
			}
			if v, ok := sMap["backupTTL"]; ok && v != "" {
				if parsed, err := strconv.Atoi(v); err == nil {
					backupTTL = parsed
				}
			}
		}
	}

	if intervalDays <= 0 {
		log.Printf("[AUTO-BACKUP] Automated backups disabled (interval <= 0)")
		return
	}

	bHour, bMinute := 0, 0
	parts := strings.Split(strings.TrimSpace(backupTimeStr), ":")
	if len(parts) >= 1 {
		if h, err := strconv.Atoi(parts[0]); err == nil && h >= 0 && h < 24 {
			bHour = h
		}
	}
	if len(parts) >= 2 {
		if m, err := strconv.Atoi(parts[1]); err == nil && m >= 0 && m < 60 {
			bMinute = m
		}
	}

	stopChan := make(chan struct{})
	backupSchedulerStop = stopChan

	log.Printf("[AUTO-BACKUP] Scheduled automated backup active (every %d day(s) at %02d:%02d)", intervalDays, bHour, bMinute)

	go func(days int, hour, minute int, bPath string, ttl int, stop chan struct{}) {
		for {
			now := time.Now()
			nextRun := CalculateNextWallClockRun(now, float64(days*24), hour, minute)
			delay := time.Until(nextRun)
			log.Printf("[AUTO-BACKUP] Next automated backup scheduled for %s (in %v)",
				nextRun.Format("2006-01-02 15:04:05 MST"), delay.Round(time.Second))

			timer := time.NewTimer(delay)
			select {
			case <-stop:
				timer.Stop()
				return
			case <-timer.C:
				runScheduledBackup(cfg, bPath, ttl)
				time.Sleep(2 * time.Second)
			}
		}
	}(intervalDays, bHour, bMinute, backupPath, backupTTL, stopChan)
}

func runScheduledBackup(cfg *config.Config, customPath string, ttl int) {
	log.Printf("[AUTO-BACKUP] Triggering automated backup...")
	data, err := ExportTachibk(context.Background())
	if err != nil {
		log.Printf("[AUTO-BACKUP] Error exporting backup: %v", err)
		return
	}

	destDir := ""
	if customPath != "" {
		destDir = customPath
	} else if cfg != nil && cfg.Data.RootDir != "" {
		destDir = filepath.Join(cfg.Data.RootDir, "backups")
	} else {
		destDir = filepath.Join(".", "data", "backups")
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		log.Printf("[AUTO-BACKUP] Error creating backup directory %s: %v", destDir, err)
		return
	}

	filename := fmt.Sprintf("org.suwayomi.tachidesk_backup_%s.tachibk", time.Now().Format("2006-01-02_15-04"))
	destFile := filepath.Join(destDir, filename)
	tempFile := destFile + ".tmp"

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		log.Printf("[AUTO-BACKUP] Error writing backup file: %v", err)
		return
	}
	_ = os.Rename(tempFile, destFile)
	log.Printf("[AUTO-BACKUP] Successfully created automated backup: %s (%d bytes)", destFile, len(data))

	if ttl > 0 {
		entries, err := os.ReadDir(destDir)
		if err == nil {
			var backupFiles []os.FileInfo
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tachibk") {
					if info, err := entry.Info(); err == nil {
						backupFiles = append(backupFiles, info)
					}
				}
			}
			if len(backupFiles) > ttl {
				sort.Slice(backupFiles, func(i, j int) bool {
					return backupFiles[i].ModTime().Before(backupFiles[j].ModTime())
				})
				toDelete := len(backupFiles) - ttl
				for i := 0; i < toDelete; i++ {
					delPath := filepath.Join(destDir, backupFiles[i].Name())
					_ = os.Remove(delPath)
					log.Printf("[AUTO-BACKUP] Pruned expired backup: %s", backupFiles[i].Name())
				}
			}
		}
	}
}
