package sources

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/util"
)

var AppConfig *config.Config

// SetConfig stores the configuration for source operations.
func SetConfig(cfg *config.Config) {
	AppConfig = cfg
}

// DefaultClient is an optimized HTTP client for source scraping with realistic browser headers.
var DefaultClient = &http.Client{
	Timeout: 25 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// ConfigureProxy inspects database settings for SOCKS5 proxy configuration and updates DefaultClient transport.
func ConfigureProxy(ctx context.Context) {
	settings, err := database.GetAllServerSettings(ctx)
	if err != nil || settings == nil {
		return
	}

	enabled := settings["socksProxyEnabled"] == "true"
	host := strings.TrimSpace(settings["socksProxyHost"])
	port := strings.TrimSpace(settings["socksProxyPort"])
	user := strings.TrimSpace(settings["socksProxyUsername"])
	pass := strings.TrimSpace(settings["socksProxyPassword"])

	baseTransport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	if !enabled || host == "" || port == "" {
		DefaultClient.Transport = baseTransport
		return
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	var auth *proxy.Auth
	if user != "" || pass != "" {
		auth = &proxy.Auth{
			User:     user,
			Password: pass,
		}
	}

	dialer, dialErr := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
	if dialErr != nil {
		log.Printf("[PROXY] Failed to create SOCKS5 dialer for %s: %v", addr, dialErr)
		DefaultClient.Transport = baseTransport
		return
	}

	baseTransport.Dial = dialer.Dial
	DefaultClient.Transport = baseTransport
	log.Printf("[PROXY] Configured SOCKS5 proxy at %s", addr)
}

// ChapterItem represents a chapter's metadata scraped from a source.
type ChapterItem struct {
	Name          string
	URL           string
	RealURL       string
	ChapterNumber float32
	Scanlator     string
	DateUpload    int64
	SourceOrder   int
}

// MangaItem represents a manga's metadata scraped from a source for catalog browsing or searching.
type MangaItem struct {
	ID           string
	Title        string
	Description  string
	Status       int // 1: ongoing, 2: completed, 5: cancelled, 6: hiatus
	ThumbnailURL string
	URL          string
	SourceID     int64
	Genres       []string
	Author       string
	Artist       string
}

// MangaDetailsFetcher can be implemented by sources that support refreshing detailed metadata.
type MangaDetailsFetcher interface {
	GetMangaDetails(ctx context.Context, mangaURL string) (*MangaItem, error)
}

// MangaSearcher is an optional interface for sources that support searching manga online.
type MangaSearcher interface {
	SearchManga(ctx context.Context, query string, page int) ([]MangaItem, bool, error)
}

// PopularLister is an optional interface for sources that support listing popular manga online.
type PopularLister interface {
	GetPopularManga(ctx context.Context, page int) ([]MangaItem, bool, error)
}

// LatestLister is an optional interface for sources that support listing latest updated manga online.
type LatestLister interface {
	GetLatestManga(ctx context.Context, page int) ([]MangaItem, bool, error)
}

// ChapterLister is an optional interface implemented by SourceHandlers that support fetching chapter lists natively in Go.
type ChapterLister interface {
	GetChapterList(ctx context.Context, mangaURL string) ([]ChapterItem, error)
}

// SourceHandler interface for extracting manga and chapter data from a source.
type SourceHandler interface {
	Name() string
	GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error)
}

var (
	registryMu sync.RWMutex
	registry   = make(map[int64]SourceHandler)
	allNative  = make(map[int64]SourceHandler)
)

// Register associates a source ID with its native Go handler.
func Register(sourceID int64, handler SourceHandler) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[sourceID] = handler
	allNative[sourceID] = handler
}

// Unregister removes a source ID from the active registry.
func Unregister(sourceID int64) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(registry, sourceID)
}

// RestoreNative restores a compiled native source handler to the active registry if it was previously registered.
func RestoreNative(sourceID int64) bool {
	registryMu.Lock()
	defer registryMu.Unlock()
	if h, ok := allNative[sourceID]; ok {
		registry[sourceID] = h
		return true
	}
	return false
}

// Get returns the native Go handler for a given source ID if one exists.
func Get(sourceID int64) (SourceHandler, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	h, ok := registry[sourceID]
	return h, ok
}

// Has returns true if a native Go handler is registered for sourceID.
func Has(sourceID int64) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[sourceID]
	return ok
}

// FetchHTML retrieves the body of a URL with desktop browser headers.
func FetchHTML(ctx context.Context, targetURL string, referer string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "es-419,es;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	domain := util.ExtractDomain(targetURL)
	if domain != "" && AppConfig != nil && AppConfig.FlareSolverr.Enabled {
		if cookies, ua, ok := util.GetSession(domain); ok && len(cookies) > 0 {
			for _, c := range cookies {
				req.AddCookie(&http.Cookie{
					Name:   c.Name,
					Value:  c.Value,
					Domain: c.Domain,
					Path:   c.Path,
				})
			}
			if ua != "" {
				req.Header.Set("User-Agent", ua)
			}
		}
	}

	resp, err := DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	bodyStr := string(body)

	isChallenge := resp.StatusCode == http.StatusForbidden ||
		resp.StatusCode == http.StatusServiceUnavailable ||
		(resp.StatusCode == http.StatusOK && (
			strings.Contains(bodyStr, "Just a moment...") ||
			strings.Contains(bodyStr, "challenges.cloudflare.com") ||
			strings.Contains(bodyStr, "_cf_chl_opt") ||
			strings.Contains(bodyStr, "cf-browser-verification") ||
			strings.Contains(bodyStr, "challenge-running")))

	if isChallenge && AppConfig != nil && AppConfig.FlareSolverr.Enabled {
		log.Printf("[SOURCES] Cloudflare challenge encountered for %s (HTTP %d). Requesting FlareSolverr bypass...", targetURL, resp.StatusCode)
		if domain != "" {
			util.InvalidateSession(domain)
		}
		html, solveErr := util.SolveAndFetchHTML(ctx, AppConfig, targetURL, referer)
		if solveErr == nil && len(html) > 0 {
			return html, nil
		}
		log.Printf("[SOURCES] FlareSolverr challenge bypass warning: %v", solveErr)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP error %d fetching %s", resp.StatusCode, targetURL)
	}

	return bodyStr, nil
}

// FetchAndSaveChapterPages extracts pages natively in Go and persists them to the PostgreSQL database.
func FetchAndSaveChapterPages(ctx context.Context, sourceID int64, mangaID int, chapterID int64, chapterIndex int, chapterURL string, mangaURL string) ([]string, error) {
	handler, ok := Get(sourceID)
	if !ok {
		return nil, fmt.Errorf("no native Go handler registered for source ID %d", sourceID)
	}

	log.Printf("[SOURCES] Fetching pages for chapter %d (manga %d) via native handler %q...", chapterID, mangaID, handler.Name())
	imageURLs, err := handler.GetPageList(ctx, chapterURL, mangaURL)
	if err != nil {
		return nil, fmt.Errorf("handler %s failed: %w", handler.Name(), err)
	}

	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("handler %s returned 0 pages for chapter %d", handler.Name(), chapterID)
	}

	// Persist to suwayomi.page in PostgreSQL
	if database.DB != nil {
		tx, err := database.DB.Begin(ctx)
		if err == nil {
			// Clear existing pages for this chapter
			_, _ = tx.Exec(ctx, "DELETE FROM suwayomi.page WHERE chapter = $1", chapterID)

			// Insert all pages
			for i, imgURL := range imageURLs {
				_, _ = tx.Exec(ctx,
					"INSERT INTO suwayomi.page (index, url, image_url, chapter) VALUES ($1, $2, $3, $4)",
					i, "", imgURL, chapterID,
				)
			}

			// Update chapter page count
			_, _ = tx.Exec(ctx,
				"UPDATE suwayomi.chapter SET page_count = $1 WHERE id = $2",
				len(imageURLs), chapterID,
			)

			_ = tx.Commit(ctx)
			log.Printf("[SOURCES] Successfully saved %d pages for chapter %d to database", len(imageURLs), chapterID)
		}
	}

	// Return deterministic API page paths for the frontend
	pages := make([]string, len(imageURLs))
	for i := range imageURLs {
		pages[i] = fmt.Sprintf("/api/v1/manga/%d/chapter/%d/page/%d?v=%d", mangaID, chapterIndex, i, chapterID)
	}

	return pages, nil
}

// FetchAndSaveMangaChapters fetches chapters via a registered native Go source handler and persists them to the PostgreSQL database.
func FetchAndSaveMangaChapters(ctx context.Context, sourceID int64, mangaID int64, mangaURL string) ([]ChapterItem, error) {
	handler, ok := Get(sourceID)
	if !ok {
		return nil, fmt.Errorf("no native Go handler registered for source ID %d", sourceID)
	}

	lister, ok := handler.(ChapterLister)
	if !ok {
		return nil, fmt.Errorf("handler %s does not support chapter listing", handler.Name())
	}

	log.Printf("[SOURCES] Fetching chapter list for manga %d via native handler %q...", mangaID, handler.Name())
	chapters, err := lister.GetChapterList(ctx, mangaURL)
	if err != nil {
		return nil, fmt.Errorf("handler %s failed to get chapters: %w", handler.Name(), err)
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("handler %s returned 0 chapters for manga %d", handler.Name(), mangaID)
	}

	// Sanity check for chapter ordering:
	// In Suwayomi/Tachidesk, source_order MUST increase chronologically (oldest = 1, newest = total).
	// If the source returned chapters where the first item has a higher ChapterNumber than the last item,
	// but the first item was given a smaller SourceOrder than the last item, the source orders are inverted!
	if len(chapters) > 1 {
		firstNum := chapters[0].ChapterNumber
		lastNum := chapters[len(chapters)-1].ChapterNumber
		firstOrder := chapters[0].SourceOrder
		lastOrder := chapters[len(chapters)-1].SourceOrder

		if firstNum > 0 && lastNum > 0 && firstNum > lastNum && firstOrder < lastOrder {
			log.Printf("[SOURCES] Inverted SourceOrder detected for manga %d (first ch=%.1f order=%d, last ch=%.1f order=%d). Normalizing...",
				mangaID, firstNum, firstOrder, lastNum, lastOrder)
			total := len(chapters)
			for i := range chapters {
				chapters[i].SourceOrder = total - i
			}
		}
	}

	// Ensure source_order is strictly 1-based (minimum >= 1)
	if len(chapters) > 0 {
		minOrder := chapters[0].SourceOrder
		for _, ch := range chapters {
			if ch.SourceOrder < minOrder {
				minOrder = ch.SourceOrder
			}
		}
		if minOrder <= 0 {
			shift := 1 - minOrder
			for i := range chapters {
				chapters[i].SourceOrder += shift
			}
		}
	}

	if database.DB != nil {
		nowSec := time.Now().Unix()
		tx, err := database.DB.Begin(ctx)
		if err == nil {
			for _, ch := range chapters {
				realURL := ch.RealURL
				if realURL == "" {
					realURL = ch.URL
				}
				var chapterID int64
				err := tx.QueryRow(ctx, `
					INSERT INTO suwayomi.chapter (url, name, date_upload, chapter_number, scanlator, source_order, manga, is_downloaded, page_count, fetched_at, real_url)
					VALUES ($1, $2, $3, $4, $5, $6, $7, false, -1, $8, $9)
					ON CONFLICT (url, manga) DO UPDATE SET 
						name = EXCLUDED.name,
						chapter_number = EXCLUDED.chapter_number,
						scanlator = EXCLUDED.scanlator,
						source_order = EXCLUDED.source_order,
						date_upload = EXCLUDED.date_upload,
						real_url = EXCLUDED.real_url
					RETURNING id
				`, ch.URL, ch.Name, ch.DateUpload, ch.ChapterNumber, ch.Scanlator, ch.SourceOrder, mangaID, nowSec, realURL).Scan(&chapterID)

				if err == nil {
					_, _ = tx.Exec(ctx, `
						INSERT INTO suwayomi.chapteruser (chapter, user_id, read, bookmark, last_page_read, last_read_at, is_downloaded, is_download_requested, last_modified_at)
						VALUES ($1, 1, false, false, 0, 0, false, false, $2)
						ON CONFLICT (chapter, user_id) DO NOTHING
					`, chapterID, nowSec)
				}
			}

			// Mark manga as initialized and save real_url if empty
			mRealURL := GetMangaURL(sourceID, mangaURL)
			if mRealURL != "" {
				_, _ = tx.Exec(ctx, `
					UPDATE suwayomi.manga SET initialized = true, last_fetched_at = $1, chapters_last_fetched_at = $1, real_url = COALESCE(NULLIF(real_url, ''), $3) WHERE id = $2
				`, nowSec, mangaID, mRealURL)
			} else {
				_, _ = tx.Exec(ctx, `
					UPDATE suwayomi.manga SET initialized = true, last_fetched_at = $1, chapters_last_fetched_at = $1 WHERE id = $2
				`, nowSec, mangaID)
			}

			if errCommit := tx.Commit(ctx); errCommit != nil {
				log.Printf("[SOURCES] Failed to commit chapters transaction for manga %d: %v", mangaID, errCommit)
			} else {
				log.Printf("[SOURCES] Successfully saved %d chapters for manga %d to database", len(chapters), mangaID)
			}
		}
	}

	return chapters, nil
}

var knownSourceNames = map[int64]string{
	7925520943983324764: "TuMangaOnline",
	2202687009511923782: "MangaOni",
	7178038133263703638: "LectorManga",
	2499283573021220255: "MangaDex (ES)",
	6400665728063187402: "MangaDex (EN)",
	4938773340256184018: "MangaDex (ALL)",
	5649269152264667286: "Shadow Manga",
	5943212552955476721: "ManhwaWeb",
	829053423099062956:  "Mangas No Sekai",
	4146344224513899730: "InManga",
	1598762687295904337: "BokugenTranslation",
	8701445043633550960: "LeerManhwas",
	3044113561993269522: "ApollComics",
	4647982373411241498: "DragonTranslation",
	1814471239661920836: "Cerberus Series",
	3548601371328353875: "Manhwa-Latino",
	8311505150576608267: "Hades no Fansub",
	6757330205319558057: "Bega Translation",
	3953107753690791431: "Uchuujin Projects",
	8385948079634749158: "Manhua Online",
	1136717301008230074: "Submanhwa",
	3053251231047124074: "NineAnime",
	6935004473807791201: "Plot Twist No Fansub",
	1348827658894665970: "Ragnarok Scanlation",
	1340622632921674920: "Shadow Manga (+18)",
	698913079501532785:  "RavenManga",
	4078570629457850125: "LectorXD",
	1326771034671033762: "Manhwas.net",
}

// GetSourceName returns a human-friendly name for a source ID.
func GetSourceName(sourceID int64) string {
	if h, ok := Get(sourceID); ok && h.Name() != "" {
		return h.Name()
	}
	if name, ok := knownSourceNames[sourceID]; ok {
		return name
	}
	return ""
}

// GetMangaURL returns the public web URL for a manga given its source ID and internal URL.
func GetMangaURL(sourceID int64, mangaURL string) string {
	cleanURL := strings.TrimSpace(mangaURL)
	if cleanURL == "" {
		return ""
	}
	if strings.HasPrefix(cleanURL, "http://") || strings.HasPrefix(cleanURL, "https://") {
		return cleanURL
	}

	switch sourceID {
	case 5649269152264667286, 1340622632921674920: // Shadow Manga, Shadow Manga (+18)
		if strings.HasPrefix(cleanURL, "ext/") {
			return "https://shademanga.com/adultos/manga/o/" + strings.TrimPrefix(cleanURL, "ext/")
		}
		if strings.HasPrefix(cleanURL, "adultos/manga/o/") {
			return "https://shademanga.com/" + cleanURL
		}
		if strings.HasPrefix(cleanURL, "serie/local/") {
			return "https://shademanga.com/" + cleanURL
		}
		return "https://shademanga.com/serie/local/" + strings.Trim(cleanURL, "/")

	case 2202687009511923782: // MangaOni
		return "https://manga-oni.com/" + strings.TrimLeft(cleanURL, "/")

	case 2499283573021220255, 6400665728063187402, 4938773340256184018: // MangaDex
		return "https://mangadex.org/title/" + strings.TrimLeft(cleanURL, "/")

	case 7925520943983324764: // TuMangaOnline
		if strings.HasPrefix(cleanURL, "/") {
			return "https://visortmo.com" + cleanURL
		}
		return "https://visortmo.com/" + cleanURL

	case 7178038133263703638: // LectorManga
		if strings.HasPrefix(cleanURL, "/") {
			return "https://lectormanga.com" + cleanURL
		}
		return "https://lectormanga.com/" + cleanURL

	case 3953107753690791431: // Uchuujin Projects
		return "https://uchuujinmangas.com/" + strings.TrimLeft(cleanURL, "/")

	case 1598762687295904337: // BokugenTranslation
		return "https://bokugents.com/" + strings.TrimLeft(cleanURL, "/")

	case 5943212552955476721: // ManhwaWeb
		return "https://manhwa-web.com/" + strings.TrimLeft(cleanURL, "/")

	case 1136717301008230074: // Submanhwa
		return "https://submanhwa.com/" + strings.TrimLeft(cleanURL, "/")

	case 829053423099062956: // Mangas No Sekai
		return "https://mangasnosekai.com/" + strings.TrimLeft(cleanURL, "/")

	case 698913079501532785: // RavenManga
		return "https://ravenmanga.com/" + strings.TrimLeft(cleanURL, "/")

	case 3548601371328353875: // Manhwa-Latino
		return "https://manhwalatino.org/" + strings.TrimLeft(cleanURL, "/")

	case 8311505150576608267: // Hades no Fansub
		return "https://lectorhades.latamtoon.com/" + strings.TrimLeft(cleanURL, "/")

	case 4078570629457850125: // LectorXD
		if strings.HasPrefix(cleanURL, "/") {
			return "https://lectorxd.com" + cleanURL
		}
		return "https://lectorxd.com/" + cleanURL

	case 1326771034671033762: // Manhwas.net
		if strings.HasPrefix(cleanURL, "/") {
			return "https://manhwa-es.com" + cleanURL
		}
		return "https://manhwa-es.com/" + cleanURL


	default:
		if strings.HasPrefix(cleanURL, "/") {
			return cleanURL
		}
		return cleanURL
	}
}

