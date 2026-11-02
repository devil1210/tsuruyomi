package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	ManhwaWebSourceID = 5943212552955476721
	manhwaWebAPIBase  = "https://api.manhwaweb.com"
	manhwaWebSiteBase = "https://manhwaweb.com"
)

func init() {
	Register(ManhwaWebSourceID, NewManhwaWebHandler())
}

type ManhwaWebHandler struct {
	client *http.Client
}

func NewManhwaWebHandler() *ManhwaWebHandler {
	return &ManhwaWebHandler{
		client: &http.Client{Timeout: 12 * time.Second},
	}
}

func (h *ManhwaWebHandler) Name() string {
	return "ManhwaWeb"
}

func (h *ManhwaWebHandler) doJSON(ctx context.Context, apiURL string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", manhwaWebSiteBase+"/")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d from %s", resp.StatusCode, apiURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

func (h *ManhwaWebHandler) GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error) {
	slug := strings.Trim(chapterURL, "/")
	slug = strings.TrimPrefix(slug, "leer/")
	if idx := strings.LastIndex(slug, "/"); idx != -1 {
		slug = slug[idx+1:]
	}

	apiURL := fmt.Sprintf("%s/chapters/see/%s", manhwaWebAPIBase, slug)

	var payload struct {
		Chapter struct {
			Img []string `json:"img"`
		} `json:"chapter"`
	}

	if err := h.doJSON(ctx, apiURL, &payload); err != nil {
		return nil, fmt.Errorf("manhwaweb page list failed: %w", err)
	}

	var pages []string
	for _, img := range payload.Chapter.Img {
		img = strings.TrimSpace(img)
		if strings.HasPrefix(img, "http") {
			pages = append(pages, img)
		}
	}

	return pages, nil
}

func (h *ManhwaWebHandler) GetChapterList(ctx context.Context, mangaURL string) ([]ChapterItem, error) {
	slug := strings.Trim(mangaURL, "/")
	slug = strings.TrimPrefix(slug, "manhwa/")
	if idx := strings.LastIndex(slug, "/"); idx != -1 {
		slug = slug[idx+1:]
	}

	apiURL := fmt.Sprintf("%s/manhwa/see/%s", manhwaWebAPIBase, slug)

	var payload struct {
		Chapters []struct {
			Chapter  float32 `json:"chapter"`
			Link     string  `json:"link"`
			Create   int64   `json:"create"`
			Versions []struct {
				Link   string `json:"link"`
				Create int64  `json:"create"`
				Joint  []struct {
					Name string `json:"name"`
				} `json:"joint"`
			} `json:"versions"`
		} `json:"chapters"`
	}

	if err := h.doJSON(ctx, apiURL, &payload); err != nil {
		return nil, fmt.Errorf("manhwaweb chapter list failed: %w", err)
	}

	var chapters []ChapterItem
	for _, ch := range payload.Chapters {
		chLink := ch.Link
		chCreate := ch.Create
		scanlator := "ManhwaWeb"

		if len(ch.Versions) > 0 {
			v := ch.Versions[0]
			if v.Link != "" {
				chLink = v.Link
			}
			if v.Create > 0 {
				chCreate = v.Create
			}
			if len(v.Joint) > 0 && v.Joint[0].Name != "" {
				scanlator = v.Joint[0].Name
			}
		}

		cleanLink := chLink
		if u, err := url.Parse(cleanLink); err == nil && u.Path != "" {
			cleanLink = u.Path
		}
		if !strings.HasPrefix(cleanLink, "/") {
			cleanLink = "/" + cleanLink
		}

		chapters = append(chapters, ChapterItem{
			Name:          fmt.Sprintf("Capítulo %v", ch.Chapter),
			URL:           cleanLink,
			ChapterNumber: ch.Chapter,
			DateUpload:    chCreate,
			Scanlator:     scanlator,
		})
	}

	return chapters, nil
}

func (h *ManhwaWebHandler) SearchManga(ctx context.Context, query string, page int) ([]MangaItem, bool, error) {
	if page < 1 {
		page = 1
	}
	apiURL := fmt.Sprintf("%s/manhwa/library?buscar=%s&page=%d", manhwaWebAPIBase, url.QueryEscape(query), page-1)

	var payload struct {
		Data []struct {
			ID          string `json:"_id"`
			TheRealName string `json:"the_real_name"`
			NameEsp     string `json:"name_esp"`
			Imagen      string `json:"_imagen"`
			Status      string `json:"_status"`
			Sinopsis    string `json:"_sinopsis"`
			Tipo        string `json:"_tipo"`
		} `json:"data"`
		Next bool `json:"next"`
	}

	if err := h.doJSON(ctx, apiURL, &payload); err != nil {
		return nil, false, err
	}

	var items []MangaItem
	for _, m := range payload.Data {
		title := m.TheRealName
		if title == "" {
			title = m.NameEsp
		}
		status := 1 // Ongoing
		if m.Status == "finalizado" {
			status = 2 // Completed
		}

		items = append(items, MangaItem{
			SourceID:     ManhwaWebSourceID,
			Title:        title,
			URL:          "manhwa/" + m.ID,
			ThumbnailURL: m.Imagen,
			Status:       status,
			Description:  m.Sinopsis,
		})
	}

	return items, payload.Next, nil
}

func (h *ManhwaWebHandler) GetPopularManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	apiURL := fmt.Sprintf("%s/manhwa/nuevos", manhwaWebAPIBase)

	var payload struct {
		Ultimos []struct {
			ID          string `json:"_id"`
			TheRealName string `json:"the_real_name"`
			Imagen      string `json:"_imagen"`
			Tipo        string `json:"_tipo"`
		} `json:"utimos_mangas_creados"`
		Ultimos18 []struct {
			ID          string `json:"_id"`
			TheRealName string `json:"the_real_name"`
			Imagen      string `json:"_imagen"`
			Tipo        string `json:"_tipo"`
		} `json:"utimos_mangas_creados_18"`
	}

	if err := h.doJSON(ctx, apiURL, &payload); err != nil {
		return nil, false, err
	}

	var items []MangaItem
	seen := make(map[string]bool)

	add := func(id, title, img string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		items = append(items, MangaItem{
			SourceID:     ManhwaWebSourceID,
			Title:        title,
			URL:          "manhwa/" + id,
			ThumbnailURL: img,
			Status:       1,
		})
	}

	for _, m := range payload.Ultimos {
		add(m.ID, m.TheRealName, m.Imagen)
	}
	for _, m := range payload.Ultimos18 {
		add(m.ID, m.TheRealName, m.Imagen)
	}

	return items, false, nil
}

func (h *ManhwaWebHandler) GetLatestManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	apiURL := fmt.Sprintf("%s/latest/new-manhwa", manhwaWebAPIBase)

	var payload struct {
		Manhwas struct {
			Items []struct {
				IDManhwa   string `json:"id_manhwa"`
				NameManhwa string `json:"name_manhwa"`
				Img        string `json:"img"`
			} `json:"_manhwas"`
		} `json:"manhwas"`
	}

	if err := h.doJSON(ctx, apiURL, &payload); err != nil {
		return nil, false, err
	}

	var items []MangaItem
	seen := make(map[string]bool)

	for _, m := range payload.Manhwas.Items {
		if m.IDManhwa == "" || seen[m.IDManhwa] {
			continue
		}
		seen[m.IDManhwa] = true
		items = append(items, MangaItem{
			SourceID:     ManhwaWebSourceID,
			Title:        m.NameManhwa,
			URL:          "manhwa/" + m.IDManhwa,
			ThumbnailURL: m.Img,
			Status:       1,
		})
	}

	return items, false, nil
}
