package sources

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"tsuruyomi/internal/database"
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

	html, err := FetchHTML(ctx, h.baseURL, h.baseURL+"/")
	if err != nil {
		return nil, false, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, false, err
	}

	var items []MangaItem
	seen := make(map[string]bool)
	doc.Find("article.up-card").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a.up-title").First()
		if a.Length() == 0 {
			a = s.Find("a").First()
		}
		href, _ := a.Attr("href")
		title := strings.TrimSpace(a.Text())
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

	// 1. Try biblioteca endpoint
	targetURL := fmt.Sprintf("%s/biblioteca?search=%s&page=%d", h.baseURL, url.QueryEscape(cleanQuery), page)
	html, err := FetchHTML(ctx, targetURL, h.baseURL+"/")
	if err == nil {
		doc, errDoc := goquery.NewDocumentFromReader(strings.NewReader(html))
		if errDoc == nil {
			var items []MangaItem
			doc.Find("a.lib-card").Each(func(i int, s *goquery.Selection) {
				href, _ := s.Attr("href")
				title := strings.TrimSpace(s.Find(".lib-title").Text())
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

			if len(items) > 0 {
				hasNext := doc.Find(".lib-pages a[aria-label=Siguiente], .pagination a[rel=next]").Length() > 0
				return items, hasNext, nil
			}
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

	// Update manga metadata in DB if available
	if database.DB != nil {
		title := strings.TrimSpace(doc.Find(".sr-info > h1").Text())
		desc := strings.TrimSpace(doc.Find(".sr-summary > p").Text())
		thumb, _ := doc.Find("img.sr-cover").Attr("src")
		if thumb == "" {
			thumb, _ = doc.Find("img.sr-cover").Attr("data-src")
		}

		status := 1 // ongoing
		if doc.Find(".ended").Length() > 0 {
			status = 2 // completed
		}

		var genres []string
		doc.Find(".sr-badge.type, .sr-cat, .sr-tag").Each(func(i int, s *goquery.Selection) {
			g := strings.TrimSpace(s.Text())
			if g != "" {
				genres = append(genres, g)
			}
		})
		genreStr := strings.Join(genres, ", ")

		cleanRelURL := mangaURL
		if strings.HasPrefix(cleanRelURL, h.baseURL) {
			cleanRelURL = strings.TrimPrefix(cleanRelURL, h.baseURL)
		}

		_, _ = database.DB.Exec(ctx, `
			UPDATE suwayomi.manga SET
				title = COALESCE(NULLIF($1, ''), title),
				thumbnail_url = COALESCE(NULLIF($2, ''), thumbnail_url),
				description = COALESCE(NULLIF($3, ''), description),
				status = $4,
				genre = COALESCE(NULLIF($5, ''), genre)
			WHERE source = 1136717301008230074 AND (url = $6 OR url = $7)
		`, title, thumb, desc, status, genreStr, cleanRelURL, fullURL)
	}

	var chapters []ChapterItem
	doc.Find(".chapters-grid [class^=chapter-card], div[class*='chapter-card']").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a.chapter-link").First()
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
			matches := reSubmanhwaChNum.FindStringSubmatch(chName)
			if len(matches) >= 2 {
				if parsed, errNum := strconv.ParseFloat(matches[1], 32); errNum == nil {
					chNum = float32(parsed)
				}
			}

			chapters = append(chapters, ChapterItem{
				Name:          chName,
				URL:           cleanURL,
				RealURL:       chURL,
				ChapterNumber: chNum,
				SourceOrder:   len(chapters),
			})
		}
	})

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
	doc.Find("#all img, .chapter-content img, div.page-break img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("data-src")
		if !exists || src == "" {
			src, exists = s.Attr("src")
		}
		if exists && src != "" && !strings.Contains(src, "logo") && !strings.Contains(src, "banner") {
			cleanSrc := strings.TrimSpace(src)
			if strings.HasPrefix(cleanSrc, "//") {
				cleanSrc = "https:" + cleanSrc
			}
			pages = append(pages, cleanSrc)
		}
	})

	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages found in Submanhwa chapter %s", fullURL)
	}

	return pages, nil
}
