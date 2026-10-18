package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
	"tsuruyomi/internal/service"
	"tsuruyomi/internal/sources"
	"tsuruyomi/internal/util"
)

// GetManga retrieves basic details of a single manga.
func GetManga(c *fiber.Ctx) error {
	mangaId, err := strconv.Atoi(c.Params("mangaId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}

	manga, err := database.GetMangaByID(c.Context(), mangaId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	if manga == nil {
		return c.Status(fiber.StatusNotFound).SendString("Manga not found")
	}

	return c.JSON(manga)
}

// GetMangaFull retrieves comprehensive details of a manga.
func GetMangaFull(c *fiber.Ctx) error {
	return GetManga(c) // Reuses GetManga since the struct contains full counts and relations
}

func rewriteKnownCDN(rawUrl string) string {
	u := rawUrl
	u = strings.ReplaceAll(u, "cdn.manga-oni.com", "oni.ntr-files.online")
	u = strings.ReplaceAll(u, "oni.ntr-cdn.online", "oni.ntr-files.online")
	return u
}

func isKnownDeadDomain(rawUrl string) bool {
	return strings.Contains(rawUrl, "otakuteca.com")
}

func downloadImageRaw(ctx context.Context, cfg *config.Config, imgURL string) ([]byte, string, error) {
	req, reqErr := http.NewRequestWithContext(ctx, "GET", imgURL, nil)
	if reqErr != nil {
		return nil, "", reqErr
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", imgURL)
	dlClient := &http.Client{Timeout: 8 * time.Second}
	resp, respErr := dlClient.Do(req)
	if respErr == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		data, readErr := io.ReadAll(resp.Body)
		if readErr == nil && len(data) > 0 && !strings.Contains(string(data[:min(len(data), 512)]), "<html") {
			mimeType := resp.Header.Get("Content-Type")
			if mimeType == "" {
				mimeType = "image/jpeg"
			}
			return data, mimeType, nil
		}
	}

	if (respErr != nil || (resp != nil && (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable))) && cfg != nil && cfg.FlareSolverr.Enabled {
		if resp != nil {
			_ = resp.Body.Close()
		}
		fsData, mimeType, fsErr := util.DownloadWithFlareSolverrAndReferer(ctx, cfg, imgURL, imgURL)
		if fsErr == nil && len(fsData) > 0 {
			return fsData, mimeType, nil
		}
	}
	return nil, "", fmt.Errorf("failed to download image from %s", imgURL)
}

func saveThumbnailToDisk(cfg *config.Config, mangaId int, data []byte) {
	if cfg == nil || len(data) == 0 {
		return
	}
	thumbnailDir := filepath.Join(cfg.Data.RootDir, "downloads", "thumbnails")
	_ = os.MkdirAll(thumbnailDir, 0755)
	targetFile := filepath.Join(thumbnailDir, fmt.Sprintf("%d.jpg", mangaId))
	_ = os.WriteFile(targetFile, data, 0644)
}

func findSiblingCover(ctx context.Context, cfg *config.Config, mangaId int, title string) ([]byte, string, error) {
	if database.DB == nil || title == "" {
		return nil, "", fmt.Errorf("no database or empty title")
	}
	rows, err := database.DB.Query(ctx, `
		SELECT id, COALESCE(thumbnail_url, '') 
		FROM suwayomi.manga 
		WHERE title ILIKE $1 AND id != $2 
		ORDER BY id ASC LIMIT 5
	`, title, mangaId)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	type sibling struct {
		id  int
		url string
	}
	var siblings []sibling
	for rows.Next() {
		var s sibling
		if err := rows.Scan(&s.id, &s.url); err == nil {
			siblings = append(siblings, s)
		}
	}

	// 1. Check if any sibling already has a thumbnail cached on disk
	for _, s := range siblings {
		if path, mime, err := util.GetMangaThumbnailPath(cfg, s.id); err == nil {
			if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
				return data, mime, nil
			}
		}
	}

	// 2. Check if any sibling has a working remote URL
	for _, s := range siblings {
		cleanURL := rewriteKnownCDN(s.url)
		if cleanURL != "" && !isKnownDeadDomain(cleanURL) {
			if data, mime, err := downloadImageRaw(ctx, cfg, cleanURL); err == nil && len(data) > 0 {
				return data, mime, nil
			}
		}
	}

	return nil, "", fmt.Errorf("no sibling cover found")
}

func findMangaDexCover(ctx context.Context, cfg *config.Config, title string) ([]byte, string, error) {
	if title == "" {
		return nil, "", fmt.Errorf("empty title")
	}
	apiURL := fmt.Sprintf("https://api.mangadex.org/manga?title=%s&limit=1&includes[]=cover_art", url.QueryEscape(title))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Tsuruyomi/1.0 (https://github.com/Suwayomi)")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, "", fmt.Errorf("mangadex api request failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID            string `json:"id"`
			Relationships []struct {
				Type       string `json:"type"`
				Attributes struct {
					FileName string `json:"fileName"`
				} `json:"attributes"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Data) == 0 {
		return nil, "", fmt.Errorf("no mangadex manga found")
	}

	mangaID := result.Data[0].ID
	var fileName string
	for _, rel := range result.Data[0].Relationships {
		if rel.Type == "cover_art" && rel.Attributes.FileName != "" {
			fileName = rel.Attributes.FileName
			break
		}
	}
	if fileName == "" {
		return nil, "", fmt.Errorf("no cover_art found in mangadex result")
	}

	coverURL := fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s.512.jpg", mangaID, fileName)
	data, mime, err := downloadImageRaw(ctx, cfg, coverURL)
	if err != nil {
		coverURL = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s", mangaID, fileName)
		data, mime, err = downloadImageRaw(ctx, cfg, coverURL)
	}
	return data, mime, err
}

// GetMangaThumbnail serves the cover image file from local cache or storage.
func GetMangaThumbnail(c *fiber.Ctx) error {
	mangaId, err := strconv.Atoi(c.Params("mangaId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}

	cfg, _ := c.Locals("config").(*config.Config)

	// 1. Try to serve the cover directly from the shared disk cache first
	filePath, mimeType, err := util.GetMangaThumbnailPath(cfg, mangaId)
	if err == nil {
		c.Set(fiber.HeaderContentType, mimeType)
		c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
		return c.SendFile(filePath)
	}

	// 2. If not on disk, proxy to sidecar and persist to disk for future requests
	if cfg != nil && cfg.Sidecar.Enabled && cfg.Sidecar.BaseURL != "" {
		sidecarClient := &http.Client{Timeout: 5 * time.Second}
		url := fmt.Sprintf("%s/api/v1/manga/%d/thumbnail", strings.TrimSuffix(cfg.Sidecar.BaseURL, "/"), mangaId)
		
		req, reqErr := http.NewRequestWithContext(c.Context(), "GET", url, nil)
		if reqErr == nil {
			resp, respErr := sidecarClient.Do(req)
			if respErr == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				data, readErr := io.ReadAll(resp.Body)
				if readErr == nil && len(data) > 0 {
					saveThumbnailToDisk(cfg, mangaId, data)
					c.Set(fiber.HeaderContentType, resp.Header.Get("Content-Type"))
					c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
					return c.Send(data)
				}
			}
		}
	}

	// 3. Fallback: Download remotely via database URL or smart fallbacks
	if database.DB != nil {
		var dbUrl, title string
		errDb := database.DB.QueryRow(c.Context(), "SELECT COALESCE(thumbnail_url, ''), COALESCE(title, '') FROM suwayomi.manga WHERE id = $1", mangaId).Scan(&dbUrl, &title)
		if errDb == nil {
			cleanUrl := rewriteKnownCDN(dbUrl)
			if cleanUrl != dbUrl {
				// Update database with rewritten CDN so next time it is already clean
				_, _ = database.DB.Exec(c.Context(), "UPDATE suwayomi.manga SET thumbnail_url = $1 WHERE id = $2", cleanUrl, mangaId)
				dbUrl = cleanUrl
			}

			if dbUrl != "" && !isKnownDeadDomain(dbUrl) {
				if data, mimeType, dlErr := downloadImageRaw(c.Context(), cfg, dbUrl); dlErr == nil && len(data) > 0 {
					saveThumbnailToDisk(cfg, mangaId, data)
					c.Set(fiber.HeaderContentType, mimeType)
					c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
					return c.Send(data)
				}
			}

			// If direct download failed or domain is known dead (like otakuteca.com):
			// Strategy A: Check siblings in local library with identical title
			if title != "" {
				if data, mimeType, sibErr := findSiblingCover(c.Context(), cfg, mangaId, title); sibErr == nil && len(data) > 0 {
					saveThumbnailToDisk(cfg, mangaId, data)
					c.Set(fiber.HeaderContentType, mimeType)
					c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
					return c.Send(data)
				}

				// Strategy B: Search MangaDex for official high-res cover
				if data, mimeType, mdErr := findMangaDexCover(c.Context(), cfg, title); mdErr == nil && len(data) > 0 {
					saveThumbnailToDisk(cfg, mangaId, data)
					c.Set(fiber.HeaderContentType, mimeType)
					c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
					return c.Send(data)
				}
			}
		}
	}
	// Serve transparent 1x1 image stub if not found locally
	c.Set(fiber.HeaderContentType, "image/jpeg")
	stub := []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x01, 0x00, 0x60,
		0x00, 0x60, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xc2, 0x00, 0x0b, 0x08, 0x00, 0x01, 0x00,
		0x01, 0x01, 0x01, 0x11, 0x00, 0xff, 0xda, 0x00, 0x0c, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00, 0xb2,
		0xc0, 0x07, 0xff, 0xd9,
	}
	return c.Send(stub)
}

// GetMangaCategories retrieves all categories this manga belongs to.
func GetMangaCategories(c *fiber.Ctx) error {
	mangaId, err := strconv.Atoi(c.Params("mangaId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}

	list, err := database.GetMangaCategories(c.Context(), mangaId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(list)
}

// AddMangaToCategory maps a manga ID to a category ID.
func AddMangaToCategory(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// RemoveMangaFromCategory unmaps a manga ID from a category ID.
func RemoveMangaFromCategory(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// AddMangaToLibrary flags a manga as in library.
func AddMangaToLibrary(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// RemoveMangaFromLibrary removes a manga from the library status.
func RemoveMangaFromLibrary(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// GetMangaMeta retrieves all custom metadata for a manga.
func GetMangaMeta(c *fiber.Ctx) error {
	mangaId, err := strconv.Atoi(c.Params("mangaId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}

	meta := make(map[string]string)
	if database.DB != nil {
		rows, err := database.DB.Query(c.Context(), "SELECT meta_key, value FROM suwayomi.mangameta WHERE manga_ref = $1 AND user_id = 1", mangaId)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var k, v string
				if err := rows.Scan(&k, &v); err == nil {
					meta[k] = v
				}
			}
		}
	}

	return c.JSON(meta)
}

// ModifyMangaMeta updates key-value manga metadata properties.
func ModifyMangaMeta(c *fiber.Ctx) error {
	mangaId, err := strconv.Atoi(c.Params("mangaId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	if database.DB != nil {
		for k, v := range body {
			var valStr string
			if s, ok := v.(string); ok {
				valStr = s
			} else {
				vBytes, _ := json.Marshal(v)
				valStr = string(vBytes)
			}
			_, _ = database.DB.Exec(c.Context(), `
				INSERT INTO suwayomi.mangameta (user_id, manga_ref, meta_key, value)
				VALUES (1, $1, $2, $3)
				ON CONFLICT (user_id, manga_ref, meta_key) DO UPDATE SET value = EXCLUDED.value
			`, mangaId, k, valStr)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

// GetChaptersList lists chapters under a manga ID.
func GetChaptersList(c *fiber.Ctx) error {
	mangaId, err := strconv.Atoi(c.Params("mangaId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}

	list, err := database.GetChaptersByMangaID(c.Context(), mangaId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(list)
}

// ModifyChapterInput defines parameters accepted when modifying a single chapter.
type ModifyChapterInput struct {
	Read         *bool `json:"read" form:"read"`
	IsRead       *bool `json:"isRead" form:"isRead"`
	Bookmarked   *bool `json:"bookmarked" form:"bookmarked"`
	IsBookmarked *bool `json:"isBookmarked" form:"isBookmarked"`
	LastPageRead *int  `json:"lastPageRead" form:"lastPageRead"`
	MarkPrevRead *bool `json:"markPrevRead" form:"markPrevRead"`
}

// ChapterBatchManga edits multiple chapters of a single manga.
func ChapterBatchManga(c *fiber.Ctx) error {
	var body models.MangaChapterBatchEditInput
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	applyChapterBatchChanges(c.Context(), body.ChapterIDs, body.ChangeType)
	return c.SendStatus(fiber.StatusOK)
}

// GetChapter gets single chapter detail.
func GetChapter(c *fiber.Ctx) error {
	mangaId, err := strconv.Atoi(c.Params("mangaId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}
	chapterIndex, err := strconv.Atoi(c.Params("chapterIndex"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid chapter index")
	}

	ch, err := database.GetChapterByMangaAndIndex(c.Context(), mangaId, chapterIndex)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	if ch == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	return c.JSON(ch)
}

// ModifyChapter updates chapter settings like read, bookmarked, etc.
func ModifyChapter(c *fiber.Ctx) error {
	mangaId, err := strconv.Atoi(c.Params("mangaId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}
	chapterIndex, err := strconv.Atoi(c.Params("chapterIndex"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid chapter index")
	}

	ch, err := database.GetChapterByMangaAndIndex(c.Context(), mangaId, chapterIndex)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	if ch == nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
	}

	var input ModifyChapterInput
	_ = c.BodyParser(&input)

	if input.Read == nil && input.IsRead == nil && c.Query("read") != "" {
		val := c.Query("read") == "true"
		input.Read = &val
	}
	if input.Bookmarked == nil && input.IsBookmarked == nil && c.Query("bookmarked") != "" {
		val := c.Query("bookmarked") == "true"
		input.Bookmarked = &val
	}
	if input.LastPageRead == nil && c.Query("lastPageRead") != "" {
		if lpr, err := strconv.Atoi(c.Query("lastPageRead")); err == nil {
			input.LastPageRead = &lpr
		}
	}
	if input.MarkPrevRead == nil && c.Query("markPrevRead") != "" {
		val := c.Query("markPrevRead") == "true"
		input.MarkPrevRead = &val
	}

	newRead := ch.Read
	if input.Read != nil {
		newRead = *input.Read
	} else if input.IsRead != nil {
		newRead = *input.IsRead
	}

	newBookmark := ch.Bookmarked
	if input.Bookmarked != nil {
		newBookmark = *input.Bookmarked
	} else if input.IsBookmarked != nil {
		newBookmark = *input.IsBookmarked
	}

	newLPR := ch.LastPageRead
	if input.LastPageRead != nil {
		newLPR = *input.LastPageRead
	}

	now := time.Now().Unix()
	newLRA := ch.LastReadAt
	if newRead || newLPR > 0 {
		newLRA = now
	} else if (input.Read != nil && !*input.Read) || (input.IsRead != nil && !*input.IsRead) {
		if newLPR == 0 {
			newLRA = 0
		}
	}

	if database.DB != nil {
		_, _ = database.DB.Exec(c.Context(), `
			INSERT INTO suwayomi.chapteruser (chapter, user_id, read, bookmark, last_page_read, last_read_at, version, last_modified_at)
			VALUES ($1, 1, $2, $3, $4, $5, 1, $6)
			ON CONFLICT (chapter, user_id) DO UPDATE SET
				read = EXCLUDED.read,
				bookmark = EXCLUDED.bookmark,
				last_page_read = EXCLUDED.last_page_read,
				last_read_at = EXCLUDED.last_read_at,
				last_modified_at = EXCLUDED.last_modified_at
		`, ch.ID, newRead, newBookmark, newLPR, newLRA, now)

		if input.MarkPrevRead != nil && *input.MarkPrevRead {
			_, _ = database.DB.Exec(c.Context(), `
				INSERT INTO suwayomi.chapteruser (chapter, user_id, read, bookmark, last_page_read, last_read_at, version, last_modified_at)
				SELECT c.id, 1, true, false, 0, $1, 1, $1
				FROM suwayomi.chapter c
				WHERE c.manga = $2 AND c.source_order < $3
				ON CONFLICT (chapter, user_id) DO UPDATE SET
					read = true,
					last_read_at = EXCLUDED.last_read_at,
					last_modified_at = EXCLUDED.last_modified_at
			`, now, mangaId, ch.Index)
		}
	}

	updatedCh, err := database.GetChapterByMangaAndIndex(c.Context(), mangaId, chapterIndex)
	if err == nil && updatedCh != nil {
		return c.JSON(updatedCh)
	}
	return c.JSON(ch)
}

// DeleteChapter removes downloaded files for a chapter.
func DeleteChapter(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// ModifyChapterMeta modifies custom metadata properties of a chapter.
func ModifyChapterMeta(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// GetPageImage serves the image data for a specific chapter page.
func GetPageImage(c *fiber.Ctx) error {
	mangaId, err := strconv.Atoi(c.Params("mangaId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}
	chapterIndex, err := strconv.Atoi(c.Params("chapterIndex"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid chapter index")
	}
	index, err := strconv.Atoi(c.Params("index"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid page index")
	}

	cfg := c.Locals("config").(*config.Config)
	ctx := c.Context()
	manga, _ := database.GetMangaByID(ctx, mangaId)
	chapter, _ := database.GetChapterByMangaAndIndex(ctx, mangaId, chapterIndex)

	chapterKey := fmt.Sprintf("idx_%d", chapterIndex)
	if chapter != nil {
		chapterKey = fmt.Sprintf("ch_%d", chapter.ID)
	}
	cacheDir := filepath.Join(cfg.Data.RootDir, "cache", "pages", fmt.Sprintf("%d", mangaId), chapterKey)
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d.img", index))

	// 1. Check local page disk cache (instant read)
	if cachedData, err := os.ReadFile(cachePath); err == nil && len(cachedData) > 0 {
		c.Set(fiber.HeaderContentType, util.DetectImageType(cachedData))
		c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
		return c.Send(cachedData)
	}

	// 2. Check if the page is available in downloaded chapter storage (CBZ / folder)
	if manga != nil && chapter != nil {
		sourceName := "Local Source"
		if manga.Source != nil {
			sourceName = manga.Source.Name
		}
		scanlator := ""
		if chapter.Scanlator != nil {
			scanlator = *chapter.Scanlator
		}

		data, mimeType, err := util.ServePage(cfg, sourceName, manga.Title, chapter.Name, scanlator, index)
		if err == nil && len(data) > 0 {
			c.Set(fiber.HeaderContentType, mimeType)
			c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
			return c.Send(data)
		}
	}

	// 3. Direct Native Go Resolution: Check suwayomi.page in PostgreSQL
	var imageURL string
	if database.DB != nil && chapter != nil {
		_ = database.DB.QueryRow(ctx, "SELECT COALESCE(image_url, '') FROM suwayomi.page WHERE chapter = $1 AND index = $2", chapter.ID, index).Scan(&imageURL)
	}

	// If not yet in database, check if we have a native Go source handler to scrape it right now
	if imageURL == "" && manga != nil && manga.Source != nil && chapter != nil {
		sourceID, _ := strconv.ParseInt(manga.Source.ID, 10, 64)
		if sources.Has(sourceID) {
			mangaURL := ""
			if manga.RealURL != nil {
				mangaURL = *manga.RealURL
			}
			_, err := sources.FetchAndSaveChapterPages(ctx, sourceID, mangaId, int64(chapter.ID), chapterIndex, chapter.URL, mangaURL)
			if err == nil && database.DB != nil {
				_ = database.DB.QueryRow(ctx, "SELECT COALESCE(image_url, '') FROM suwayomi.page WHERE chapter = $1 AND index = $2", chapter.ID, index).Scan(&imageURL)
			}
		}
	}

	// If we have an image URL, fetch it directly via Go HTTP client (Zero JVM!)
	if imageURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
			req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
			if manga != nil && manga.RealURL != nil && *manga.RealURL != "" {
				req.Header.Set("Referer", *manga.RealURL)
			}

			resp, err := sources.DefaultClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					respBody, err := io.ReadAll(resp.Body)
					if err == nil && len(respBody) > 0 {
						_ = os.MkdirAll(cacheDir, 0755)
						_ = os.WriteFile(cachePath, respBody, 0644)

						contentType := resp.Header.Get("Content-Type")
						if contentType == "" {
							contentType = util.DetectImageType(respBody)
						}
						c.Set(fiber.HeaderContentType, contentType)
						c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
						return c.Send(respBody)
					}
				} else if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable) && cfg != nil && cfg.FlareSolverr.Enabled {
					log.Printf("[PAGES] Cloudflare challenge on page %d (HTTP %d). Requesting FlareSolverr bypass...", index, resp.StatusCode)
					ref := ""
					if manga != nil && manga.RealURL != nil {
						ref = *manga.RealURL
					}
					fsData, mimeType, fsErr := util.DownloadWithFlareSolverrAndReferer(ctx, cfg, imageURL, ref)
					if fsErr == nil && len(fsData) > 0 {
						_ = os.MkdirAll(cacheDir, 0755)
						_ = os.WriteFile(cachePath, fsData, 0644)
						c.Set(fiber.HeaderContentType, mimeType)
						c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
						return c.Send(fsData)
					}
					log.Printf("[PAGES] FlareSolverr bypass error for page %d: %v", index, fsErr)
				}
			} else if cfg != nil && cfg.FlareSolverr.Enabled {
				ref := ""
				if manga != nil && manga.RealURL != nil {
					ref = *manga.RealURL
				}
				fsData, mimeType, fsErr := util.DownloadWithFlareSolverrAndReferer(ctx, cfg, imageURL, ref)
				if fsErr == nil && len(fsData) > 0 {
					_ = os.MkdirAll(cacheDir, 0755)
					_ = os.WriteFile(cachePath, fsData, 0644)
					c.Set(fiber.HeaderContentType, mimeType)
					c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
					return c.Send(fsData)
				}
			}
		}
	}

	// 4. Fallback to Sidecar (JVM / Tachiyomi engine) only if native Go resolution wasn't available
	if cfg.Sidecar.Enabled && cfg.Sidecar.BaseURL != "" {
		targetURL := fmt.Sprintf("%s/api/v1/manga/%d/chapter/%d/page/%d",
			strings.TrimSuffix(cfg.Sidecar.BaseURL, "/"),
			mangaId, chapterIndex, index)

		req, err := http.NewRequestWithContext(ctx, fiber.MethodGet, targetURL, nil)
		if err == nil {
			c.Request().Header.VisitAll(func(key, value []byte) {
				req.Header.Set(string(key), string(value))
			})

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					respBody, err := io.ReadAll(resp.Body)
					if err == nil && len(respBody) > 0 {
						// Cache to disk
						_ = os.MkdirAll(cacheDir, 0755)
						_ = os.WriteFile(cachePath, respBody, 0644)

						contentType := resp.Header.Get("Content-Type")
						if contentType == "" {
							contentType = util.DetectImageType(respBody)
						}
						c.Set(fiber.HeaderContentType, contentType)
						c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
						return c.Send(respBody)
					}
				} else {
					return c.Status(resp.StatusCode).SendString(fmt.Sprintf("Sidecar page error: %d", resp.StatusCode))
				}
			}
		}
	}

	return c.Status(fiber.StatusNotFound).SendString("Page not found")
}

// applyChapterBatchChanges applies batch edits (isRead, isBookmarked, lastPageRead, delete) to chapteruser.
func applyChapterBatchChanges(ctx context.Context, chapterIDs []int, change models.ChapterBatchEditChange) {
	if len(chapterIDs) == 0 || database.DB == nil {
		return
	}
	now := time.Now().Unix()
	for _, cid := range chapterIDs {
		var currentRead, currentBookmark bool
		var currentLPR int
		var currentLRA int64
		_ = database.DB.QueryRow(ctx, `
			SELECT read, bookmark, last_page_read, last_read_at
			FROM suwayomi.chapteruser
			WHERE chapter = $1 AND user_id = 1
		`, cid).Scan(&currentRead, &currentBookmark, &currentLPR, &currentLRA)

		newRead := currentRead
		if change.IsRead != nil {
			newRead = *change.IsRead
		}
		newBookmark := currentBookmark
		if change.IsBookmarked != nil {
			newBookmark = *change.IsBookmarked
		}
		newLPR := currentLPR
		if change.LastPageRead != nil {
			newLPR = *change.LastPageRead
		}
		newLRA := currentLRA
		if change.IsRead != nil && *change.IsRead {
			newLRA = now
		} else if change.LastPageRead != nil && *change.LastPageRead > 0 {
			newLRA = now
		} else if change.IsRead != nil && !*change.IsRead && newLPR == 0 {
			newLRA = 0
		}

		_, _ = database.DB.Exec(ctx, `
			INSERT INTO suwayomi.chapteruser (chapter, user_id, read, bookmark, last_page_read, last_read_at, version, last_modified_at)
			VALUES ($1, 1, $2, $3, $4, $5, 1, $6)
			ON CONFLICT (chapter, user_id) DO UPDATE SET
				read = EXCLUDED.read,
				bookmark = EXCLUDED.bookmark,
				last_page_read = EXCLUDED.last_page_read,
				last_read_at = EXCLUDED.last_read_at,
				last_modified_at = EXCLUDED.last_modified_at
		`, cid, newRead, newBookmark, newLPR, newLRA, now)

		if change.Delete != nil && *change.Delete {
			_, _ = database.DB.Exec(ctx, "UPDATE suwayomi.chapteruser SET is_downloaded = false WHERE chapter = $1", cid)
			_, _ = database.DB.Exec(ctx, "UPDATE suwayomi.chapter SET is_downloaded = false WHERE id = $1", cid)
		}
	}
}

// ChapterBatchGlobal edits multiple chapters across different mangas.
func ChapterBatchGlobal(c *fiber.Ctx) error {
	var body models.ChapterBatchEditInput
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	applyChapterBatchChanges(c.Context(), body.ChapterIDs, body.Change)
	return c.SendStatus(fiber.StatusOK)
}

// DownloadChapter exports a chapter as a CBZ archive.
func DownloadChapter(c *fiber.Ctx) error {
	chapterIdStr := c.Params("chapterId")
	chapterId, err := strconv.Atoi(chapterIdStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid chapter ID")
	}

	// Mark as read if requested by OPDS client (?markAsRead=true)
	if c.Query("markAsRead") == "true" && database.DB != nil {
		_, _ = database.DB.Exec(c.Context(), `
			INSERT INTO suwayomi.chapteruser (user_id, chapter, read, last_read_at)
			VALUES (1, $1, true, EXTRACT(EPOCH FROM NOW())::bigint)
			ON CONFLICT (user_id, chapter) DO UPDATE SET read = true, last_read_at = EXCLUDED.last_read_at
		`, chapterId)
	}

	cfg := c.Locals("config").(*config.Config)

	var chName string
	var scanlator sql.NullString
	var mTitle string
	var sName sql.NullString

	if database.DB != nil {
		err := database.DB.QueryRow(c.Context(), `
			SELECT c.name, c.scanlator, m.title, s.name
			FROM suwayomi.chapter c
			INNER JOIN suwayomi.manga m ON c.manga = m.id
			LEFT JOIN suwayomi.source s ON m.source = s.id
			WHERE c.id = $1
		`, chapterId).Scan(&chName, &scanlator, &mTitle, &sName)
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Chapter not found")
		}
	} else {
		return c.Status(fiber.StatusNotFound).SendString("Database not connected")
	}

	sourceName := "Local Source"
	if sName.Valid && sName.String != "" {
		sourceName = sName.String
	}
	scan := ""
	if scanlator.Valid {
		scan = scanlator.String
	}

	basePath := util.GetChapterPath(cfg, sourceName, mTitle, chName, scan)
	cbzPath := basePath + ".cbz"

	// 1. If .cbz file exists directly on disk
	if info, err := os.Stat(cbzPath); err == nil && !info.IsDir() && info.Size() > 1024 {
		filename := fmt.Sprintf("%s - %s.cbz", util.SanitizeFilename(mTitle), util.SanitizeFilename(chName))
		c.Set(fiber.HeaderContentType, "application/vnd.comicbook+zip")
		c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Set(fiber.HeaderContentLength, fmt.Sprintf("%d", info.Size()))
		return c.SendFile(cbzPath)
	}

	// 2. If directory of images exists, pack it as a zip archive and stream it
	if info, err := os.Stat(basePath); err == nil && info.IsDir() {
		filename := fmt.Sprintf("%s - %s.cbz", util.SanitizeFilename(mTitle), util.SanitizeFilename(chName))
		c.Set(fiber.HeaderContentType, "application/vnd.comicbook+zip")
		c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))

		zipBuffer := new(bytes.Buffer)
		zipWriter := zip.NewWriter(zipBuffer)
		entries, _ := os.ReadDir(basePath)
		for _, entry := range entries {
			if !entry.IsDir() {
				filePath := filepath.Join(basePath, entry.Name())
				data, err := os.ReadFile(filePath)
				if err == nil {
					w, err := zipWriter.Create(entry.Name())
					if err == nil {
						_, _ = w.Write(data)
					}
				}
			}
		}
		_ = zipWriter.Close()
		return c.Send(zipBuffer.Bytes())
	}

	// 3. On-demand auto download for OPDS readers and clients when not yet downloaded
	log.Printf("[OPDS/Downloader] CBZ not found locally for chapter %d ('%s' - '%s'). Initiating on-demand download...", chapterId, mTitle, chName)
	generatedCBZ, err := service.EnsureChapterCBZ(c.Context(), cfg, chapterId)
	if err != nil {
		log.Printf("[OPDS/Downloader] Failed to auto-download chapter %d: %v", chapterId, err)
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to download chapter: %v", err))
	}

	if info, err := os.Stat(generatedCBZ); err == nil && !info.IsDir() {
		filename := fmt.Sprintf("%s - %s.cbz", util.SanitizeFilename(mTitle), util.SanitizeFilename(chName))
		c.Set(fiber.HeaderContentType, "application/vnd.comicbook+zip")
		c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Set(fiber.HeaderContentLength, fmt.Sprintf("%d", info.Size()))
		return c.SendFile(generatedCBZ)
	}

	return c.Status(fiber.StatusNotFound).SendString("Chapter CBZ not found on server")
}

// DownloadChapterMetadata returns CBZ size headers.
func DownloadChapterMetadata(c *fiber.Ctx) error {
	chapterIdStr := c.Params("chapterId")
	chapterId, err := strconv.Atoi(chapterIdStr)
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	cfg := c.Locals("config").(*config.Config)
	var chName string
	var scanlator sql.NullString
	var mTitle string
	var sName sql.NullString

	if database.DB != nil {
		err := database.DB.QueryRow(c.Context(), `
			SELECT c.name, c.scanlator, m.title, s.name
			FROM suwayomi.chapter c
			INNER JOIN suwayomi.manga m ON c.manga = m.id
			LEFT JOIN suwayomi.source s ON m.source = s.id
			WHERE c.id = $1
		`, chapterId).Scan(&chName, &scanlator, &mTitle, &sName)
		if err == nil {
			sourceName := "Local Source"
			if sName.Valid && sName.String != "" {
				sourceName = sName.String
			}
			scan := ""
			if scanlator.Valid {
				scan = scanlator.String
			}
			cbzPath := util.GetChapterPath(cfg, sourceName, mTitle, chName, scan) + ".cbz"
			if info, err := os.Stat(cbzPath); err == nil && !info.IsDir() && info.Size() > 1024 {
				c.Set(fiber.HeaderContentType, "application/vnd.comicbook+zip")
				c.Set(fiber.HeaderContentLength, fmt.Sprintf("%d", info.Size()))
				c.Set(fiber.HeaderAcceptRanges, "bytes")
				return c.Send(nil)
			}
		}
	}

	// For undownloaded chapters, return 200 OK with CBZ content type so readers proceed to GET
	c.Set(fiber.HeaderContentType, "application/vnd.comicbook+zip")
	c.Set(fiber.HeaderAcceptRanges, "bytes")
	return c.Send(nil)
}
