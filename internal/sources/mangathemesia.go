package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func init() {
	// Register MangaThemesia family sources
	Register(3953107753690791431, NewMangaThemesiaHandler("Uchuujin Projects", "https://uchuujinmangas.com"))
	Register(1598762687295904337, NewMangaThemesiaHandler("BokugenTranslation", "https://bokugents.com"))
	Register(1814471239661920836, NewMangaThemesiaHandler("Cerberus Series", "https://legionscans.com/wp"))
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
	seen := make(map[string]bool)

	doc.Find("div.bsx, div.animposx, div.listupd div.bs").Each(func(i int, s *goquery.Selection) {
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

		mangaURL := href
		if strings.HasPrefix(mangaURL, h.baseURL) {
			mangaURL = strings.TrimPrefix(mangaURL, h.baseURL)
		}

		if seen[mangaURL] {
			return
		}
		seen[mangaURL] = true

		img := s.Find("img").First()
		thumb := ""
		for _, attr := range []string{"data-lazy-src", "data-src", "data-original", "data-wpfc-original-src", "data-cfsrc", "src"} {
			if val, ok := img.Attr(attr); ok && strings.TrimSpace(val) != "" {
				thumb = strings.TrimSpace(val)
				break
			}
		}

		if strings.HasPrefix(thumb, "//") {
			thumb = "https:" + thumb
		} else if strings.HasPrefix(thumb, "/") {
			thumb = h.baseURL + thumb
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

func (h *MangaThemesiaHandler) GetMangaDetails(ctx context.Context, mangaURL string) (*MangaItem, error) {
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

	title := strings.TrimSpace(doc.Find("h1.entry-title").Text())
	thumb := ""
	img := doc.Find("div.thumb img, div.bigcontent img").First()
	for _, attr := range []string{"data-lazy-src", "data-src", "data-original", "data-wpfc-original-src", "data-cfsrc", "src"} {
		if val, ok := img.Attr(attr); ok && strings.TrimSpace(val) != "" {
			thumb = strings.TrimSpace(val)
			break
		}
	}
	if strings.HasPrefix(thumb, "//") {
		thumb = "https:" + thumb
	} else if strings.HasPrefix(thumb, "/") {
		thumb = h.baseURL + thumb
	}

	desc := strings.TrimSpace(doc.Find("div.entry-content[itemprop=\"description\"], div.desc, div.synopsis").Text())

	status := 1
	doc.Find(".imptdt, .tsinfo .imptdt").Each(func(i int, s *goquery.Selection) {
		text := strings.ToLower(s.Text())
		if strings.Contains(text, "status") || strings.Contains(text, "estado") {
			val := strings.ToLower(s.Find("i").Text())
			if strings.Contains(val, "complet") || strings.Contains(val, "finaliz") {
				status = 2
			} else if strings.Contains(val, "cancel") {
				status = 5
			} else if strings.Contains(val, "paus") || strings.Contains(val, "hiat") {
				status = 6
			}
		}
	})

	var author, artist string
	doc.Find(".imptdt, .tsinfo .imptdt").Each(func(i int, s *goquery.Selection) {
		text := strings.ToLower(s.Text())
		if strings.Contains(text, "autor") || strings.Contains(text, "author") {
			author = strings.TrimSpace(s.Find("i").Text())
		}
		if strings.Contains(text, "artista") || strings.Contains(text, "artist") {
			artist = strings.TrimSpace(s.Find("i").Text())
		}
	})

	var genres []string
	doc.Find(".mgen a, .seriestugenre a").Each(func(i int, s *goquery.Selection) {
		g := strings.TrimSpace(s.Text())
		if g != "" {
			genres = append(genres, g)
		}
	})

	cleanURL := mangaURL
	if strings.HasPrefix(cleanURL, h.baseURL) {
		cleanURL = strings.TrimPrefix(cleanURL, h.baseURL)
	}

	return &MangaItem{
		Title:        title,
		URL:          cleanURL,
		ThumbnailURL: thumb,
		Description:  desc,
		Status:       status,
		Author:       author,
		Artist:       artist,
		Genres:       genres,
	}, nil
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
	seenCh := make(map[string]bool)
	doc.Find("#chapterlist ul li, ul.clstyle li").Each(func(i int, s *goquery.Selection) {
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
			if !seenCh[cleanURL] {
				seenCh[cleanURL] = true

				var chNum float32
				dataNum := strings.TrimSpace(s.AttrOr("data-num", ""))
				if dataNum != "" {
					if f, err := strconv.ParseFloat(dataNum, 32); err == nil {
						chNum = float32(f)
					}
				}
				if chNum == 0 {
					chNum = ParseChapterNumber(chName)
				}

				dateStr := strings.TrimSpace(s.Find("span.chapterdate").Text())
				dateUpload := ParseSpanishOrEnglishDate(dateStr)
				if dateUpload == 0 {
					for _, layout := range []string{"2 January, 2006", "January 2, 2006", "02/01/2006", "2006-01-02", "2006-01-02 15:04:05"} {
						if t, err := time.Parse(layout, dateStr); err == nil {
							dateUpload = t.UnixMilli()
							break
						}
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
	})

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
			src = strings.TrimSpace(src)
			if exists && src != "" && !strings.HasPrefix(src, "data:") && !strings.Contains(src, "blank.gif") && !strings.Contains(src, "logo") && !strings.Contains(src, "banner") {
				if strings.HasPrefix(src, "//") {
					src = "https:" + src
				} else if strings.HasPrefix(src, "/") {
					src = h.baseURL + src
				}
				pages = append(pages, src)
			}
		})

		if len(pages) > 0 {
			return pages, nil
		}
	}

	return nil, fmt.Errorf("no pages found in MangaThemesia chapter %s", fullURL)
}
