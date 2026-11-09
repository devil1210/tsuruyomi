package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func init() {
	Register(1136717301008230074, NewSubmanhwaHandler())
}

type SubmanhwaHandler struct {
	name    string
	baseURL string
}

func NewSubmanhwaHandler() *SubmanhwaHandler {
	return &SubmanhwaHandler{
		name:    "Submanhwa",
		baseURL: "https://submanhwa.com",
	}
}

func (h *SubmanhwaHandler) Name() string {
	return h.name
}

func (h *SubmanhwaHandler) GetPopularManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := fmt.Sprintf("%s/filterList?page=%d&sortBy=views&asc=false", h.baseURL, page)
	html, err := FetchHTML(ctx, targetURL, h.baseURL+"/")
	if err != nil {
		return nil, false, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, false, err
	}

	var items []MangaItem
	doc.Find("div.series-card").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a").First()
		href, _ := a.Attr("href")
		title := strings.TrimSpace(s.Find(".series-title").First().Text())
		if title == "" {
			title = strings.TrimSpace(a.AttrOr("title", a.Text()))
		}
		if idx := strings.Index(title, "\n"); idx != -1 {
			title = strings.TrimSpace(title[:idx])
		}
		if href == "" || title == "" {
			return
		}

		img := s.Find("img").First()
		thumb, _ := img.Attr("data-src")
		if thumb == "" {
			thumb, _ = img.Attr("src")
		}
		if strings.HasPrefix(thumb, "//") {
			thumb = "https:" + thumb
		} else if strings.HasPrefix(thumb, "/") {
			thumb = h.baseURL + thumb
		}

		mangaURL := href
		if strings.HasPrefix(mangaURL, h.baseURL) {
			mangaURL = strings.TrimPrefix(mangaURL, h.baseURL)
		}

		items = append(items, MangaItem{
			Title:        title,
			URL:          mangaURL,
			ThumbnailURL: thumb,
			Status:       1,
			SourceID:     1136717301008230074,
		})
	})

	hasNext := doc.Find("li a[rel=next]").Length() > 0
	return items, hasNext, nil
}

func (h *SubmanhwaHandler) GetLatestManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	if page > 1 {
		return h.GetPopularManga(ctx, page)
	}

	html, err := FetchHTML(ctx, h.baseURL+"/", h.baseURL+"/")
	if err != nil {
		return h.GetPopularManga(ctx, page)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return h.GetPopularManga(ctx, page)
	}

	var items []MangaItem
	seen := make(map[string]bool)
	doc.Find("article.up-card").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a.up-title").First()
		if a.Length() == 0 {
			a = s.Find("a.up-cover").First()
		}
		if a.Length() == 0 {
			a = s.Find("a").First()
		}
		href, _ := a.Attr("href")
		title := strings.TrimSpace(a.AttrOr("title", a.Text()))
		if title == "" {
			title = strings.TrimSpace(s.Find("img").First().AttrOr("alt", ""))
		}
		if href == "" || title == "" || seen[href] {
			return
		}
		seen[href] = true

		img := s.Find("img").First()
		thumb, _ := img.Attr("data-src")
		if thumb == "" {
			thumb, _ = img.Attr("src")
		}
		if strings.HasPrefix(thumb, "//") {
			thumb = "https:" + thumb
		} else if strings.HasPrefix(thumb, "/") {
			thumb = h.baseURL + thumb
		}

		mangaURL := href
		if strings.HasPrefix(mangaURL, h.baseURL) {
			mangaURL = strings.TrimPrefix(mangaURL, h.baseURL)
		}

		items = append(items, MangaItem{
			Title:        title,
			URL:          mangaURL,
			ThumbnailURL: thumb,
			Status:       1,
			SourceID:     1136717301008230074,
		})
	})

	if len(items) == 0 {
		return h.GetPopularManga(ctx, page)
	}

	return items, false, nil
}

func (h *SubmanhwaHandler) SearchManga(ctx context.Context, query string, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return h.GetPopularManga(ctx, page)
	}

	// 1. Primary: Official JSON Search API
	apiURL := fmt.Sprintf("%s/api/series-search?q=%s&kind=all&rating=all&exact=0", h.baseURL, url.QueryEscape(cleanQuery))
	respBody, err := FetchHTML(ctx, apiURL, h.baseURL+"/")
	if err == nil && len(respBody) > 0 {
		var res struct {
			Mode  string `json:"mode"`
			Items []struct {
				Name     string `json:"name"`
				URL      string `json:"url"`
				Cover    string `json:"cover"`
				Chapters int    `json:"chapters"`
				Status   string `json:"status"`
			} `json:"items"`
		}
		if errJSON := json.Unmarshal([]byte(respBody), &res); errJSON == nil && len(res.Items) > 0 {
			var items []MangaItem
			for _, it := range res.Items {
				mangaURL := it.URL
				if strings.HasPrefix(mangaURL, h.baseURL) {
					mangaURL = strings.TrimPrefix(mangaURL, h.baseURL)
				}
				thumb := it.Cover
				if strings.HasPrefix(thumb, "//") {
					thumb = "https:" + thumb
				} else if strings.HasPrefix(thumb, "/") {
					thumb = h.baseURL + thumb
				}
				items = append(items, MangaItem{
					Title:        it.Name,
					URL:          mangaURL,
					ThumbnailURL: thumb,
					Status:       1,
					SourceID:     1136717301008230074,
				})
			}
			return items, false, nil
		}
	}

	// 2. Fallback: Search through latest updates from homepage
	latestItems, _, errLatest := h.GetLatestManga(ctx, 1)
	if errLatest == nil {
		var matched []MangaItem
		lowerQuery := strings.ToLower(cleanQuery)
		for _, it := range latestItems {
			if strings.Contains(strings.ToLower(it.Title), lowerQuery) {
				matched = append(matched, it)
			}
		}
		if len(matched) > 0 {
			return matched, false, nil
		}
	}

	return nil, false, nil
}

var reSubmanhwaChNum = regexp.MustCompile(`(?i)(?:cap[ií]tulo|cap\.?|ch\.?)\s*(\d+(?:\.\d+)?)`)

func (h *SubmanhwaHandler) GetMangaDetails(ctx context.Context, mangaURL string) (*MangaItem, error) {
	fullURL := mangaURL
	if !strings.HasPrefix(fullURL, "http") {
		fullURL = h.baseURL + "/" + strings.TrimLeft(mangaURL, "/")
	}

	html, err := FetchHTML(ctx, fullURL, h.baseURL+"/")
	if err != nil {
		return nil, fmt.Errorf("fetching Submanhwa manga details: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	title := strings.TrimSpace(doc.Find(".sr-info > h1").Text())
	desc := strings.TrimSpace(doc.Find(".sr-summary > p, #sr-summary, .sr-summary").Text())
	if desc == "" {
		desc = doc.Find("meta[property='og:description']").AttrOr("content", "")
	}
	thumb, _ := doc.Find("img.sr-cover").Attr("src")
	if thumb == "" {
		thumb, _ = doc.Find("img.sr-cover").Attr("data-src")
	}

	status := 1 // ongoing
	if doc.Find(".ended").Length() > 0 || strings.Contains(strings.ToLower(doc.Find(".sr-status, .status, .sr-info").Text()), "completado") {
		status = 2 // completed
	}

	var genres []string
	doc.Find(".sr-badge.type, .sr-cat, .sr-tag").Each(func(i int, s *goquery.Selection) {
		g := strings.TrimSpace(s.Text())
		if g != "" {
			genres = append(genres, g)
		}
	})

	return &MangaItem{
		Title:        title,
		Description:  desc,
		Status:       status,
		Genres:       genres,
		ThumbnailURL: thumb,
		URL:          mangaURL,
	}, nil
}

func (h *SubmanhwaHandler) GetChapterList(ctx context.Context, mangaURL string) ([]ChapterItem, error) {
	fullURL := mangaURL
	if !strings.HasPrefix(fullURL, "http") {
		fullURL = h.baseURL + "/" + strings.TrimLeft(mangaURL, "/")
	}

	html, err := FetchHTML(ctx, fullURL, h.baseURL+"/")
	if err != nil {
		return nil, fmt.Errorf("fetching Submanhwa manga details: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	chaptersDoc := doc
	// Try fetching full chapter list via AJAX endpoint if available
	reSerieID := regexp.MustCompile(`lista-capitulos/serie/(\d+)`)
	if mSerie := reSerieID.FindStringSubmatch(html); len(mSerie) >= 2 {
		ajaxURL := fmt.Sprintf("%s/lista-capitulos/serie/%s?orden=desc&offset=0&limit=5000", h.baseURL, mSerie[1])
		req, errReq := http.NewRequestWithContext(ctx, http.MethodGet, ajaxURL, nil)
		if errReq == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
			req.Header.Set("Referer", fullURL)
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			if resp, errResp := DefaultClient.Do(req); errResp == nil {
				defer resp.Body.Close()
				var ajaxRes struct {
					Ok   bool   `json:"ok"`
					HTML string `json:"html"`
				}
				if errDec := json.NewDecoder(resp.Body).Decode(&ajaxRes); errDec == nil && ajaxRes.HTML != "" {
					if d, errParse := goquery.NewDocumentFromReader(strings.NewReader(ajaxRes.HTML)); errParse == nil {
						chaptersDoc = d
					}
				}
			}
		}
	}

	var chapters []ChapterItem
	chaptersDoc.Find(".lc-row, div[id^='chapter-card'], .chapters-grid [class^=chapter-card], div[class*='chapter-card']").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a.lc-t").First()
		if a.Length() == 0 {
			a = s.Find("a.lc-n").First()
		}
		if a.Length() == 0 {
			a = s.Find("a.chapter-link").First()
		}
		if a.Length() == 0 {
			a = s.Find("a").First()
		}
		chURL, _ := a.Attr("href")
		chName := strings.TrimSpace(a.Text())
		if chURL != "" && chName != "" {
			cleanURL := chURL
			if strings.HasPrefix(cleanURL, h.baseURL) {
				cleanURL = strings.TrimPrefix(cleanURL, h.baseURL)
			}

			var chNum float32
			if dataNum, ok := s.Attr("data-num"); ok && dataNum != "" {
				if parsed, errNum := strconv.ParseFloat(dataNum, 32); errNum == nil {
					chNum = float32(parsed)
				}
			}
			if chNum == 0 {
				matches := reSubmanhwaChNum.FindStringSubmatch(chName)
				if len(matches) >= 2 {
					if parsed, errNum := strconv.ParseFloat(matches[1], 32); errNum == nil {
						chNum = float32(parsed)
					}
				}
			}

			dateElem := s.Find(".lc-d, time, .chapter-date, span.date, span.chapter-time, small, .chapter-meta").First()
			dateStr := strings.TrimSpace(dateElem.Text())
			dateStr = strings.TrimSpace(strings.ReplaceAll(dateStr, "🕒", ""))
			dateUpload := ParseSpanishOrEnglishDate(dateStr)

			chapters = append(chapters, ChapterItem{
				Name:          chName,
				URL:           cleanURL,
				RealURL:       chURL,
				ChapterNumber: chNum,
				DateUpload:    dateUpload,
			})
		}
	})

	total := len(chapters)
	for i := range chapters {
		chapters[i].SourceOrder = total - i
	}

	return chapters, nil
}

func (h *SubmanhwaHandler) GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error) {
	fullURL := chapterURL
	if !strings.HasPrefix(fullURL, "http") {
		fullURL = h.baseURL + "/" + strings.TrimLeft(chapterURL, "/")
	}

	html, err := FetchHTML(ctx, fullURL, h.baseURL+"/")
	if err != nil {
		return nil, fmt.Errorf("fetching chapter page: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var pages []string
	doc.Find("#all img, .rd-viewer img, .viewer-cnt img, .chapter-content img, div.page-break img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("data-src")
		if !exists || src == "" || strings.HasPrefix(src, "data:") {
			src, exists = s.Attr("src")
		}
		if exists && src != "" && !strings.HasPrefix(src, "data:") && !strings.Contains(src, "logo") && !strings.Contains(src, "banner") {
			cleanSrc := strings.TrimSpace(src)
			if strings.HasPrefix(cleanSrc, "//") {
				cleanSrc = "https:" + cleanSrc
			}
			pages = append(pages, cleanSrc)
		}
	})

	if len(pages) == 0 {
		reVarPages := regexp.MustCompile(`var\s+pages\s*=\s*(\[\{.*?\}\]);`)
		if m := reVarPages.FindStringSubmatch(html); len(m) >= 2 {
			var jsonPages []struct {
				PageImage string `json:"page_image"`
			}
			if errJ := json.Unmarshal([]byte(m[1]), &jsonPages); errJ == nil {
				for _, jp := range jsonPages {
					if jp.PageImage != "" {
						pages = append(pages, jp.PageImage)
					}
				}
			}
		}
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages found in Submanhwa chapter %s", fullURL)
	}

	return pages, nil
}
