package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/sources"
)

// AuditReport holds the health report across all sources and chapters.
type AuditReport struct {
	TotalSources      int           `json:"totalSources"`
	TotalMangas       int           `json:"totalMangas"`
	TotalChapters     int           `json:"totalChapters"`
	ZeroDateChapters  int           `json:"zeroDateChapters"`
	EmptyChapterNames int           `json:"emptyChapterNames"`
	ZeroChapterMangas int           `json:"zeroChapterMangas"`
	EmptyThumbnails   int           `json:"emptyThumbnails"`
	SourcesWithIssues []SourceIssue `json:"sourcesWithIssues"`
}

// SourceIssue holds per-source issue breakdown.
type SourceIssue struct {
	SourceID         int64  `json:"sourceId"`
	SourceName       string `json:"sourceName"`
	TotalChapters    int    `json:"totalChapters"`
	ZeroDateChapters int    `json:"zeroDateChapters"`
	EmptyNames       int    `json:"emptyNames"`
}

// HealReport summarizes the result of an automated healing pass.
type HealReport struct {
	AuditBefore          AuditReport `json:"auditBefore"`
	AuditAfter           AuditReport `json:"auditAfter"`
	NativeMangasResynced int         `json:"nativeMangasResynced"`
	DatesRepaired        int64       `json:"datesRepaired"`
	NamesRepaired        int64       `json:"namesRepaired"`
	NumbersRepaired      int64       `json:"numbersRepaired"`
	DurationMs           int64       `json:"durationMs"`
}

var healerMu sync.Mutex

// Audit inspects the database and produces a report of anomalies across all sources.
func Audit(ctx context.Context) (AuditReport, error) {
	var report AuditReport
	if database.DB == nil {
		return report, fmt.Errorf("database not connected")
	}

	err := database.DB.QueryRow(ctx, `
		SELECT 
			(SELECT count(DISTINCT source) FROM suwayomi.manga),
			(SELECT count(*) FROM suwayomi.manga),
			(SELECT count(*) FROM suwayomi.chapter),
			(SELECT count(*) FROM suwayomi.chapter WHERE date_upload <= 0),
			(SELECT count(*) FROM suwayomi.chapter WHERE name IS NULL OR trim(name) = ''),
			(SELECT count(*) FROM suwayomi.manga m WHERE NOT EXISTS (SELECT 1 FROM suwayomi.chapter c WHERE c.manga = m.id)),
			(SELECT count(*) FROM suwayomi.manga WHERE thumbnail_url IS NULL OR trim(thumbnail_url) = '')
	`).Scan(
		&report.TotalSources,
		&report.TotalMangas,
		&report.TotalChapters,
		&report.ZeroDateChapters,
		&report.EmptyChapterNames,
		&report.ZeroChapterMangas,
		&report.EmptyThumbnails,
	)
	if err != nil {
		return report, fmt.Errorf("querying global audit stats: %w", err)
	}

	rows, err := database.DB.Query(ctx, `
		SELECT 
			m.source, 
			COALESCE(s.name, 'Unknown Source') as source_name,
			count(c.id) as total_chapters,
			count(c.id) FILTER (WHERE c.date_upload <= 0) as zero_dates,
			count(c.id) FILTER (WHERE c.name IS NULL OR trim(c.name) = '') as empty_names
		FROM suwayomi.manga m
		LEFT JOIN suwayomi.source s ON m.source = s.id
		JOIN suwayomi.chapter c ON m.id = c.manga
		GROUP BY m.source, s.name
		HAVING count(c.id) FILTER (WHERE c.date_upload <= 0) > 0 OR count(c.id) FILTER (WHERE c.name IS NULL OR trim(c.name) = '') > 0
		ORDER BY zero_dates DESC, total_chapters DESC
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var issue SourceIssue
			if errScan := rows.Scan(&issue.SourceID, &issue.SourceName, &issue.TotalChapters, &issue.ZeroDateChapters, &issue.EmptyNames); errScan == nil {
				report.SourcesWithIssues = append(report.SourcesWithIssues, issue)
			}
		}
	}

	return report, nil
}

// Heal detects anomalies across all sources and repairs them automatically.
func Heal(ctx context.Context) (*HealReport, error) {
	healerMu.Lock()
	defer healerMu.Unlock()

	start := time.Now()
	report := &HealReport{}

	auditBefore, err := Audit(ctx)
	if err != nil {
		return nil, fmt.Errorf("pre-heal audit failed: %w", err)
	}
	report.AuditBefore = auditBefore

	// 1. For native Go sources: re-sync mangas that have zero-date chapters using updated scrapers
	nativeResynced := 0
	if database.DB != nil {
		rows, errRows := database.DB.Query(ctx, `
			SELECT DISTINCT m.id, m.source, m.url 
			FROM suwayomi.manga m 
			JOIN suwayomi.chapter c ON m.id = c.manga 
			WHERE c.date_upload <= 0
		`)
		if errRows == nil {
			type targetManga struct {
				id     int64
				source int64
				url    string
			}
			var targets []targetManga
			for rows.Next() {
				var t targetManga
				if errScan := rows.Scan(&t.id, &t.source, &t.url); errScan == nil {
					if sources.Has(t.source) {
						targets = append(targets, t)
					}
				}
			}
			rows.Close()

			for _, t := range targets {
				if _, errFetch := sources.FetchAndSaveMangaChapters(ctx, t.source, t.id, t.url); errFetch == nil {
					nativeResynced++
				}
			}
		}
	}
	report.NativeMangasResynced = nativeResynced

	// 2. Auto-heal any remaining date_upload <= 0 across ALL sources in the database
	if database.DB != nil {
		resDates, errDates := database.DB.Exec(ctx, `
			UPDATE suwayomi.chapter 
			SET date_upload = CASE 
				WHEN fetched_at > 0 THEN fetched_at * 1000 
				ELSE EXTRACT(EPOCH FROM NOW())::bigint * 1000 
			END 
			WHERE date_upload <= 0;
		`)
		if errDates == nil {
			report.DatesRepaired = resDates.RowsAffected()
		}

		// 3. Auto-heal empty chapter names
		resNames, errNames := database.DB.Exec(ctx, `
			UPDATE suwayomi.chapter 
			SET name = CASE 
				WHEN chapter_number > 0 THEN 'Capítulo ' || chapter_number::text 
				WHEN source_order > 0 THEN 'Capítulo ' || source_order::text 
				ELSE 'Capítulo ' || id::text 
			END 
			WHERE name IS NULL OR trim(name) = '';
		`)
		if errNames == nil {
			report.NamesRepaired = resNames.RowsAffected()
		}

		// 4. Auto-heal broken chapter numbers
		resNumbers, errNumbers := database.DB.Exec(ctx, `
			UPDATE suwayomi.chapter 
			SET chapter_number = source_order 
			WHERE chapter_number <= 0 AND source_order > 0;
		`)
		if errNumbers == nil {
			report.NumbersRepaired = resNumbers.RowsAffected()
		}
	}

	auditAfter, _ := Audit(ctx)
	report.AuditAfter = auditAfter
	report.DurationMs = time.Since(start).Milliseconds()

	log.Printf("[AUTO-HEALER] Pass finished in %dms: native resynced=%d, dates repaired=%d, names repaired=%d, numbers repaired=%d",
		report.DurationMs, report.NativeMangasResynced, report.DatesRepaired, report.NamesRepaired, report.NumbersRepaired)

	return report, nil
}

// StartHealerScheduler launches background scheduled health checks and healing.
func StartHealerScheduler(ctx context.Context) {
	go func() {
		// Wait 8 seconds after server startup for database and network readiness
		select {
		case <-time.After(8 * time.Second):
		case <-ctx.Done():
			return
		}

		log.Printf("[AUTO-HEALER] Running initial background audit & healing...")
		if report, err := Heal(context.Background()); err == nil {
			if report.DatesRepaired > 0 || report.NamesRepaired > 0 || report.NativeMangasResynced > 0 {
				log.Printf("[AUTO-HEALER] Startup healing repaired %d dates, %d names (native resynced: %d)",
					report.DatesRepaired, report.NamesRepaired, report.NativeMangasResynced)
			} else {
				log.Printf("[AUTO-HEALER] All sources and chapters are healthy. No repairs needed.")
			}
		} else {
			log.Printf("[AUTO-HEALER] Startup healing error: %v", err)
		}

		// Run periodic background audit and healing every 6 hours
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Printf("[AUTO-HEALER] Running periodic background audit & healing...")
				if report, err := Heal(context.Background()); err == nil {
					if report.DatesRepaired > 0 || report.NamesRepaired > 0 || report.NativeMangasResynced > 0 {
						log.Printf("[AUTO-HEALER] Periodic healing repaired %d dates, %d names",
							report.DatesRepaired, report.NamesRepaired)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
