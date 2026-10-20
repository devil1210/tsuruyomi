package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
	doc.Find("li.wp-manga-chapter, a.leermos").Each(func(i int, s *goquery.Selection) {
		a := s
		if !s.Is("a") {
			a = s.Find("a").First()
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
			chapters = append(chapters, ChapterItem{
				Name:        chName,
				URL:         cleanURL,
				RealURL:     chURL,
				SourceOrder: len(chapters),
			})
		}
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
							chapters = append(chapters, ChapterItem{
								Name:        chName,
								URL:         cleanURL,
								RealURL:     chURL,
								SourceOrder: len(chapters),
							})
						}
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
							chURL, _ := a.Attr("href")
							chName := strings.TrimSpace(a.Text())
							if chURL != "" && chName != "" {
								cleanURL := chURL
								if strings.HasPrefix(cleanURL, h.baseURL) {
									cleanURL = strings.TrimPrefix(cleanURL, h.baseURL)
								}
								chapters = append(chapters, ChapterItem{
									Name:        chName,
									URL:         cleanURL,
									RealURL:     chURL,
									SourceOrder: len(chapters),
								})
							}
						})
					}
				}
			}
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
			if exists && src != "" && !strings.Contains(src, "logo") && !strings.Contains(src, "ads") && !strings.Contains(src, "icon") {
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
