package sources

import (
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func init() {
	// Register Madara family sources
	Register(8701445043633550960, NewMadaraHandler("LeerManhwas", "https://leermanhwas.com"))
	Register(1598762687295904337, NewMadaraHandler("BokugenTranslation", "https://bokugentranslation.com"))
	Register(3044113561993269522, NewMadaraHandler("ApollComics", "https://apollcomics.xyz"))
	Register(4647982373411241498, NewMadaraHandler("DragonTranslation.org", "https://dragontranslation.org"))
	Register(1814471239661920836, NewMadaraHandler("Cerberus Series", "https://cerberuseries.com"))
	Register(3548601371328353875, NewMadaraHandler("Manhwa-Latino", "https://manhwalatino.com"))
	Register(8311505150576608267, NewMadaraHandler("Hades no Fansub", "https://hadesnofansub.com"))
	Register(6757330205319558057, NewMadaraHandler("Bega Translation", "https://begatranslation.com"))
}

type MadaraHandler struct {
	name    string
	baseURL string
}

func NewMadaraHandler(name, baseURL string) *MadaraHandler {
	return &MadaraHandler{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (h *MadaraHandler) Name() string {
	return h.name
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
