package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/util"
)

// ReconcileResult summarizes the outcome of a downloads storage audit.
type ReconcileResult struct {
	CleanedGhostDownloads int   `json:"cleanedGhostDownloads"`
	RestoredDownloads     int   `json:"restoredDownloads"`
	TotalChecked          int   `json:"totalChecked"`
	DurationMs            int64 `json:"durationMs"`
}

type chapterCheckInfo struct {
	id         int
	name       string
	scanlator  string
	mangaTitle string
	sourceName string
}

// ReconcileDownloads audits physical files on disk against database records:
// 1. Clears is_downloaded = false for chapters where .cbz or chapter directory was deleted.
// 2. Discovers existing .cbz or chapter directories for library chapters and sets is_downloaded = true.
func ReconcileDownloads(ctx context.Context, cfg *config.Config) (ReconcileResult, error) {
	start := time.Now()
	res := ReconcileResult{}

	if database.DB == nil {
		return res, fmt.Errorf("database not initialized")
	}

	// 1. Fetch all chapters currently marked as downloaded
	queryMarked := `
		SELECT c.id, c.name, COALESCE(c.scanlator, ''), m.title, COALESCE(s.name, 'Local Source')
		FROM suwayomi.chapter c
		JOIN suwayomi.manga m ON c.manga = m.id
		LEFT JOIN suwayomi.source s ON m.source = s.id
		LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter AND cu.user_id = 1
		WHERE COALESCE(cu.is_downloaded, false) = true OR COALESCE(c.is_downloaded, false) = true
	`
	rows, err := database.DB.Query(ctx, queryMarked)
	if err != nil {
		return res, fmt.Errorf("querying marked chapters: %w", err)
	}

	var markedList []chapterCheckInfo
	for rows.Next() {
		var ci chapterCheckInfo
		if err := rows.Scan(&ci.id, &ci.name, &ci.scanlator, &ci.mangaTitle, &ci.sourceName); err == nil {
			markedList = append(markedList, ci)
		}
	}
	rows.Close()

	res.TotalChecked = len(markedList)
	var missingIDs []int

	for _, ci := range markedList {
		basePath := util.GetChapterPath(cfg, ci.sourceName, ci.mangaTitle, ci.name, ci.scanlator)
		cbzPath := basePath + ".cbz"

		exists := false
		if info, err := os.Stat(cbzPath); err == nil && !info.IsDir() {
			exists = true
		} else if info, err := os.Stat(basePath); err == nil && info.IsDir() {
			exists = true
		}

		if !exists {
			missingIDs = append(missingIDs, ci.id)
		}
	}

	if len(missingIDs) > 0 {
		_, _ = database.DB.Exec(ctx, `
			UPDATE suwayomi.chapter 
			SET is_downloaded = false 
			WHERE id = ANY($1)
		`, missingIDs)

		_, _ = database.DB.Exec(ctx, `
			UPDATE suwayomi.chapteruser 
			SET is_downloaded = false 
			WHERE user_id = 1 AND chapter = ANY($1)
		`, missingIDs)

		res.CleanedGhostDownloads = len(missingIDs)
		log.Printf("[RECONCILER] Cleaned %d ghost downloaded chapters (files no longer on disk)", len(missingIDs))
	}

	// 2. Discover files on disk for in-library chapters that may be missing the downloaded flag
	queryLibrary := `
		SELECT c.id, c.name, COALESCE(c.scanlator, ''), m.title, COALESCE(s.name, 'Local Source')
		FROM suwayomi.chapter c
		JOIN suwayomi.manga m ON c.manga = m.id
		LEFT JOIN suwayomi.source s ON m.source = s.id
		LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter AND cu.user_id = 1
		LEFT JOIN suwayomi.mangauser mu ON m.id = mu.manga AND mu.user_id = 1
		WHERE COALESCE(mu.in_library, false) = true 
		  AND COALESCE(cu.is_downloaded, false) = false
		  AND COALESCE(c.is_downloaded, false) = false
	`
	libRows, err := database.DB.Query(ctx, queryLibrary)
	if err == nil {
		var unflaggedList []chapterCheckInfo
		for libRows.Next() {
			var ci chapterCheckInfo
			if err := libRows.Scan(&ci.id, &ci.name, &ci.scanlator, &ci.mangaTitle, &ci.sourceName); err == nil {
				unflaggedList = append(unflaggedList, ci)
			}
		}
		libRows.Close()

		var foundIDs []int
		for _, ci := range unflaggedList {
			basePath := util.GetChapterPath(cfg, ci.sourceName, ci.mangaTitle, ci.name, ci.scanlator)
			cbzPath := basePath + ".cbz"

			exists := false
			if info, err := os.Stat(cbzPath); err == nil && !info.IsDir() {
				exists = true
			} else if info, err := os.Stat(basePath); err == nil && info.IsDir() {
				exists = true
			}

			if exists {
				foundIDs = append(foundIDs, ci.id)
			}
		}

		if len(foundIDs) > 0 {
			_, _ = database.DB.Exec(ctx, `
				UPDATE suwayomi.chapter 
				SET is_downloaded = true 
				WHERE id = ANY($1)
			`, foundIDs)

			_, _ = database.DB.Exec(ctx, `
				INSERT INTO suwayomi.chapteruser (user_id, chapter, is_downloaded)
				SELECT 1, unnest($1::int[]), true
				ON CONFLICT (user_id, chapter) DO UPDATE SET is_downloaded = true
			`, foundIDs)

			res.RestoredDownloads = len(foundIDs)
			log.Printf("[RECONCILER] Restored downloaded flag for %d chapters found on disk", len(foundIDs))
		}
	}

	res.DurationMs = time.Since(start).Milliseconds()
	log.Printf("[RECONCILER] Reconciliation completed in %d ms: %d ghosts cleaned, %d restored (checked %d)",
		res.DurationMs, res.CleanedGhostDownloads, res.RestoredDownloads, res.TotalChecked)

	return res, nil
}
