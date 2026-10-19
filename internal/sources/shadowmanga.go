package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"tsuruyomi/internal/util"
)


func init() {
	// Register Shadow Manga native Go handlers with canonical source IDs
	Register(5649269152264667286, NewShadowMangaHandler("Shadow Manga", "https://shademanga.com", false))
	Register(1340622632921674920, NewShadowMangaHandler("Shadow Manga (+18)", "https://shademanga.com", true))
}

type ShadowMangaHandler struct {
	name    string
	baseURL string
	isNSFW  bool
}

func NewShadowMangaHandler(name, baseURL string, isNSFW bool) *ShadowMangaHandler {
	return &ShadowMangaHandler{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		isNSFW:  isNSFW,
	}
}

func (h *ShadowMangaHandler) Name() string {
	return h.name
}

type shadowSeriesSummary struct {
	ID            int64  `json:"id"`
	PublicID      string `json:"publicId"`
	Titulo        string `json:"titulo"`
	Autor         string `json:"autor"`
	PortadaURL    string `json:"portadaUrl"`
	EsMayorDeEdad bool   `json:"esMayorDeEdad"`
	Generos       string `json:"generos"`
}

func (h *ShadowMangaHandler) GetPopularManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	apiURL := fmt.Sprintf("%s/api/series-locales?page=%d&limit=24", h.baseURL, page)
	return h.fetchSeriesList(ctx, apiURL)
}

func (h *ShadowMangaHandler) GetLatestManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	return h.GetPopularManga(ctx, page)
}

func (h *ShadowMangaHandler) SearchManga(ctx context.Context, query string, page int) ([]MangaItem, bool, error) {
	if page <= 0 {
		page = 1
	}
	apiURL := fmt.Sprintf("%s/api/series-locales?page=%d&limit=24&q=%s", h.baseURL, page, url.QueryEscape(query))
	return h.fetchSeriesList(ctx, apiURL)
}

func (h *ShadowMangaHandler) fetchSeriesList(ctx context.Context, apiURL string) ([]MangaItem, bool, error) {
	var list []shadowSeriesSummary
	if err := h.doJSONGet(ctx, apiURL, &list); err != nil {
		return nil, false, err
	}
	var items []MangaItem
	for _, it := range list {
		if it.EsMayorDeEdad != h.isNSFW {
			continue
		}
		var genres []string
		if it.Generos != "" {
			for _, g := range strings.Split(it.Generos, ",") {
				if tg := strings.TrimSpace(g); tg != "" {
					genres = append(genres, tg)
				}
			}
		}
		items = append(items, MangaItem{
			Title:        it.Titulo,
			URL:          fmt.Sprintf("serie/local/%d", it.ID),
			ThumbnailURL: it.PortadaURL,
			Author:       it.Autor,
			Artist:       it.Autor,
			Genres:       genres,
			Status:       1,
		})
	}
	return items, len(list) >= 24, nil
}


type shadowChapter struct {
	ID             int64   `json:"id"`
	PublicID       string  `json:"publicId"`
	NumeroCapitulo float32 `json:"numeroCapitulo"`
	Titulo         *string `json:"titulo"`
	FechaSubida    string  `json:"fechaSubida"`
	Orden          int     `json:"orden"`
}

type shadowSeriesResponse struct {
	ID        int64           `json:"id"`
	Titulo    string          `json:"titulo"`
	Capitulos []shadowChapter `json:"capitulos"`
}

type shadowOneshotResponse struct {
	SMID         int64  `json:"smId"`
	Titulo       string `json:"titulo"`
	CapituloID   int64  `json:"capituloId"`
	TotalPaginas int    `json:"totalPaginas"`
}

type shadowPagesResponse struct {
	Paginas []string `json:"paginas"`
}

func (h *ShadowMangaHandler) doJSONGet(ctx context.Context, targetURL string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", h.baseURL+"/")

	resp, err := DefaultClient.Do(req)
	if err != nil {
		if AppConfig != nil && AppConfig.FlareSolverr.Enabled {
			log.Printf("[SHADOWMANGA] GET network error %v for %s. Requesting FlareSolverr bypass...", err, targetURL)
			rawHTML, solveErr := util.SolveAndFetchHTML(ctx, AppConfig, targetURL, h.baseURL+"/")
			if solveErr == nil && len(rawHTML) > 0 {
				cleanJSON := util.ExtractJSONFromHTML(rawHTML)
				return json.Unmarshal([]byte(cleanJSON), target)
			}
		}
		return err
	}
	defer resp.Body.Close()

	if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == 429) && AppConfig != nil && AppConfig.FlareSolverr.Enabled {
		log.Printf("[SHADOWMANGA] Cloudflare challenge encountered for %s (HTTP %d). Requesting FlareSolverr bypass...", targetURL, resp.StatusCode)
		rawHTML, solveErr := util.SolveAndFetchHTML(ctx, AppConfig, targetURL, h.baseURL+"/")
		if solveErr == nil && len(rawHTML) > 0 {
			cleanJSON := util.ExtractJSONFromHTML(rawHTML)
			return json.Unmarshal([]byte(cleanJSON), target)
		}
		log.Printf("[SHADOWMANGA] FlareSolverr challenge bypass warning: %v", solveErr)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP error %d fetching %s", resp.StatusCode, targetURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

func (h *ShadowMangaHandler) GetChapterList(ctx context.Context, mangaURL string) ([]ChapterItem, error) {
	cleanURL := strings.TrimSpace(mangaURL)
	cleanURL = strings.TrimPrefix(cleanURL, h.baseURL)
	cleanURL = strings.Trim(cleanURL, "/")

	// Check if this is an adult oneshot (ext/...)
	if strings.HasPrefix(cleanURL, "ext/") || strings.HasPrefix(cleanURL, "adultos/manga/o/") {
		smID := strings.TrimPrefix(cleanURL, "adultos/manga/o/")
		smID = strings.TrimPrefix(smID, "ext/")
		smID = strings.Split(smID, "/")[0]

		apiURL := fmt.Sprintf("%s/api/series-locales/ext/%s", h.baseURL, smID)
		var oneshot shadowOneshotResponse
		if err := h.doJSONGet(ctx, apiURL, &oneshot); err != nil {
			return nil, fmt.Errorf("fetching oneshot %s: %w", smID, err)
		}

		chID := oneshot.CapituloID
		if chID <= 0 {
			chID = 1
		}

		return []ChapterItem{
			{
				Name:          "Capítulo 1",
				URL:           fmt.Sprintf("ext/%s/%d", smID, chID),
				RealURL:       fmt.Sprintf("%s/adultos/manga/o/%s", h.baseURL, smID),
				ChapterNumber: 1,
				DateUpload:    0,
				SourceOrder:   1,
			},
		}, nil
	}

	// Standard local series
	idStr := cleanURL
	if strings.HasPrefix(idStr, "serie/local/") {
		idStr = strings.TrimPrefix(idStr, "serie/local/")
	}
	idStr = strings.Split(idStr, "/")[0]

	apiURL := fmt.Sprintf("%s/api/series-locales/%s", h.baseURL, idStr)
	var series shadowSeriesResponse
	if err := h.doJSONGet(ctx, apiURL, &series); err != nil {
		return nil, fmt.Errorf("fetching series %s: %w", idStr, err)
	}

	if len(series.Capitulos) == 0 {
		return nil, fmt.Errorf("no chapters found for series %s", idStr)
	}

	total := len(series.Capitulos)
	// Sort descending by chapter number / orden (newest first, Tachiyomi convention)
	sort.Slice(series.Capitulos, func(i, j int) bool {
		if series.Capitulos[i].NumeroCapitulo != series.Capitulos[j].NumeroCapitulo {
			return series.Capitulos[i].NumeroCapitulo > series.Capitulos[j].NumeroCapitulo
		}
		if series.Capitulos[i].Orden != series.Capitulos[j].Orden {
			return series.Capitulos[i].Orden > series.Capitulos[j].Orden
		}
		return series.Capitulos[i].ID > series.Capitulos[j].ID
	})

	items := make([]ChapterItem, 0, total)
	for i, ch := range series.Capitulos {
		numStr := strconv.FormatFloat(float64(ch.NumeroCapitulo), 'f', -1, 32)
		name := "Cap. " + numStr
		if ch.Titulo != nil && strings.TrimSpace(*ch.Titulo) != "" {
			name += " - " + strings.TrimSpace(*ch.Titulo)
		}

		var dateUpload int64
		if ch.FechaSubida != "" {
			if t, err := time.Parse(time.RFC3339Nano, ch.FechaSubida); err == nil {
				dateUpload = t.UnixMilli()
			} else if t, err := time.Parse(time.RFC3339, ch.FechaSubida); err == nil {
				dateUpload = t.UnixMilli()
			}
		}

		items = append(items, ChapterItem{
			Name:          name,
			URL:           fmt.Sprintf("%s/%d", idStr, ch.ID),
			RealURL:       fmt.Sprintf("%s/reader/local/%d", h.baseURL, ch.ID),
			ChapterNumber: ch.NumeroCapitulo,
			DateUpload:    dateUpload,
			SourceOrder:   total - i, // 1 for oldest chapter, total for newest chapter
		})
	}

	return items, nil
}

func (h *ShadowMangaHandler) GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error) {
	cleanChURL := strings.TrimSpace(chapterURL)
	cleanChURL = strings.TrimPrefix(cleanChURL, h.baseURL)
	cleanChURL = strings.Trim(cleanChURL, "/")

	var apiURL string
	if strings.HasPrefix(cleanChURL, "ext/") {
		parts := strings.Split(strings.TrimPrefix(cleanChURL, "ext/"), "/")
		smID := parts[0]
		apiURL = fmt.Sprintf("%s/api/series-locales/ext/%s/paginas", h.baseURL, smID)
	} else if strings.HasPrefix(cleanChURL, "reader/local/") {
		chID := strings.TrimPrefix(cleanChURL, "reader/local/")
		mangaID := strings.Trim(mangaURL, "/")
		if strings.HasPrefix(mangaID, "serie/local/") {
			mangaID = strings.TrimPrefix(mangaID, "serie/local/")
		}
		mangaID = strings.Split(mangaID, "/")[0]
		apiURL = fmt.Sprintf("%s/api/series-locales/%s/capitulos/%s/paginas", h.baseURL, mangaID, chID)
	} else if strings.Contains(cleanChURL, "/") {
		parts := strings.Split(cleanChURL, "/")
		mangaID := parts[0]
		chID := parts[1]
		apiURL = fmt.Sprintf("%s/api/series-locales/%s/capitulos/%s/paginas", h.baseURL, mangaID, chID)
	} else {
		mangaID := strings.Trim(mangaURL, "/")
		if strings.HasPrefix(mangaID, "serie/local/") {
			mangaID = strings.TrimPrefix(mangaID, "serie/local/")
		}
		mangaID = strings.Split(mangaID, "/")[0]
		apiURL = fmt.Sprintf("%s/api/series-locales/%s/capitulos/%s/paginas", h.baseURL, mangaID, cleanChURL)
	}

	var pagesResp shadowPagesResponse
	if err := h.doJSONGet(ctx, apiURL, &pagesResp); err != nil {
		return nil, fmt.Errorf("fetching pages from %s: %w", apiURL, err)
	}

	if len(pagesResp.Paginas) == 0 {
		return nil, fmt.Errorf("no pages found in %s", apiURL)
	}

	return pagesResp.Paginas, nil
}
