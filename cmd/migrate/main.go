package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/extensions"
	"tsuruyomi/internal/service"
	"tsuruyomi/internal/sources"
)

func main() {
	configPath := flag.String("config", "/config/config.yaml", "Path to config file")
	idsFlag := flag.String("ids", "", "Comma-separated list of manga IDs to migrate (e.g. 2,3,4)")
	allTmo := flag.Bool("all-tmo", false, "Migrate all TuMangaOnline mangas in library")
	limitFlag := flag.Int("limit", 0, "Limit number of mangas to migrate (0 for no limit)")
	dryRun := flag.Bool("dry-run", false, "Preview migration without modifying the database")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		// Try fallback paths
		fallbacks := []string{"config.yaml", "/data/config.json"}
		for _, fb := range fallbacks {
			if c, e := config.Load(fb); e == nil {
				cfg = c
				err = nil
				break
			}
		}
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}
	}

	sources.SetConfig(cfg)
	if err := extensions.LoadInstalledExtensions(cfg.Data.RootDir); err != nil {
		log.Printf("[MIGRATE] Warning loading installed extensions: %v", err)
	}

	// Connect to PostgreSQL
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := database.Connect(ctx, cfg.Database); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	var targetIDs []int64
	if *idsFlag != "" {
		for _, part := range strings.Split(*idsFlag, ",") {
			part = strings.TrimSpace(part)
			if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
				targetIDs = append(targetIDs, id)
			}
		}
	} else if *allTmo {
		query := `
			SELECT m.id FROM suwayomi.manga m
			JOIN suwayomi.mangauser mu ON m.id = mu.manga
			WHERE m.source = 7925520943983324764 AND mu.in_library = true
			ORDER BY m.id
		`
		if *limitFlag > 0 {
			query += fmt.Sprintf(" LIMIT %d", *limitFlag)
		}
		rows, err := database.DB.Query(ctx, query)
		if err != nil {
			log.Fatalf("Failed to query TMO mangas: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			var mid int64
			if err := rows.Scan(&mid); err == nil {
				targetIDs = append(targetIDs, mid)
			}
		}
	}

	if len(targetIDs) == 0 {
		log.Fatalf("No valid manga IDs provided to migrate. Use -ids 2,3,4,... or -all-tmo")
	}

	fmt.Println("================================================================================")
	fmt.Printf(" Tsuruyomi Manga Migration Utility (Total: %d | Dry-Run: %v)\n", len(targetIDs), *dryRun)
	fmt.Println("================================================================================")

	var reports []*service.MigrationReport

	for i, id := range targetIDs {
		fmt.Printf("\n[%d/%d] Processing TMO Manga ID %d...\n", i+1, len(targetIDs), id)

		if *dryRun {
			// Dry-run mode: only search target
			var oldTitle, oldURL, oldDesc string
			var oldSource int64
			err := database.DB.QueryRow(ctx, "SELECT source, url, title, description FROM suwayomi.manga WHERE id = $1", id).Scan(&oldSource, &oldURL, &oldTitle, &oldDesc)
			if err != nil {
				fmt.Printf("  -> Error reading manga: %v\n", err)
				continue
			}

			target, errFind := service.FindBestMigrationTarget(ctx, oldTitle, oldURL, oldDesc)
			if errFind != nil || target == nil {
				fmt.Printf("  -> NOT FOUND: %v\n", errFind)
				reports = append(reports, &service.MigrationReport{
					OldMangaID: id,
					OldTitle:   oldTitle,
					Status:     "NOT_FOUND",
					Message:    fmt.Sprintf("%v", errFind),
				})
			} else {
				fmt.Printf("  -> FOUND MATCH: %s (%s) | Target Chapters: %d | Lang: %s\n", target.Title, sources.GetSourceName(target.SourceID), target.ChapterCount, target.Language)
				reports = append(reports, &service.MigrationReport{
					OldMangaID:       id,
					OldTitle:         oldTitle,
					NewTitle:         target.Title,
					NewSource:        target.SourceID,
					NewSourceName:    sources.GetSourceName(target.SourceID),
					NewTotalChapters: target.ChapterCount,
					Status:           "READY",
					Message:          "Found match online",
				})
			}
			continue
		}

		rep, errMig := service.MigrateSingleManga(ctx, cfg, id, nil)
		if errMig != nil {
			fmt.Printf("  -> Migration failed: %v\n", errMig)
		} else {
			fmt.Printf("  -> %s: %s\n", rep.Status, rep.Message)
		}
		if rep != nil {
			reports = append(reports, rep)
		}
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("\n================================================================================")
	fmt.Println(" MIGRATION PILOT REPORT SUMMARY")
	fmt.Println("================================================================================")
	fmt.Println("| ID | Original Title (TMO) | Target Title | Target Source | Old Read/Total | New Read/Total | Status |")
	fmt.Println("|---|---|---|---|---|---|---|")
	for _, r := range reports {
		fmt.Printf("| %d | %s | %s | %s | %d/%d | %d/%d | %s |\n",
			r.OldMangaID,
			truncate(r.OldTitle, 25),
			truncate(r.NewTitle, 25),
			r.NewSourceName,
			r.OldReadChapters, r.OldTotalChapters,
			r.NewReadChapters, r.NewTotalChapters,
			r.Status,
		)
	}
	fmt.Println("================================================================================")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
