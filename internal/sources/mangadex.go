package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

func init() {
	// Register MangaDex across language variants
	Register(6400665728063187402, NewMangaDexHandler(6400665728063187402)) // es
	Register(2499283573021220255, NewMangaDexHandler(2499283573021220255)) // en
	Register(4938773340256184018, NewMangaDexHandler(4938773340256184018)) // es-419
	Register(4638673959522768501, NewMangaDexHandler(4638673959522768501)) // general
}

type MangaDexHandler struct {
	sourceID int64
}

func NewMangaDexHandler(sourceID int64) *MangaDexHandler {
	return &MangaDexHandler{sourceID: sourceID}
}

func (h *MangaDexHandler) Name() string {
	return "MangaDex"
}

func (h *MangaDexHandler) GetPopularManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	dexItems, hasNext, err := FetchMangaDexOnline(ctx, h.sourceID, "", page, 20)
	if err != nil {
		return nil, false, err
	}
	items := make([]MangaItem, len(dexItems))
	for i, d := range dexItems {
		items[i] = MangaItem{
			ID:           d.ID,
			Title:        d.Title,
			Description:  d.Description,
			Status:       d.Status,
			ThumbnailURL: d.ThumbnailURL,
			URL:          d.URL,
			SourceID:     d.SourceID,
			Genres:       d.Genres,
			Author:       d.Author,
			Artist:       d.Artist,
		}
	}
	return items, hasNext, nil
}

func (h *MangaDexHandler) GetLatestManga(ctx context.Context, page int) ([]MangaItem, bool, error) {
	return h.GetPopularManga(ctx, page)
}

func (h *MangaDexHandler) SearchManga(ctx context.Context, query string, page int) ([]MangaItem, bool, error) {
	dexItems, hasNext, err := FetchMangaDexOnline(ctx, h.sourceID, query, page, 20)
	if err != nil {
		return nil, false, err
	}
	items := make([]MangaItem, len(dexItems))
	for i, d := range dexItems {
		items[i] = MangaItem{
			ID:           d.ID,
			Title:        d.Title,
			Description:  d.Description,
			Status:       d.Status,
			ThumbnailURL: d.ThumbnailURL,
			URL:          d.URL,
			SourceID:     d.SourceID,
			Genres:       d.Genres,
			Author:       d.Author,
			Artist:       d.Artist,
		}
	}
	return items, hasNext, nil
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

func (h *MangaDexHandler) GetChapterList(ctx context.Context, mangaURL string) ([]ChapterItem, error) {
	mangaUUID := reUUID.FindString(mangaURL)
	if mangaUUID == "" {
		return nil, fmt.Errorf("could not extract manga UUID from MangaDex URL: %s", mangaURL)
	}

	apiURL := fmt.Sprintf("https://api.mangadex.org/manga/%s/feed?limit=500&order[chapter]=desc&includes[]=scanlation_group", mangaUUID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Tsuruyomi/1.0 (https://github.com/Suwayomi)")
	req.Header.Set("Accept", "application/json")

	resp, err := DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling MangaDex chapters feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaDex chapters feed returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Chapter   string `json:"chapter"`
				Title     string `json:"title"`
				PublishAt string `json:"publishAt"`
			} `json:"attributes"`
			Relationships []struct {
				Type       string `json:"type"`
				Attributes struct {
					Name string `json:"name"`
				} `json:"attributes"`
			} `json:"relationships"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	total := len(res.Data)
	chapters := make([]ChapterItem, 0, total)
	for i, d := range res.Data {
		chNumFloat := float32(0)
		if f, err := strconv.ParseFloat(d.Attributes.Chapter, 32); err == nil {
			chNumFloat = float32(f)
		} else {
			chNumFloat = float32(total - i)
		}

		name := ""
		if d.Attributes.Chapter != "" {
			name = "Capítulo " + d.Attributes.Chapter
			if d.Attributes.Title != "" {
				name += ": " + d.Attributes.Title
			}
		} else if d.Attributes.Title != "" {
			name = d.Attributes.Title
		} else {
			name = fmt.Sprintf("Capítulo %.1f", chNumFloat)
		}

		scanlator := ""
		for _, rel := range d.Relationships {
			if rel.Type == "scanlation_group" && rel.Attributes.Name != "" {
				scanlator = rel.Attributes.Name
				break
			}
		}

		var dateUpload int64
		if t, err := time.Parse(time.RFC3339, d.Attributes.PublishAt); err == nil {
			dateUpload = t.UnixMilli()
		}

		chapters = append(chapters, ChapterItem{
			Name:          name,
			URL:           "/chapter/" + d.ID,
			RealURL:       "https://mangadex.org/chapter/" + d.ID,
			ChapterNumber: chNumFloat,
			Scanlator:     scanlator,
			DateUpload:    dateUpload,
			SourceOrder:   total - 1 - i,
		})
	}

	return chapters, nil
}

// MangaDexOnlineItem represents a manga returned by the online MangaDex API.
type MangaDexOnlineItem struct {
	ID           string
	Title        string
	Description  string
	Status       int
	ThumbnailURL string
	URL          string
	SourceID     int64
	Genres       []string
	Author       string
	Artist       string
}

// FetchMangaDexOnline retrieves popular or searched manga directly from MangaDex API.
func FetchMangaDexOnline(ctx context.Context, sourceID int64, query string, page int, limit int) ([]MangaDexOnlineItem, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	lang := ""
	switch sourceID {
	case 6400665728063187402:
		lang = "es"
	case 2499283573021220255:
		lang = "en"
	case 4938773340256184018:
		lang = "es-la"
	}

	apiURL := fmt.Sprintf("https://api.mangadex.org/manga?limit=%d&offset=%d&includes[]=cover_art&includes[]=author&includes[]=artist", limit, offset)
	if query != "" {
		apiURL += "&title=" + url.QueryEscape(query)
	} else {
		apiURL += "&order[followedCount]=desc"
	}
	if lang != "" {
		apiURL += "&availableTranslatedLanguage[]=" + lang
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", "Tsuruyomi/1.0 (https://github.com/Suwayomi)")
	req.Header.Set("Accept", "application/json")

	resp, err := DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("MangaDex API status %d", resp.StatusCode)
	}

	var res struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Title       map[string]string `json:"title"`
				Description map[string]string `json:"description"`
				Status      string            `json:"status"`
				Tags        []struct {
					Attributes struct {
						Name map[string]string `json:"name"`
					} `json:"attributes"`
				} `json:"tags"`
			} `json:"attributes"`
			Relationships []struct {
				ID         string `json:"id"`
				Type       string `json:"type"`
				Attributes struct {
					FileName string `json:"fileName"`
					Name     string `json:"name"`
				} `json:"attributes"`
			} `json:"relationships"`
		} `json:"data"`
		Total int `json:"total"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return nil, false, err
	}

	var items []MangaDexOnlineItem
	for _, d := range res.Data {
		title := ""
		if t, ok := d.Attributes.Title["en"]; ok && t != "" {
			title = t
		} else if t, ok := d.Attributes.Title["es"]; ok && t != "" {
			title = t
		} else if t, ok := d.Attributes.Title["es-la"]; ok && t != "" {
			title = t
		} else {
			for _, v := range d.Attributes.Title {
				if v != "" {
					title = v
					break
				}
			}
		}

		desc := ""
		if dText, ok := d.Attributes.Description["es"]; ok && dText != "" {
			desc = dText
		} else if dText, ok := d.Attributes.Description["es-la"]; ok && dText != "" {
			desc = dText
		} else if dText, ok := d.Attributes.Description["en"]; ok && dText != "" {
			desc = dText
		}

		var coverFile, author, artist string
		for _, rel := range d.Relationships {
			switch rel.Type {
			case "cover_art":
				coverFile = rel.Attributes.FileName
			case "author":
				if author == "" {
					author = rel.Attributes.Name
				}
			case "artist":
				if artist == "" {
					artist = rel.Attributes.Name
				}
			}
		}

		thumb := ""
		if coverFile != "" {
			thumb = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s.256.jpg", d.ID, coverFile)
		}

		var genres []string
		for _, tag := range d.Attributes.Tags {
			if n, ok := tag.Attributes.Name["en"]; ok && n != "" {
				genres = append(genres, n)
			}
		}

		statusInt := 1 // ongoing
		switch d.Attributes.Status {
		case "completed":
			statusInt = 2
		case "hiatus":
			statusInt = 6
		case "cancelled":
			statusInt = 5
		}

		items = append(items, MangaDexOnlineItem{
			ID:           d.ID,
			Title:        title,
			Description:  desc,
			Status:       statusInt,
			ThumbnailURL: thumb,
			URL:          "/title/" + d.ID,
			SourceID:     sourceID,
			Genres:       genres,
			Author:       author,
			Artist:       artist,
		})
	}

	hasNext := offset+limit < res.Total
	return items, hasNext, nil
}
