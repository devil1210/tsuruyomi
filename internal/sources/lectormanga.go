package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

func init() {
	// Register LectorManga.lat with its canonical Tachiyomi source ID
	Register(7178038133263703638, &LectorMangaHandler{})
}

type LectorMangaHandler struct{}

func (h *LectorMangaHandler) Name() string {
	return "LectorManga.lat"
}

var reDigits = regexp.MustCompile(`\d+`)

type zeroComicsResponse struct {
	Data struct {
		ID      int      `json:"id"`
		Numero  float64  `json:"numero"`
		Paginas []string `json:"paginas"`
	} `json:"data"`
}

func (h *LectorMangaHandler) GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error) {
	// Extract the chapter numeric ID
	matches := reDigits.FindAllString(chapterURL, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("could not extract chapter ID from URL: %s", chapterURL)
	}
	chapterID := matches[len(matches)-1] // Take last numeric sequence

	apiURL := fmt.Sprintf("https://api.zerocomics.net/api/capitulos/%s", chapterID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://lector-mangas.lat")
	req.Header.Set("Referer", "https://lector-mangas.lat/")

	resp, err := DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ZeroComics API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ZeroComics API returned status %d for chapter %s", resp.StatusCode, chapterID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res zeroComicsResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("parsing ZeroComics JSON: %w", err)
	}

	if len(res.Data.Paginas) == 0 {
		return nil, fmt.Errorf("no pages returned from ZeroComics API for chapter %s", chapterID)
	}

	// Clean escaped slashes in URLs if any
	pages := make([]string, len(res.Data.Paginas))
	for i, p := range res.Data.Paginas {
		pages[i] = strings.ReplaceAll(p, `\/`, `/`)
	}

	return pages, nil
}
