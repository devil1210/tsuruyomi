package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func init() {
	// Register MangaThemesia family sources
	Register(3953107753690791431, NewMangaThemesiaHandler("Uchuujin Projects", "https://uchuujinmangas.com"))
	Register(5943212552955476721, NewMangaThemesiaHandler("ManhwaWeb", "https://manhwa-web.com"))
	Register(1598762687295904337, NewMangaThemesiaHandler("BokugenTranslation", "https://bokugents.com"))
	Register(829053423099062956, NewMangaThemesiaHandler("Mangas No Sekai", "https://mangasnosekai.com"))
	Register(698913079501532785, NewMangaThemesiaHandler("RavenManga", "https://ravenmanga.com"))
}

type MangaThemesiaHandler struct {
	name    string
	baseURL string
}

func NewMangaThemesiaHandler(name, baseURL string) *MangaThemesiaHandler {
	return &MangaThemesiaHandler{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (h *MangaThemesiaHandler) Name() string {
	return h.name
}

func (h *MangaThemesiaHandler) GetPopularManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := fmt.Sprintf("%s/manga/?order=popular&page=%d", h.baseURL, page)
	return h.fetchMangaList(ctx, targetURL)
}

func (h *MangaThemesiaHandler) GetLatestManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := fmt.Sprintf("%s/manga/?order=update&page=%d", h.baseURL, page)
	return h.fetchMangaList(ctx, targetURL)
}

func (h *MangaThemesiaHandler) SearchManga(ctx context.Context, query string, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	targetURL := fmt.Sprintf("%s/page/%d/?s=%s", h.baseURL, page, url.QueryEscape(query))
	return h.fetchMangaList(ctx, targetURL)
}

func (h *MangaThemesiaHandler) fetchMangaList(ctx context.Context, targetURL string) ([]MangaItem, bool, error) {
	html, err := FetchHTML(ctx, targetURL, h.baseURL+"/")
	if err != nil {
		return nil, false, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, false, err
	}

	var items []MangaItem
	doc.Find("div.bsx, div.listupd div.bs, div.animposx").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a").First()
		href, _ := a.Attr("href")
		title, _ := a.Attr("title")
		if title == "" {
			title = strings.TrimSpace(s.Find("div.tt").Text())
		}
		if title == "" {
			title = strings.TrimSpace(a.Text())
		}
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

	hasNext := doc.Find("a.next, a.r, div.hpage a.r").Length() > 0
	return items, hasNext, nil
}

func (h *MangaThemesiaHandler) GetChapterList(ctx context.Context, mangaURL string) ([]ChapterItem, error) {
	fullURL := mangaURL
	if !strings.HasPrefix(fullURL, "http") {
		fullURL = h.baseURL + "/" + strings.TrimLeft(mangaURL, "/")
	}

	html, err := FetchHTML(ctx, fullURL, h.baseURL+"/")
	if err != nil {
		return nil, fmt.Errorf("fetching MangaThemesia manga details: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var chapters []ChapterItem
	doc.Find("#chapterlist ul li, ul.clstyle li, div.eph-num").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a").First()
		chURL, _ := a.Attr("href")
		chName := strings.TrimSpace(s.Find("span.chapternum").Text())
		if chName == "" {
			chName = strings.TrimSpace(a.Text())
		}
		if chURL != "" && chName != "" {
			cleanURL := chURL
			if strings.HasPrefix(cleanURL, h.baseURL) {
				cleanURL = strings.TrimPrefix(cleanURL, h.baseURL)
			}
			chapters = append(chapters, ChapterItem{
				Name:    chName,
				URL:     cleanURL,
				RealURL: chURL,
			})
		}
	})

	total := len(chapters)
	for i := range chapters {
		chapters[i].SourceOrder = total - 1 - i
	}

	return chapters, nil
}

var reTsReader = regexp.MustCompile(`ts_reader\.run\((\{.*?\})\);`)

type tsReaderData struct {
	Sources []struct {
		Source string   `json:"source"`
		Images []string `json:"images"`
	} `json:"sources"`
}

func (h *MangaThemesiaHandler) GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error) {
	fullURL := chapterURL
	if !strings.HasPrefix(fullURL, "http") {
		fullURL = h.baseURL + "/" + strings.TrimLeft(chapterURL, "/")
	}

	html, err := FetchHTML(ctx, fullURL, h.baseURL+"/")
	if err != nil {
		return nil, fmt.Errorf("fetching chapter page: %w", err)
	}

	// 1. Primary method: ts_reader.run({...}) JSON script
	matches := reTsReader.FindStringSubmatch(html)
	if len(matches) >= 2 {
		var ts tsReaderData
		if err := json.Unmarshal([]byte(matches[1]), &ts); err == nil && len(ts.Sources) > 0 && len(ts.Sources[0].Images) > 0 {
			var pages []string
			for _, img := range ts.Sources[0].Images {
				cleanURL := strings.ReplaceAll(img, `\/`, `/`)
				pages = append(pages, cleanURL)
			}
			return pages, nil
		}
	}

	// 2. Fallback method: DOM parse #readerarea img
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err == nil {
		var pages []string
		doc.Find("#readerarea img").Each(func(i int, s *goquery.Selection) {
			src, exists := s.Attr("data-src")
			if !exists || src == "" {
				src, exists = s.Attr("data-lazy-src")
			}
			if !exists || src == "" {
				src, exists = s.Attr("src")
			}
			if exists && src != "" && !strings.Contains(src, "logo") && !strings.Contains(src, "banner") {
				pages = append(pages, strings.TrimSpace(src))
			}
		})

		if len(pages) > 0 {
			return pages, nil
		}
	}

	return nil, fmt.Errorf("no pages found in MangaThemesia chapter %s", fullURL)
}
