package service

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/sources"
	"tsuruyomi/internal/util"
)

type ComicInfoXML struct {
	XMLName         xml.Name `xml:"ComicInfo"`
	Title           string   `xml:"Title,omitempty"`
	Series          string   `xml:"Series,omitempty"`
	Number          string   `xml:"Number,omitempty"`
	PageCount       int      `xml:"PageCount,omitempty"`
	ScanInformation string   `xml:"ScanInformation,omitempty"`
}

var (
	inFlightDownloads sync.Map // map[int]chan struct{}
	inFlightMu        sync.Mutex
)

// EnsureChapterCBZ guarantees that the CBZ archive exists on disk.
// If it is already downloaded, it returns the existing path immediately.
// If not, it automatically discovers all chapter pages, downloads them concurrently,
// generates the .cbz archive with ComicInfo metadata, updates the database, and returns the path.
func EnsureChapterCBZ(ctx context.Context, cfg *config.Config, chapterID int) (string, error) {
	if database.DB == nil {
		return "", fmt.Errorf("database not initialized")
	}

	// 1. Fetch chapter and manga metadata
	query := `
		SELECT 
			c.id, c.manga, c.name, c.chapter_number, c.source_order, c.scanlator, c.url, c.real_url,
			m.title, m.source, m.real_url,
			COALESCE(s.name, 'Local Source')
		FROM suwayomi.chapter c
		INNER JOIN suwayomi.manga m ON c.manga = m.id
		LEFT JOIN suwayomi.source s ON m.source = s.id
		WHERE c.id = $1
	`

	var (
		chID, mangaID, sourceOrder               int
		chName, mTitle, sourceName               string
		chNumber                                 float64
		scanlator, chURL, chRealURL, mRealURL    sql.NullString
		sourceID                                 int64
	)

	err := database.DB.QueryRow(ctx, query, chapterID).Scan(
		&chID, &mangaID, &chName, &chNumber, &sourceOrder, &scanlator, &chURL, &chRealURL,
		&mTitle, &sourceID, &mRealURL,
		&sourceName,
	)
	if err != nil {
		return "", fmt.Errorf("chapter %d not found: %w", chapterID, err)
	}

	scan := ""
	if scanlator.Valid {
		scan = scanlator.String
	}

	basePath := util.GetChapterPath(cfg, sourceName, mTitle, chName, scan)
	cbzPath := basePath + ".cbz"

	// 2. Check if .cbz already exists on disk
	if info, err := os.Stat(cbzPath); err == nil && !info.IsDir() && info.Size() > 1024 {
		return cbzPath, nil
	}

	// 3. Prevent duplicate concurrent downloads for the exact same chapter
	inFlightMu.Lock()
	val, loaded := inFlightDownloads.Load(chapterID)
	if loaded {
		inFlightMu.Unlock()
		// Wait for existing download to finish
		ch := val.(chan struct{})
		select {
		case <-ch:
			if _, err := os.Stat(cbzPath); err == nil {
				return cbzPath, nil
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	} else {
		ch := make(chan struct{})
		inFlightDownloads.Store(chapterID, ch)
		inFlightMu.Unlock()

		defer func() {
			inFlightDownloads.Delete(chapterID)
			close(ch)
		}()
	}

	log.Printf("[OPDS/DOWNLOAD] Chapter %d (%s - %s) not present as CBZ. Starting on-demand download...", chapterID, mTitle, chName)

	// 4. Page Discovery: Fetch page URLs from database or source
	type pageEntry struct {
		Order    int
		Index    int
		ImageURL string
	}

	fetchPagesFromDB := func() ([]pageEntry, error) {
		rows, err := database.DB.Query(ctx, "SELECT index, COALESCE(image_url, '') FROM suwayomi.page WHERE chapter = $1 ORDER BY index ASC", chapterID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var pages []pageEntry
		order := 0
		for rows.Next() {
			var p pageEntry
			if err := rows.Scan(&p.Index, &p.ImageURL); err == nil {
				p.Order = order
				pages = append(pages, p)
				order++
			}
		}
		return pages, nil
	}

	targetChURL := ""
	if chRealURL.Valid && chRealURL.String != "" {
		targetChURL = chRealURL.String
	} else if chURL.Valid {
		targetChURL = chURL.String
	}
	targetMangaURL := ""
	if mRealURL.Valid {
		targetMangaURL = mRealURL.String
	}

	pages, err := fetchPagesFromDB()
	if err != nil || len(pages) == 0 {
		if sources.Has(sourceID) {
			_, errScrape := sources.FetchAndSaveChapterPages(ctx, sourceID, mangaID, int64(chapterID), sourceOrder, targetChURL, targetMangaURL)
			if errScrape != nil {
				log.Printf("[OPDS/DOWNLOAD] Native scraping warning: %v", errScrape)
			}
		} else if cfg.Sidecar.Enabled && cfg.Sidecar.BaseURL != "" {
			// Trigger JVM sidecar page resolution by requesting page 0
			sidecarPageURL := fmt.Sprintf("%s/api/v1/manga/%d/chapter/%d/page/0",
				strings.TrimSuffix(cfg.Sidecar.BaseURL, "/"), mangaID, sourceOrder)
			req, reqErr := http.NewRequestWithContext(ctx, "GET", sidecarPageURL, nil)
			if reqErr == nil {
				client := &http.Client{Timeout: 35 * time.Second}
				resp, respErr := client.Do(req)
				if respErr == nil {
					_ = resp.Body.Close()
				}
			}
		}

		// Re-query database after scraping/resolution
		pages, _ = fetchPagesFromDB()
	}

	if len(pages) == 0 {
		return "", fmt.Errorf("unable to discover page list for chapter %d (%s)", chapterID, chName)
	}

	log.Printf("[OPDS/DOWNLOAD] Discovered %d pages for chapter %d. Downloading page assets...", len(pages), chapterID)

	// 5. Download and cache all pages concurrently (4 workers)
	cacheDir := filepath.Join(cfg.Data.RootDir, "cache", "pages", fmt.Sprintf("%d", mangaID), fmt.Sprintf("ch_%d", chapterID))
	_ = os.MkdirAll(cacheDir, 0755)

	type downloadedPage struct {
		Index    int
		Data     []byte
		MimeType string
	}

	downloadedPages := make([]downloadedPage, len(pages))
	var dlWg sync.WaitGroup
	pageChan := make(chan pageEntry, len(pages))
	errChan := make(chan error, 1)

	dlCtx, cancelDl := context.WithCancel(ctx)
	defer cancelDl()

	workerCount := 4
	if len(pages) < workerCount {
		workerCount = len(pages)
	}

	for w := 0; w < workerCount; w++ {
		dlWg.Add(1)
		go func() {
			defer dlWg.Done()
			for {
				select {
				case <-dlCtx.Done():
					return
				case p, ok := <-pageChan:
					if !ok {
						return
					}

					cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d.img", p.Index))

					// Check cache first
					if cached, err := os.ReadFile(cachePath); err == nil && len(cached) > 0 {
						downloadedPages[p.Order] = downloadedPage{
							Index:    p.Order,
							Data:     cached,
							MimeType: util.DetectImageType(cached),
						}
						continue
					}

					var data []byte
					var mimeType string
					var dlErr error

					referer := targetMangaURL
					if referer == "" {
						referer = targetChURL
					}

					for attempt := 0; attempt < 3; attempt++ {
						if attempt > 0 {
							select {
							case <-dlCtx.Done():
								return
							case <-time.After(time.Duration(attempt*500) * time.Millisecond):
							}
						}

						// Download via FlareSolverr / Direct HTTP with referer
						if p.ImageURL != "" {
							data, mimeType, dlErr = util.DownloadWithFlareSolverrAndReferer(dlCtx, cfg, p.ImageURL, referer)
							if dlErr == nil && len(data) > 0 {
								break
							}
						}

						// If failed or empty URL, try sidecar proxy
						if cfg.Sidecar.Enabled && cfg.Sidecar.BaseURL != "" {
							sidecarURL := fmt.Sprintf("%s/api/v1/manga/%d/chapter/%d/page/%d",
								strings.TrimSuffix(cfg.Sidecar.BaseURL, "/"), mangaID, sourceOrder, p.Index)
							scReq, scReqErr := http.NewRequestWithContext(dlCtx, "GET", sidecarURL, nil)
							if scReqErr == nil {
								scClient := &http.Client{Timeout: 40 * time.Second}
								scResp, scRespErr := scClient.Do(scReq)
								if scRespErr == nil && scResp.StatusCode == http.StatusOK {
									data, dlErr = io.ReadAll(scResp.Body)
									mimeType = scResp.Header.Get("Content-Type")
									_ = scResp.Body.Close()
									if dlErr == nil && len(data) > 0 {
										break
									}
								}
							}
						}
					}

					if dlErr != nil || len(data) == 0 {
						cancelDl()
						select {
						case errChan <- fmt.Errorf("failed downloading page %d: %v", p.Index, dlErr):
						default:
						}
						return
					}

					// Cache locally on disk
					_ = os.WriteFile(cachePath, data, 0644)

					if mimeType == "" {
						mimeType = util.DetectImageType(data)
					}

					downloadedPages[p.Order] = downloadedPage{
						Index:    p.Order,
						Data:     data,
						MimeType: mimeType,
					}
				}
			}
		}()
	}

	for _, p := range pages {
		pageChan <- p
	}
	close(pageChan)
	dlWg.Wait()

	select {
	case err := <-errChan:
		return "", fmt.Errorf("page download failed: %w", err)
	default:
	}

	// 6. Build the CBZ (ZIP) Archive
	_ = os.MkdirAll(filepath.Dir(cbzPath), 0755)
	tempCbz := cbzPath + ".tmp"
	zipFile, err := os.Create(tempCbz)
	if err != nil {
		return "", fmt.Errorf("creating temp CBZ file: %w", err)
	}

	zipWriter := zip.NewWriter(zipFile)

	// Add ComicInfo.xml metadata
	comicInfo := ComicInfoXML{
		Title:           chName,
		Series:          mTitle,
		Number:          fmt.Sprintf("%g", chNumber),
		PageCount:       len(downloadedPages),
		ScanInformation: scan,
	}
	xmlData, err := xml.MarshalIndent(comicInfo, "", "  ")
	if err == nil {
		if w, err := zipWriter.Create("ComicInfo.xml"); err == nil {
			_, _ = w.Write([]byte(xml.Header + string(xmlData)))
		}
	}

	// Write all downloaded page images
	for i, dp := range downloadedPages {
		if len(dp.Data) == 0 {
			_ = zipWriter.Close()
			_ = zipFile.Close()
			_ = os.Remove(tempCbz)
			return "", fmt.Errorf("page %d has no data", i)
		}
		ext := ".jpg"
		switch dp.MimeType {
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		case "image/gif":
			ext = ".gif"
		}

		entryName := fmt.Sprintf("%03d%s", i+1, ext)
		w, err := zipWriter.Create(entryName)
		if err != nil {
			_ = zipWriter.Close()
			_ = zipFile.Close()
			_ = os.Remove(tempCbz)
			return "", fmt.Errorf("writing entry %s to CBZ: %w", entryName, err)
		}
		if _, err := w.Write(dp.Data); err != nil {
			_ = zipWriter.Close()
			_ = zipFile.Close()
			_ = os.Remove(tempCbz)
			return "", fmt.Errorf("writing page data for %s: %w", entryName, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		_ = zipFile.Close()
		_ = os.Remove(tempCbz)
		return "", fmt.Errorf("closing zip writer: %w", err)
	}
	if err := zipFile.Close(); err != nil {
		_ = os.Remove(tempCbz)
		return "", fmt.Errorf("closing zip file: %w", err)
	}

	// Atomic rename to final cbzPath
	if err := os.Rename(tempCbz, cbzPath); err != nil {
		// Fallback for Windows if destination exists
		_ = os.Remove(cbzPath)
		if err := os.Rename(tempCbz, cbzPath); err != nil {
			return "", fmt.Errorf("finalizing CBZ archive: %w", err)
		}
	}

	// 7. Update database download status and page count
	_, _ = database.DB.Exec(ctx, "UPDATE suwayomi.chapter SET is_downloaded = true, page_count = $1 WHERE id = $2", len(downloadedPages), chapterID)
	_, _ = database.DB.Exec(ctx, `
		INSERT INTO suwayomi.chapteruser (user_id, chapter, is_downloaded)
		VALUES (1, $1, true)
		ON CONFLICT (user_id, chapter) DO UPDATE SET is_downloaded = true
	`, chapterID)

	log.Printf("[OPDS/DOWNLOAD] Successfully assembled and saved CBZ for chapter %d at %s (%d pages)", chapterID, cbzPath, len(downloadedPages))
	return cbzPath, nil
}
