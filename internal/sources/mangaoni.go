package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func init() {
	// Register MangaOni with its canonical Tachiyomi source ID
	Register(2202687009511923782, &MangaOniHandler{})
}

type MangaOniHandler struct{}

func (h *MangaOniHandler) Name() string {
	return "MangaOni"
}

var reUnicap = regexp.MustCompile(`var\s+unicap\s*=\s*'([^']+)';`)

func (h *MangaOniHandler) GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error) {
	fullURL := chapterURL
	if !strings.HasPrefix(fullURL, "http") {
		fullURL = "https://manga-oni.com" + "/" + strings.TrimPrefix(chapterURL, "/")
	}

	html, err := FetchHTML(ctx, fullURL, "https://manga-oni.com/")
	if err != nil {
		return nil, fmt.Errorf("fetching chapter page: %w", err)
	}

	matches := reUnicap.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("unicap variable not found in MangaOni chapter HTML")
	}

	rawB64 := matches[1]
	// Pad base64 if needed
	if pad := len(rawB64) % 4; pad > 0 {
		rawB64 += strings.Repeat("=", 4-pad)
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return nil, fmt.Errorf("decoding unicap base64: %w", err)
	}

	decoded := string(decodedBytes)
	parts := strings.Split(decoded, "||")
	if len(parts) < 2 {
		return nil, fmt.Errorf("unexpected unicap format: %s", decoded)
	}

	baseURL := strings.TrimRight(parts[0], "/")
	var filenames []string
	if err := json.Unmarshal([]byte(parts[1]), &filenames); err != nil {
		return nil, fmt.Errorf("unmarshaling filenames from unicap: %w", err)
	}

	var pages []string
	for _, fn := range filenames {
		if strings.HasPrefix(fn, "http") {
			pages = append(pages, fn)
		} else {
			pages = append(pages, baseURL+"/"+strings.TrimLeft(fn, "/"))
		}
	}

	return pages, nil
}

var reMangaOniChapter = regexp.MustCompile(`<a[^>]+href="([^"]+)"[^>]*>\s*<div class="entry-title">\s*<span[^>]*data-user="([^"]*)"[^>]*data-num="([^"]*)"[^>]*datetime="([^"]*)"[^>]*><\/span>\s*<h3 class="entry-title-h2">([^<]+)<\/h3>`)

func (h *MangaOniHandler) GetChapterList(ctx context.Context, mangaURL string) ([]ChapterItem, error) {
	fullURL := mangaURL
	if !strings.HasPrefix(fullURL, "http") {
		fullURL = "https://manga-oni.com" + "/" + strings.TrimLeft(mangaURL, "/")
	}

	html, err := FetchHTML(ctx, fullURL, "https://manga-oni.com/")
	if err != nil {
		return nil, fmt.Errorf("fetching manga page from MangaOni: %w", err)
	}

	matches := reMangaOniChapter.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no chapters found on MangaOni page %s", fullURL)
	}

	total := len(matches)
	chapters := make([]ChapterItem, 0, total)
	for i, m := range matches {
		rawHref := m[1]
		user := m[2]
		numStr := m[3]
		dateStr := m[4]
		title := strings.TrimSpace(m[5])

		var chNum float32
		if n, err := strconv.ParseFloat(numStr, 32); err == nil {
			chNum = float32(n)
		}

		var dateUpload int64
		if t, err := time.Parse("2006-01-02 15:04:05", dateStr); err == nil {
			dateUpload = t.UnixMilli()
		}

		// Normalize to relative URL
		relURL := rawHref
		if idx := strings.Index(relURL, "manga-oni.com"); idx != -1 {
			relURL = relURL[idx+len("manga-oni.com"):]
		}

		chapters = append(chapters, ChapterItem{
			Name:          title,
			URL:           relURL,
			RealURL:       rawHref,
			ChapterNumber: chNum,
			Scanlator:     user,
			DateUpload:    dateUpload,
			SourceOrder:   total - i, // Older chapters have order 1, newest order total
		})
	}

	return chapters, nil
}

var reCSRFToken = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)">`)

type mangaOniSearchResponse struct {
	Mangas []struct {
		Nombre  string `json:"nombre"`
		Alterno string `json:"alterno"`
		Visible int    `json:"visible"`
		Slug    string `json:"slug"`
		Autor   string `json:"autor"`
		URL     string `json:"url"`
		Img     string `json:"img"`
	} `json:"mangas"`
}

// SearchManga queries MangaOni online using its AJAX search endpoint.
func (h *MangaOniHandler) SearchManga(ctx context.Context, query string, page int) ([]MangaItem, bool, error) {
	if strings.TrimSpace(query) == "" {
		return nil, false, nil
	}

	// 1. Fetch homepage to extract CSRF token and cookie
	homeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://manga-oni.com", nil)
	if err != nil {
		return nil, false, err
	}
	homeReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
	homeResp, err := DefaultClient.Do(homeReq)
	if err != nil {
		return nil, false, fmt.Errorf("fetching MangaOni CSRF: %w", err)
	}
	defer homeResp.Body.Close()

	homeHTML, _ := io.ReadAll(homeResp.Body)
	m := reCSRFToken.FindStringSubmatch(string(homeHTML))
	if len(m) < 2 {
		return nil, false, fmt.Errorf("could not find CSRF token on MangaOni homepage")
	}
	csrfToken := m[1]

	// 2. Perform search POST
	form := url.Values{}
	form.Set("buscar", query)
	form.Set("_token", csrfToken)

	searchReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://manga-oni.com/buscar", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, false, err
	}
	searchReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
	searchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	searchReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	searchReq.Header.Set("X-CSRF-TOKEN", csrfToken)
	searchReq.Header.Set("Referer", "https://manga-oni.com/")

	// Copy cookies from homepage response
	for _, cookie := range homeResp.Cookies() {
		searchReq.AddCookie(cookie)
	}

	searchResp, err := DefaultClient.Do(searchReq)
	if err != nil {
		return nil, false, fmt.Errorf("calling MangaOni search: %w", err)
	}
	defer searchResp.Body.Close()

	if searchResp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("MangaOni search returned HTTP %d", searchResp.StatusCode)
	}

	var res mangaOniSearchResponse
	if err := json.NewDecoder(searchResp.Body).Decode(&res); err != nil {
		return nil, false, fmt.Errorf("decoding MangaOni search JSON: %w", err)
	}

	var items []MangaItem
	for _, m := range res.Mangas {
		relURL := m.URL
		if idx := strings.Index(relURL, "manga-oni.com"); idx != -1 {
			relURL = relURL[idx+len("manga-oni.com"):]
		}
		if relURL == "" && m.Slug != "" {
			relURL = "/manga/" + m.Slug + "/"
		}

		items = append(items, MangaItem{
			ID:           m.Slug,
			Title:        m.Nombre,
			Status:       1,
			ThumbnailURL: m.Img,
			URL:          relURL,
			SourceID:     2202687009511923782,
			Author:       m.Autor,
		})
	}

	return items, false, nil
}

// GetPopularManga retrieves popular mangas from MangaOni top / directory.
func (h *MangaOniHandler) GetPopularManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := "https://manga-oni.com/top/"
	if page > 1 {
		targetURL = fmt.Sprintf("https://manga-oni.com/directorio/?p=%d", page)
	}
	return h.fetchHTMLMangaList(ctx, targetURL)
}

// GetLatestManga retrieves recently updated mangas from MangaOni.
func (h *MangaOniHandler) GetLatestManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := "https://manga-oni.com/recientes/"
	if page > 1 {
		targetURL = fmt.Sprintf("https://manga-oni.com/recientes/?p=%d", page)
	}
	return h.fetchHTMLMangaList(ctx, targetURL)
}

func (h *MangaOniHandler) fetchHTMLMangaList(ctx context.Context, targetURL string) ([]MangaItem, bool, error) {
	html, err := FetchHTML(ctx, targetURL, "https://manga-oni.com/")
	if err != nil {
		return nil, false, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, false, err
	}

	var items []MangaItem
	seen := make(map[string]bool)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}
		if !strings.Contains(href, "/manga/") && !strings.Contains(href, "/manhua/") {
			return
		}
		if strings.HasSuffix(href, "/manga/") || strings.HasSuffix(href, "/top/") || strings.HasSuffix(href, "/directorio/") || strings.HasSuffix(href, "/recientes/") {
			return
		}

		relURL := href
		if idx := strings.Index(relURL, "manga-oni.com"); idx != -1 {
			relURL = relURL[idx+len("manga-oni.com"):]
		}
		relURL = "/" + strings.Trim(relURL, "/") + "/"
		if seen[relURL] {
			return
		}

		img := s.Find("img").First()
		thumbURL, _ := img.Attr("data-src")
		if thumbURL == "" {
			thumbURL, _ = img.Attr("src")
		}
		title, _ := img.Attr("alt")
		if title == "" {
			title = strings.TrimSpace(s.Find("h2, h3, .entry-title").Text())
		}
		if title == "" {
			title = strings.TrimSpace(s.Text())
		}

		if title == "" || strings.HasPrefix(title, "Capítulo") {
			return
		}

		seen[relURL] = true
		items = append(items, MangaItem{
			Title:        title,
			URL:          relURL,
			ThumbnailURL: thumbURL,
			SourceID:     2202687009511923782,
			Status:       1,
		})
	})

	hasNext := len(items) >= 10
	return items, hasNext, nil
}


