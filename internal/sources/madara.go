package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func init() {
	// Register Madara family sources
	Register(8701445043633550960, NewMadaraHandlerWithPath("LeerManhwas", "https://leermanhwas.com", "leer-manhwa-18"))
	Register(3044113561993269522, NewMadaraHandler("ApollComics", "https://apollcomics.xyz"))
	Register(4647982373411241498, NewMadaraHandler("DragonTranslation.org", "https://dragontranslation.org"))
	Register(1814471239661920836, NewMadaraHandler("Cerberus Series", "https://cerberuseries.com"))
	Register(3548601371328353875, NewMadaraHandler("Manhwa-Latino", "https://manhwalatino.org"))
	Register(8311505150576608267, NewMadaraHandlerWithPath("Hades no Fansub", "https://lectorhades.latamtoon.com", "tmo"))
	Register(6757330205319558057, NewMadaraHandler("Bega Translation", "https://begatranslation.com"))
	Register(1326771034671033762, NewMadaraHandler("Manhwas.net", "https://manhwa-es.com"))
}

type MadaraHandler struct {
	name      string
	baseURL   string
	mangaPath string
}

func NewMadaraHandler(name, baseURL string) *MadaraHandler {
	return NewMadaraHandlerWithPath(name, baseURL, "manga")
}

func NewMadaraHandlerWithPath(name, baseURL, mangaPath string) *MadaraHandler {
	if mangaPath == "" {
		mangaPath = "manga"
	}
	return &MadaraHandler{
		name:      name,
		baseURL:   strings.TrimRight(baseURL, "/"),
		mangaPath: strings.Trim(mangaPath, "/"),
	}
}

func (h *MadaraHandler) Name() string {
	return h.name
}

func (h *MadaraHandler) GetPopularManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := fmt.Sprintf("%s/%s/?m_orderby=views", h.baseURL, h.mangaPath)
	if page > 1 {
		targetURL = fmt.Sprintf("%s/%s/page/%d/?m_orderby=views", h.baseURL, h.mangaPath, page)
	}
	return h.fetchMangaList(ctx, targetURL)
}

func (h *MadaraHandler) GetLatestManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := fmt.Sprintf("%s/%s/?m_orderby=latest", h.baseURL, h.mangaPath)
	if page > 1 {
		targetURL = fmt.Sprintf("%s/%s/page/%d/?m_orderby=latest", h.baseURL, h.mangaPath, page)
	}
	return h.fetchMangaList(ctx, targetURL)
}

func (h *MadaraHandler) SearchManga(ctx context.Context, query string, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := fmt.Sprintf("%s/?s=%s&post_type=wp-manga", h.baseURL, url.QueryEscape(query))
	if page > 1 {
		targetURL = fmt.Sprintf("%s/page/%d/?s=%s&post_type=wp-manga", h.baseURL, page, url.QueryEscape(query))
	}
	return h.fetchMangaList(ctx, targetURL)
}

func (h *MadaraHandler) fetchMangaList(ctx context.Context, targetURL string) ([]MangaItem, bool, error) {
	html, err := FetchHTML(ctx, targetURL, h.baseURL+"/")
	if err != nil {
		return nil, false, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, false, err
	}

	var items []MangaItem
	selectors := []string{
		"div.page-item-detail.manga",
		"div.c-tabs-item__content",
		"div.badge-pos-1",
		"div.latest-item",
	}

	for _, sel := range selectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			titleEl := s.Find("div.post-title a, h3.h4 a, h3 a, h4 a, h5 a, div.mm-name a, .item-title a").First()
			title := strings.TrimSpace(titleEl.Text())
			href, _ := titleEl.Attr("href")
			if title == "" {
				title, _ = s.Find(".item-thumb a[title], div.post-title a[title], a[title]").First().Attr("title")
			}
			if href == "" {
				href, _ = s.Find(".item-thumb a, div.post-title a, a[href*='/manga/'], a[href*='/manhwa/'], a[href*='/tmo/']").First().Attr("href")
			}
			if title == "" {
				title, _ = s.Find("img").First().Attr("alt")
			}
			title = strings.TrimSpace(title)
			if href == "" || title == "" {
				return
			}

			img := s.Find("img").First()
			thumb, _ := img.Attr("data-src")
			if thumb == "" {
				thumb, _ = img.Attr("data-lazy-src")
			}
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
			})
		})
		if len(items) > 0 {
			break
		}
	}

	// Fallback for modern grid layouts (e.g. DragonTranslation)
	if len(items) == 0 {
		doc.Find("div.agrid > a, .mk-archive .agrid a").Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			title, _ := s.Find("img").Attr("alt")
			if title == "" {
				title = strings.TrimSpace(s.Find(".ac-title, div:last-child").Text())
			}
			if href == "" || title == "" {
				return
			}

			thumb, _ := s.Find("img").Attr("src")
			if thumb == "" {
				thumb, _ = s.Find("img").Attr("data-src")
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
			})
		})
	}

	hasNext := doc.Find("div.nav-previous, a.next, .pagination .next, a.page-numbers.next, a.r, a.nextpostslink, .wp-pagenavi .nextpostslink, a[rel='next']").Length() > 0
	return items, hasNext, nil
}




var spanishMonths = map[string]string{
	"enero":      "January",
	"febrero":    "February",
	"marzo":      "March",
	"abril":      "April",
	"mayo":       "May",
	"junio":      "June",
	"julio":      "July",
	"agosto":     "August",
	"septiembre": "September",
	"setiembre":  "September",
	"octubre":    "October",
	"noviembre":  "November",
	"diciembre":  "December",
}

// ParseSpanishOrEnglishDate parses Spanish relative and calendar dates as well as standard English layouts.
func ParseSpanishOrEnglishDate(dateStr string) int64 {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return 0
	}

	lower := strings.ToLower(dateStr)

	// Check relative dates in Spanish: "hace 2 días", "hace 3 semanas", "hace 5 horas"
	// Also English: "2 days ago", "3 weeks ago"
	now := time.Now()
	if strings.HasPrefix(lower, "hace") || strings.Contains(lower, "ago") {
		var num int
		var unit string
		if strings.HasPrefix(lower, "hace") {
			parts := strings.Fields(lower)
			if len(parts) >= 3 {
				_, _ = fmt.Sscan(parts[1], &num)
				unit = parts[2]
			}
		} else if strings.Contains(lower, "ago") {
			parts := strings.Fields(lower)
			if len(parts) >= 2 {
				_, _ = fmt.Sscan(parts[0], &num)
				unit = parts[1]
			}
		}
		if num > 0 {
			switch {
			case strings.HasPrefix(unit, "segundo") || strings.HasPrefix(unit, "sec"):
				return now.Add(-time.Duration(num) * time.Second).UnixMilli()
			case strings.HasPrefix(unit, "minuto") || strings.HasPrefix(unit, "min"):
				return now.Add(-time.Duration(num) * time.Minute).UnixMilli()
			case strings.HasPrefix(unit, "hora") || strings.HasPrefix(unit, "hour"):
				return now.Add(-time.Duration(num) * time.Hour).UnixMilli()
			case strings.HasPrefix(unit, "dia") || strings.HasPrefix(unit, "día") || strings.HasPrefix(unit, "day"):
				return now.AddDate(0, 0, -num).UnixMilli()
			case strings.HasPrefix(unit, "semana") || strings.HasPrefix(unit, "week"):
				return now.AddDate(0, 0, -num*7).UnixMilli()
			case strings.HasPrefix(unit, "mes") || strings.HasPrefix(unit, "month"):
				return now.AddDate(0, -num, 0).UnixMilli()
			case strings.HasPrefix(unit, "año") || strings.HasPrefix(unit, "ano") || strings.HasPrefix(unit, "year"):
				return now.AddDate(-num, 0, 0).UnixMilli()
			}
		}
	}

	// Replace Spanish words: " de " -> " "
	normalized := lower
	normalized = strings.ReplaceAll(normalized, " de ", " ")
	for es, en := range spanishMonths {
		if strings.Contains(normalized, es) {
			normalized = strings.ReplaceAll(normalized, es, en)
			break
		}
	}

	layouts := []string{
		"January 2, 2006",
		"January 2 2006",
		"2 January, 2006",
		"2 January 2006",
		"02/01/2006",
		"02-01-2006",
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2 Jan 2006",
		"Jan 2, 2006",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, normalized); err == nil {
			return t.UnixMilli()
		}
	}

	return 0
}

func (h *MadaraHandler) GetChapterList(ctx context.Context, mangaURL string) ([]ChapterItem, error) {
	fullURL := mangaURL
	if !strings.HasPrefix(fullURL, "http") {
		fullURL = h.baseURL + "/" + strings.TrimLeft(mangaURL, "/")
	}

	html, err := FetchHTML(ctx, fullURL, h.baseURL+"/")
	if err != nil {
		return nil, fmt.Errorf("fetching Madara manga details: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var chapters []ChapterItem
	seenCh := make(map[string]bool)

	// 0. Check for embedded Madara script JSON (e.g. DragonTranslation / modern Madara themes)
	doc.Find("script[type='application/json']").Each(func(i int, s *goquery.Selection) {
		scriptText := strings.TrimSpace(s.Text())
		if strings.Contains(scriptText, `"items"`) && (strings.Contains(scriptText, `"mangaId"`) || strings.Contains(scriptText, `"num"`)) {
			var payload struct {
				Items []struct {
					ID   int    `json:"id"`
					Num  string `json:"num"`
					Name string `json:"name"`
					URL  string `json:"url"`
					Ago  string `json:"ago"`
				} `json:"items"`
			}
			if err := json.Unmarshal([]byte(scriptText), &payload); err == nil && len(payload.Items) > 0 {
				for _, item := range payload.Items {
					if item.URL == "" {
						continue
					}
					chURL := item.URL
					cleanURL := chURL
					if strings.HasPrefix(cleanURL, h.baseURL) {
						cleanURL = strings.TrimPrefix(cleanURL, h.baseURL)
					}
					if !seenCh[cleanURL] {
						seenCh[cleanURL] = true
						chName := strings.TrimSpace(item.Name)
						if chName == "" {
							chName = fmt.Sprintf("Capítulo %s", item.Num)
						}
						chNum := ParseChapterNumber(item.Num)
						if chNum == 0 {
							chNum = ParseChapterNumber(chName)
						}
						dateUpload := ParseSpanishOrEnglishDate(item.Ago)
						chapters = append(chapters, ChapterItem{
							Name:          chName,
							URL:           cleanURL,
							RealURL:       chURL,
							ChapterNumber: chNum,
							DateUpload:    dateUpload,
						})
					}
				}
			}
		}
	})

	addChapter := func(s *goquery.Selection, a *goquery.Selection) {
		if a == nil || a.Length() == 0 {
			return
		}
		chURL, _ := a.Attr("href")
		chName := strings.TrimSpace(a.Find("span.chapter-name").Text())
		if chName == "" {
			chName = strings.TrimSpace(a.Text())
		}
		if chURL != "" && chName != "" {
			cleanURL := chURL
			if strings.HasPrefix(cleanURL, h.baseURL) {
				cleanURL = strings.TrimPrefix(cleanURL, h.baseURL)
			}
			if !seenCh[cleanURL] {
				seenCh[cleanURL] = true
				chNum := ParseChapterNumber(chName)
				if chNum == 0 {
					chNum = ParseChapterNumber(cleanURL)
				}
				var dateUpload int64
				if s != nil {
					dateStr := strings.TrimSpace(s.Find("span.chapter-release-date, .chapter-release-date").Text())
					dateUpload = ParseSpanishOrEnglishDate(dateStr)
				}
				chapters = append(chapters, ChapterItem{
					Name:          chName,
					URL:           cleanURL,
					RealURL:       chURL,
					ChapterNumber: chNum,
					DateUpload:    dateUpload,
				})
			}
		}
	}

	doc.Find("li.wp-manga-chapter, a.leermos").Each(func(i int, s *goquery.Selection) {
		a := s
		if !s.Is("a") {
			a = s.Find("a").First()
		}
		addChapter(s, a)
	})

	if len(chapters) == 0 {
		// 1. Try modern Madara AJAX endpoint: {mangaURL}/ajax/chapters/
		ajaxChaptersURL := strings.TrimRight(fullURL, "/") + "/ajax/chapters/"
		reqAjax, errReq := http.NewRequestWithContext(ctx, http.MethodPost, ajaxChaptersURL, nil)
		if errReq == nil {
			reqAjax.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			reqAjax.Header.Set("X-Requested-With", "XMLHttpRequest")
			reqAjax.Header.Set("Referer", fullURL)
			respAjax, errDo := DefaultClient.Do(reqAjax)
			if errDo == nil && respAjax.StatusCode == http.StatusOK {
				defer respAjax.Body.Close()
				ajaxDoc, errDoc := goquery.NewDocumentFromReader(respAjax.Body)
				if errDoc == nil {
					ajaxDoc.Find("li.wp-manga-chapter, a.leermos").Each(func(i int, s *goquery.Selection) {
						a := s
						if !s.Is("a") {
							a = s.Find("a").First()
						}
						addChapter(s, a)
					})
				}
			}
		}
	}

	if len(chapters) == 0 {
		postID, _ := doc.Find("input.rating-post-id, input#wp-manga-chapter-post-id, input[name='postID']").Attr("value")
		if postID != "" {
			ajaxURL := h.baseURL + "/wp-admin/admin-ajax.php"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, ajaxURL, strings.NewReader("action=manga_get_chapters&manga="+postID))
			if err == nil {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			req.Header.Set("Referer", fullURL)
				resp, err := DefaultClient.Do(req)
				if err == nil {
					defer resp.Body.Close()
					ajaxDoc, err := goquery.NewDocumentFromReader(resp.Body)
				if err == nil {
					ajaxDoc.Find("li.wp-manga-chapter a").Each(func(i int, a *goquery.Selection) {
						addChapter(nil, a)
					})
				}
			}
		}
	}
	}

	total := len(chapters)
	isOldestFirst := false
	if len(chapters) > 1 && chapters[0].ChapterNumber > 0 && chapters[total-1].ChapterNumber > 0 {
		if chapters[0].ChapterNumber < chapters[total-1].ChapterNumber {
			isOldestFirst = true
		}
	}

	for i := range chapters {
		if isOldestFirst {
			chapters[i].SourceOrder = i + 1
		} else {
			chapters[i].SourceOrder = total - i
		}
	}

	return chapters, nil
}

func (h *MadaraHandler) GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error) {
	fullURL := chapterURL
	if !strings.HasPrefix(fullURL, "http") {
		fullURL = h.baseURL + "/" + strings.TrimLeft(chapterURL, "/")
	}

	html, err := FetchHTML(ctx, fullURL, h.baseURL+"/")
	if err != nil {
		return nil, fmt.Errorf("fetching Madara chapter page: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var pages []string
	selectors := []string{
		"div.reading-content img",
		"div.page-break img",
		"div.entry-content img",
		"div.wp-manga-chapter-img img",
		".read-container img",
		"img.loading",
	}

	for _, sel := range selectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			src, exists := s.Attr("data-src")
			if !exists || src == "" {
				src, exists = s.Attr("data-lazy-src")
			}
			if !exists || src == "" {
				src, exists = s.Attr("src")
			}
			src = strings.TrimSpace(src)
			if exists && src != "" && !strings.HasPrefix(src, "data:") && !strings.Contains(src, "blank.gif") && !strings.Contains(src, "logo") && !strings.Contains(src, "/ads/") && !strings.Contains(src, "advertisement") && !strings.Contains(src, "icon") {
				if strings.HasPrefix(src, "//") {
					src = "https:" + src
				} else if strings.HasPrefix(src, "/") {
					src = h.baseURL + src
				}
				pages = append(pages, src)
			}
		})
		if len(pages) > 0 {
			break
		}
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages found in Madara chapter %s", fullURL)
	}

	return pages, nil
}
