package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func init() {
	// Register Madara family sources
	Register(8701445043633550960, NewMadaraHandlerWithPath("LeerManhwas", "https://leermanhwas.com", "leer-manhwa-18"))
	Register(3044113561993269522, NewMadaraHandler("ApollComics", "https://apollcomics.xyz"))
	Register(4647982373411241498, NewMadaraHandler("DragonTranslation.org", "https://dragontranslation.org"))
	Register(3548601371328353875, NewMadaraHandler("Manhwa-Latino", "https://manhwalatino.org"))
	Register(8311505150576608267, NewMadaraHandlerWithPath("Hades no Fansub", "https://lectorhades.latamtoon.com", "tmo"))
	Register(6757330205319558057, NewMadaraHandlerWithPath("Bega Translation", "https://begatranslation.com", "mangas"))
	Register(1326771034671033762, NewMadaraHandler("Manhwas.net", "https://manhwa-es.com"))
	Register(829053423099062956, NewMadaraHandler("Mangas No Sekai", "https://mangasnosekai.com"))
	Register(1348827658894665970, NewMadaraHandlerWithPath("Ragnarok Scanlation", "https://ragnarokscanlation.org", "series"))
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
				href, _ = s.Find(".item-thumb a, div.post-title a, a[href*='/manga/'], a[href*='/manhwa/'], a[href*='/tmo/'], a[href*='/series/']").First().Attr("href")
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




var monthWordRegex = regexp.MustCompile(`(?i)\b[a-zA-ZáéíóúÁÉÍÓÚ]{3,10}\b`)

var spanishAndEnglishMonths = map[string]string{
	// Spanish full
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
	// Spanish short
	"ene": "Jan",
	"feb": "Feb",
	"mar": "Mar",
	"abr": "Apr",
	"may": "May",
	"jun": "Jun",
	"jul": "Jul",
	"ago": "Aug",
	"sep": "Sep",
	"set": "Sep",
	"oct": "Oct",
	"nov": "Nov",
	"dic": "Dec",
	// English full
	"january":   "January",
	"february":  "February",
	"march":     "March",
	"april":     "April",
	"june":      "June",
	"july":      "July",
	"august":    "August",
	"september": "September",
	"october":   "October",
	"november":  "November",
	"december":  "December",
	// English short
	"jan": "Jan",
	"apr": "Apr",
	"aug": "Aug",
	"dec": "Dec",
}

// ParseSpanishOrEnglishDate parses Spanish relative and calendar dates as well as standard English layouts.
func ParseSpanishOrEnglishDate(dateStr string) int64 {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return 0
	}

	lower := strings.ToLower(dateStr)
	now := time.Now()

	if lower == "hoy" || lower == "today" || lower == "new" {
		return now.UnixMilli()
	}
	if lower == "ayer" || lower == "yesterday" {
		return now.AddDate(0, 0, -1).UnixMilli()
	}

	// Check relative dates in Spanish: "hace 2 días", "hace 3 semanas", "hace 5 horas"
	// Also English: "2 days ago", "3 weeks ago"
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

	// Clean separators
	cleaned := dateStr
	cleaned = strings.ReplaceAll(cleaned, " de ", " ")
	cleaned = strings.ReplaceAll(cleaned, " De ", " ")
	// Strip dot after abbreviated month (e.g. "Nov." -> "Nov")
	cleaned = regexp.MustCompile(`([a-zA-Z]{3,})\.`).ReplaceAllString(cleaned, "$1")

	// Replace month words with English Title-cased equivalents
	normalized := monthWordRegex.ReplaceAllStringFunc(cleaned, func(word string) string {
		if repl, ok := spanishAndEnglishMonths[strings.ToLower(word)]; ok {
			return repl
		}
		return word
	})

	layouts := []string{
		"January 2, 2006",
		"January 2 2006",
		"January 02, 2006",
		"January 02 2006",
		"2 January, 2006",
		"2 January 2006",
		"02 January, 2006",
		"02 January 2006",
		"Jan 2, 2006",
		"Jan 2 2006",
		"Jan 02, 2006",
		"Jan 02 2006",
		"2 Jan 2006",
		"02 Jan 2006",
		"2 Jan, 2006",
		"02 Jan, 2006",
		"02/01/2006",
		"2/1/2006",
		"02-01-2006",
		"2-1-2006",
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05Z07:00",
		"02 Jan 2006 15:04",
		"Jan 02, 2006 15:04",
		"02 Jan 06",
		"Jan 02 06",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, normalized); err == nil {
			return t.UnixMilli()
		}
	}

	return 0
}

func (h *MadaraHandler) GetMangaDetails(ctx context.Context, mangaURL string) (*MangaItem, error) {
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

	title := strings.TrimSpace(doc.Find(".post-title h1, div.post-title > h1, .post-title, h1.entry-title, .entry-title").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("meta[property='og:title']").AttrOr("content", ""))
	}

	desc := strings.TrimSpace(doc.Find("div.description-summary div.summary__content, div.summary__content, div.short-desc-content, div.manga-excerpt, .c-page-content p, div.post-content_item:contains('Sinopsis') .summary-content, .sinopsis-text, .sinopsis").Text())
	if desc == "" {
		if mDesc, ok := doc.Find("meta[property='og:description']").Attr("content"); ok && strings.TrimSpace(mDesc) != "" {
			desc = strings.TrimSpace(mDesc)
		} else if mDesc, ok := doc.Find("meta[name='description']").Attr("content"); ok && strings.TrimSpace(mDesc) != "" {
			desc = strings.TrimSpace(mDesc)
		}
	}

	author := strings.TrimSpace(doc.Find("div.author-content a, div.author-content, a[href*='/autor/']").First().Text())
	artist := strings.TrimSpace(doc.Find("div.artist-content a, div.artist-content, a[href*='/artista/']").First().Text())

	status := 1 // ongoing
	statusText := strings.ToLower(doc.Find("div.post-status, div.post-content_item:contains('Estado'), .status").Text())
	if strings.Contains(statusText, "complet") || strings.Contains(statusText, "finaliz") {
		status = 2 // completed
	}

	var genres []string
	doc.Find("div.genres-content a, .genres a, a[href*='/genero/']").Each(func(i int, s *goquery.Selection) {
		g := strings.TrimSpace(s.Text())
		if g != "" {
			genres = append(genres, g)
		}
	})

	img := doc.Find("div.summary_image img, .tab-summary img").First()
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
	if thumb == "" {
		thumb = strings.TrimSpace(doc.Find("meta[property='og:image']").AttrOr("content", ""))
	}

	return &MangaItem{
		Title:        title,
		Description:  desc,
		Author:       author,
		Artist:       artist,
		Status:       status,
		Genres:       genres,
		ThumbnailURL: thumb,
		URL:          mangaURL,
	}, nil
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
			aClone := a.Clone()
			aClone.Find("span.chapter-release-date, .chapter-release-date, span.ct-update, .ct-update, span.c-new-tag, .c-new-tag, i.chapter-release-date, .post-on, span.date, img").Remove()
			chName = strings.TrimSpace(aClone.Text())
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

				// Check if flagged as NEW (badge or gif)
				isNew := false
				if s != nil && s.Find("img[alt='NEW'], img[src*='new.gif'], .c-new-tag, span.c-new-tag").Length() > 0 {
					isNew = true
				}
				if !isNew && a != nil && a.Find("img[alt='NEW'], img[src*='new.gif'], .c-new-tag, span.c-new-tag").Length() > 0 {
					isNew = true
				}

				if isNew {
					dateUpload = time.Now().UnixMilli()
				} else {
					var dateStr string
					dateSelectors := "span.chapter-release-date, .chapter-release-date, span.ct-update, .ct-update, span.c-new-tag, .c-new-tag, i.chapter-release-date, .post-on, span.date"
					if s != nil {
						dateStr = strings.TrimSpace(s.Find(dateSelectors).Text())
					}
					if dateStr == "" && a != nil {
						dateStr = strings.TrimSpace(a.Find(dateSelectors).Text())
					}
					if strings.EqualFold(dateStr, "new") {
						dateUpload = time.Now().UnixMilli()
					} else if dateStr != "" {
						dateUpload = ParseSpanishOrEnglishDate(dateStr)
					}
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

	doc.Find("li.wp-manga-chapter, a.leermos, li.myfut, ul.chapter-list li").Each(func(i int, s *goquery.Selection) {
		a := s
		if !s.Is("a") {
			a = s.Find("a").First()
		}
		addChapter(s, a)
	})

	if len(chapters) == 0 {
		// 1. Try modern Madara AJAX endpoint: {mangaURL}/ajax/chapters/
		ajaxChaptersURL := strings.TrimRight(fullURL, "/") + "/ajax/chapters/"
		if ajaxHTML, errPost := FetchHTMLPost(ctx, ajaxChaptersURL, fullURL, "", nil); errPost == nil && len(ajaxHTML) > 0 {
			if ajaxDoc, errDoc := goquery.NewDocumentFromReader(strings.NewReader(ajaxHTML)); errDoc == nil {
				ajaxDoc.Find("li.wp-manga-chapter, a.leermos, li.myfut, ul.chapter-list li").Each(func(i int, s *goquery.Selection) {
					a := s
					if !s.Is("a") {
						a = s.Find("a").First()
					}
					addChapter(s, a)
				})
			}
		}
	}

	if len(chapters) == 0 {
		postID, _ := doc.Find("input.rating-post-id, input#wp-manga-chapter-post-id, input[name='postID']").Attr("value")
		if postID != "" {
			ajaxURL := h.baseURL + "/wp-admin/admin-ajax.php"
			if ajaxHTML, errPost := FetchHTMLPost(ctx, ajaxURL, fullURL, "application/x-www-form-urlencoded", strings.NewReader("action=manga_get_chapters&manga="+postID)); errPost == nil && len(ajaxHTML) > 0 {
				if ajaxDoc, errDoc := goquery.NewDocumentFromReader(strings.NewReader(ajaxHTML)); errDoc == nil {
					ajaxDoc.Find("li.wp-manga-chapter").Each(func(i int, s *goquery.Selection) {
						a := s.Find("a").First()
						addChapter(s, a)
					})
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
