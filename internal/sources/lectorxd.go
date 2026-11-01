package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// LectorXDSourceID is the canonical 64-bit Tachiyomi ID for LectorXD (es).
const LectorXDSourceID int64 = 4078570629457850125

func init() {
	Register(LectorXDSourceID, NewLectorXDHandler())
}

type LectorXDHandler struct {
	name    string
	baseURL string
}

func NewLectorXDHandler() *LectorXDHandler {
	return &LectorXDHandler{
		name:    "LectorXD",
		baseURL: "https://lectorxd.com",
	}
}

func (h *LectorXDHandler) Name() string {
	return h.name
}

// GetPopularManga retrieves popular manga from /catalogo?page=X
func (h *LectorXDHandler) GetPopularManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := fmt.Sprintf("%s/catalogo?page=%d", h.baseURL, page)
	return h.fetchMangaGrid(ctx, targetURL)
}

// GetLatestManga retrieves latest added/updated manga from /agregados-recientemente?page=X
func (h *LectorXDHandler) GetLatestManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := fmt.Sprintf("%s/agregados-recientemente?page=%d", h.baseURL, page)
	return h.fetchMangaGrid(ctx, targetURL)
}

func (h *LectorXDHandler) fetchMangaGrid(ctx context.Context, targetURL string) ([]MangaItem, bool, error) {
	html, err := FetchHTML(ctx, targetURL, h.baseURL+"/")
	if err != nil {
		return nil, false, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, false, err
	}

	var items []MangaItem
	seen := make(map[string]bool)

	doc.Find("article").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a[href]").First()
		href, _ := a.Attr("href")
		if href == "" {
			return
		}

		img := s.Find("img").First()
		title, _ := img.Attr("alt")
		if title == "" {
			title = strings.TrimSpace(s.Find("h3, h2, h4, .title").First().Text())
		}
		if title == "" {
			title = strings.TrimSpace(a.Text())
		}

		thumb, _ := img.Attr("src")
		if thumb == "" {
			thumb, _ = img.Attr("data-src")
		}

		mangaURL := href
		if strings.HasPrefix(mangaURL, h.baseURL) {
			mangaURL = strings.TrimPrefix(mangaURL, h.baseURL)
		}

		if seen[mangaURL] || title == "" {
			return
		}
		seen[mangaURL] = true

		items = append(items, MangaItem{
			Title:        title,
			URL:          mangaURL,
			ThumbnailURL: thumb,
			Status:       1,
			SourceID:     LectorXDSourceID,
		})
	})

	hasNext := len(items) >= 20
	return items, hasNext, nil
}

// SearchManga searches manga online via /api/search?q=...
func (h *LectorXDHandler) SearchManga(ctx context.Context, query string, page int) ([]MangaItem, bool, error) {
	if strings.TrimSpace(query) == "" {
		return h.GetPopularManga(ctx, page)
	}

	searchURL := fmt.Sprintf("%s/api/search?q=%s", h.baseURL, url.QueryEscape(query))
	rawJSON, err := FetchHTML(ctx, searchURL, h.baseURL+"/")
	if err != nil {
		return nil, false, err
	}

	type LectorSearchResult struct {
		Results []struct {
			ID         int    `json:"id"`
			Title      string `json:"title"`
			Slug       string `json:"slug"`
			CoverImage string `json:"coverImage"`
			Type       string `json:"type"`
		} `json:"results"`
	}

	var result LectorSearchResult
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return nil, false, fmt.Errorf("parsing LectorXD search response: %w", err)
	}

	var items []MangaItem
	for _, it := range result.Results {
		mType := it.Type
		if mType == "" {
			mType = "manhwa"
		}
		mangaURL := fmt.Sprintf("/%s/%s", mType, it.Slug)
		items = append(items, MangaItem{
			Title:        it.Title,
			URL:          mangaURL,
			ThumbnailURL: it.CoverImage,
			Status:       1,
			SourceID:     LectorXDSourceID,
		})
	}

	return items, false, nil
}

// GetChapterList retrieves all chapters by extracting the embedded chaptersList JSON array.
func (h *LectorXDHandler) GetChapterList(ctx context.Context, mangaURL string) ([]ChapterItem, error) {
	targetURL := mangaURL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = h.baseURL + "/" + strings.TrimLeft(mangaURL, "/")
	}

	html, err := FetchHTML(ctx, targetURL, h.baseURL+"/")
	if err != nil {
		return nil, err
	}

	// Extract slug, type, and chaptersList
	reSlug := regexp.MustCompile(`const\s+(?:mangaSlug|slug)\s*=\s*"([^"]+)"`)
	reType := regexp.MustCompile(`(?:const\s+mangaType\s*=\s*"([^"]+)"|"type"\s*:\s*"([^"]+)")`)
	reList := regexp.MustCompile(`const\s+chaptersList\s*=\s*(\[.*?\]);`)

	mSlugMatch := reSlug.FindStringSubmatch(html)
	mTypeMatch := reType.FindStringSubmatch(html)
	mListMatch := reList.FindStringSubmatch(html)

	// Pre-extract chapter upload dates from DOM elements
	chapterDates := make(map[string]int64)
	if doc, errDoc := goquery.NewDocumentFromReader(strings.NewReader(html)); errDoc == nil {
		doc.Find("div.group").Each(func(i int, s *goquery.Selection) {
			a := s.Find("a[href*='/leer/']").First()
			href, _ := a.Attr("href")
			if href == "" {
				return
			}
			parts := strings.Split(href, "/leer/")
			if len(parts) < 2 {
				return
			}
			chPart := strings.TrimSpace(strings.Split(parts[1], "?")[0])
			if chPart == "" {
				return
			}

			s.Find("span").Each(func(_ int, span *goquery.Selection) {
				txt := strings.TrimSpace(span.Text())
				if txt == "" || txt == "schedule" || strings.HasPrefix(txt, "Cap") {
					return
				}
				if d := parseLectorXDRelativeDate(txt); d > 0 {
					chapterDates[chPart] = d
				}
			})
		})
	}

	if len(mListMatch) < 2 {
		// Fallback to scraping a[href*="/leer/"]
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			return nil, fmt.Errorf("parsing HTML for chapters: %w", err)
		}

		var fallbackChapters []ChapterItem
		seen := make(map[string]bool)

		doc.Find("a[href*='/leer/']").Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			if href == "" || seen[href] {
				return
			}
			seen[href] = true

			chURL := href
			if strings.HasPrefix(chURL, h.baseURL) {
				chURL = strings.TrimPrefix(chURL, h.baseURL)
			}

			// Extract chapter number from URL e.g. /manhwa/slug/leer/12
			parts := strings.Split(strings.Trim(chURL, "/"), "/")
			chStr := parts[len(parts)-1]
			num, _ := strconv.ParseFloat(chStr, 32)

			name := strings.TrimSpace(s.Text())
			if name == "" {
				name = fmt.Sprintf("Capítulo %s", chStr)
			}

			dateUp := chapterDates[chStr]

			fallbackChapters = append(fallbackChapters, ChapterItem{
				Name:          name,
				URL:           chURL,
				RealURL:       h.baseURL + chURL,
				ChapterNumber: float32(num),
				DateUpload:    dateUp,
			})
		})

		total := len(fallbackChapters)
		for i := range fallbackChapters {
			fallbackChapters[i].SourceOrder = total - i
		}

		return fallbackChapters, nil
	}

	mSlug := ""
	if len(mSlugMatch) >= 2 {
		mSlug = mSlugMatch[1]
	} else {
		// Parse from URL
		parts := strings.Split(strings.Trim(mangaURL, "/"), "/")
		mSlug = parts[len(parts)-1]
	}

	mType := ""
	if len(mTypeMatch) >= 2 {
		if mTypeMatch[1] != "" {
			mType = mTypeMatch[1]
		} else if len(mTypeMatch) >= 3 && mTypeMatch[2] != "" {
			mType = mTypeMatch[2]
		}
	}
	if mType == "" {
		parts := strings.Split(strings.Trim(mangaURL, "/"), "/")
		if len(parts) >= 2 {
			mType = parts[0]
		}
		if mType == "" {
			mType = "manhwa"
		}
	}

	type ChapterEntry struct {
		Chapter string      `json:"chapter"`
		GroupID interface{} `json:"groupId"`
	}

	var rawEntries []ChapterEntry
	if err := json.Unmarshal([]byte(mListMatch[1]), &rawEntries); err != nil {
		return nil, fmt.Errorf("decoding chaptersList: %w", err)
	}

	var chapters []ChapterItem
	// Reverse so that newest/highest chapter is index 0 for UI presentation
	total := len(rawEntries)
	for i := total - 1; i >= 0; i-- {
		entry := rawEntries[i]
		chNum, _ := strconv.ParseFloat(entry.Chapter, 32)
		chURL := fmt.Sprintf("/%s/%s/leer/%s", mType, mSlug, entry.Chapter)
		chName := fmt.Sprintf("Capítulo %s", entry.Chapter)

		dateUp := chapterDates[entry.Chapter]

		chapters = append(chapters, ChapterItem{
			Name:          chName,
			URL:           chURL,
			RealURL:       h.baseURL + chURL,
			ChapterNumber: float32(chNum),
			DateUpload:    dateUp,
			SourceOrder:   i + 1,
		})
	}

	return chapters, nil
}

func parseLectorXDRelativeDate(dateStr string) int64 {
	dateStr = strings.TrimSpace(strings.ToLower(dateStr))
	if dateStr == "" {
		return 0
	}
	now := time.Now()

	if dateStr == "hoy" || strings.Contains(dateStr, "ahora") {
		return now.UnixMilli()
	}
	if dateStr == "ayer" {
		return now.Add(-24 * time.Hour).UnixMilli()
	}

	re := regexp.MustCompile(`(\d+)\s*([a-zA-Záéíóúñ]+)`)
	m := re.FindStringSubmatch(dateStr)
	if len(m) == 3 {
		n, err := strconv.Atoi(m[1])
		if err == nil && n > 0 {
			unit := m[2]
			switch {
			case strings.HasPrefix(unit, "d"): // d, dia, dias, días
				return now.Add(-time.Duration(n) * 24 * time.Hour).UnixMilli()
			case strings.HasPrefix(unit, "h"): // h, hora, horas
				return now.Add(-time.Duration(n) * time.Hour).UnixMilli()
			case strings.HasPrefix(unit, "min") || unit == "m": // m, min, minuto, minutos
				return now.Add(-time.Duration(n) * time.Minute).UnixMilli()
			case strings.HasPrefix(unit, "sem") || strings.HasPrefix(unit, "w"): // sem, semanas, w
				return now.Add(-time.Duration(n) * 7 * 24 * time.Hour).UnixMilli()
			case strings.HasPrefix(unit, "mes"): // mes, meses
				return now.Add(-time.Duration(n) * 30 * 24 * time.Hour).UnixMilli()
			case strings.HasPrefix(unit, "a") || strings.HasPrefix(unit, "y"): // año, anos, a, year
				return now.Add(-time.Duration(n) * 365 * 24 * time.Hour).UnixMilli()
			}
		}
	}

	for _, layout := range []string{"02/01/2006", "2006-01-02", "02-01-2006", "2 Jan 2006", "02/01/06"} {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t.UnixMilli()
		}
	}

	return 0
}

// GetPageList retrieves all page image URLs from a chapter reader page.
func (h *LectorXDHandler) GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error) {
	targetURL := chapterURL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = h.baseURL + "/" + strings.TrimLeft(chapterURL, "/")
	}

	html, err := FetchHTML(ctx, targetURL, h.baseURL+"/")
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var pages []string
	seen := make(map[string]bool)

	// Primary selector: img.page-image inside #reader-app
	doc.Find("img.page-image, #reader-app img").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("data-src")
		if src == "" {
			src, _ = s.Attr("src")
		}
		if src == "" {
			src, _ = s.Attr("data-original-src")
		}
		if src == "" || seen[src] {
			return
		}
		// Ignore cover thumbnails or small UI assets
		if strings.Contains(src, "/covers/") || strings.Contains(src, "avatar") || strings.HasPrefix(src, "data:") {
			return
		}
		seen[src] = true
		pages = append(pages, src)
	})

	if len(pages) == 0 {
		// Fallback: search regex for cdn URLs in HTML
		reImg := regexp.MustCompile(`https?://[^"'\s]*cdnlxd\.xyz/[^"'\s]+\.webp`)
		matches := reImg.FindAllString(html, -1)
		for _, m := range matches {
			if !strings.Contains(m, "/covers/") && !seen[m] {
				seen[m] = true
				pages = append(pages, m)
			}
		}
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages found on %s", targetURL)
	}

	return pages, nil
}
