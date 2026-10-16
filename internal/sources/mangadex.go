package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

func init() {
	// Register MangaDex across language variants
	dex := &MangaDexHandler{}
	Register(6400665728063187402, dex) // es
	Register(2499283573021220255, dex) // en
	Register(4938773340256184018, dex) // es-419
	Register(4638673959522768501, dex) // general
}

type MangaDexHandler struct{}

func (h *MangaDexHandler) Name() string {
	return "MangaDex"
}

var reUUID = regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)

type mangaDexAtHomeResponse struct {
	Result  string `json:"result"`
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		Data      []string `json:"data"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
}

func (h *MangaDexHandler) GetPageList(ctx context.Context, chapterURL string, mangaURL string) ([]string, error) {
	chapterUUID := reUUID.FindString(chapterURL)
	if chapterUUID == "" {
		return nil, fmt.Errorf("could not extract chapter UUID from MangaDex URL: %s", chapterURL)
	}

	apiURL := fmt.Sprintf("https://api.mangadex.org/at-home/server/%s", chapterUUID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Tsuruyomi/1.0 (https://github.com/Suwayomi)")
	req.Header.Set("Accept", "application/json")

	resp, err := DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling MangaDex at-home API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaDex API returned status %d for chapter %s", resp.StatusCode, chapterUUID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res mangaDexAtHomeResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("parsing MangaDex JSON: %w", err)
	}

	if res.Result != "ok" || len(res.Chapter.Data) == 0 {
		return nil, fmt.Errorf("invalid response from MangaDex API: %s", res.Result)
	}

	pages := make([]string, len(res.Chapter.Data))
	for i, fn := range res.Chapter.Data {
		pages[i] = fmt.Sprintf("%s/data/%s/%s", res.BaseURL, res.Chapter.Hash, fn)
	}

	return pages, nil
}
