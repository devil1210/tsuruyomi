package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func init() {
	// Register MangaThemesia family sources
	Register(3953107753690791431, NewMangaThemesiaHandler("Uchuujin Projects", "https://uchuujinmangas.com"))
	Register(5943212552955476721, NewMangaThemesiaHandler("ManhwaWeb", "https://manhwa-web.com"))
	Register(1136717301008230074, NewMangaThemesiaHandler("Submanhwa", "https://submanhwa.com"))
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
