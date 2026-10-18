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
	"tsuruyomi/internal/service"
	"tsuruyomi/internal/sources"
)

func main() {
	configPath := flag.String("config", "/config/config.yaml", "Path to config file")
	idsFlag := flag.String("ids", "", "Comma-separated list of manga IDs to migrate (e.g. 2,3,4)")
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

	// Connect to PostgreSQL
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
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
	}

	if len(targetIDs) == 0 {
		log.Fatalf("No valid manga IDs provided to migrate. Use -ids 2,3,4,...")
	}

	fmt.Println("================================================================================")
	fmt.Printf(" Tsuruyomi Manga Migration Utility (IDs: %v | Dry-Run: %v)\n", targetIDs, *dryRun)
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
