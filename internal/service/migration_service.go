package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/sources"
)

// MigrationReport holds the comprehensive outcome of migrating a single manga.
type MigrationReport struct {
	OldMangaID         int64    `json:"oldMangaId"`
	OldTitle           string   `json:"oldTitle"`
	OldSource          int64    `json:"oldSource"`
	OldSourceName      string   `json:"oldSourceName"`
	OldTotalChapters   int      `json:"oldTotalChapters"`
	OldReadChapters    int      `json:"oldReadChapters"`
	NewMangaID         int64    `json:"newMangaId"`
	NewTitle           string   `json:"newTitle"`
	NewSource          int64    `json:"newSource"`
	NewSourceName      string   `json:"newSourceName"`
	NewTotalChapters   int      `json:"newTotalChapters"`
	NewReadChapters    int      `json:"newReadChapters"`
	CategoriesMigrated []string `json:"categoriesMigrated"`
	Status             string   `json:"status"` // SUCCESS, SKIPPED, NOT_FOUND, ERROR
	Message            string   `json:"message"`
}

var (
	reYearTags     = regexp.MustCompile(`(?i)\(\s*\d{4}[^)]*\)|\[[^\]]+\]`)
	reExtraSpaces  = regexp.MustCompile(`\s+`)
	reSlugCleaner  = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	reMangaDexUUID = regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)
)

// CleanTitle removes year suffixes like "( 2022 )", "( 2019 )", or "[Serialización]".
func CleanTitle(title string) string {
	cleaned := reYearTags.ReplaceAllString(title, " ")
	cleaned = strings.TrimSpace(reExtraSpaces.ReplaceAllString(cleaned, " "))
	return cleaned
}

// ExtractSlug extracts and humanizes a slug from a TMO URL like "/library/manga/67351/yatara-to-tsugai...".
func ExtractSlug(tmoURL string) string {
	parts := strings.Split(strings.Trim(tmoURL, "/"), "/")
	if len(parts) >= 4 && parts[0] == "library" && parts[1] == "manga" {
		slug := parts[3]
		// Convert dashes to spaces
		words := strings.ReplaceAll(slug, "-", " ")
		return strings.TrimSpace(words)
	}
	return ""
}

// TargetMatch represents a discovered migration candidate.
type TargetMatch struct {
	SourceID     int64
	URL          string
	Title        string
	Artist       string
	Author       string
	Description  string
	Genre        string
	Status       int
	ThumbnailURL string
	Language     string
	ChapterCount int
}

// SearchMangaDexOnlineWithAggregate searches MangaDex for matching manga and checks available chapter count.
func SearchMangaDexOnlineWithAggregate(ctx context.Context, query string, targetLang string) (*TargetMatch, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("empty query")
	}

	searchURL := fmt.Sprintf("https://api.mangadex.org/manga?limit=5&includes[]=cover_art&includes[]=author&includes[]=artist&title=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Tsuruyomi/1.0 (https://github.com/Suwayomi)")
	req.Header.Set("Accept", "application/json")

	resp, err := sources.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mangadex search returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Title       map[string]string   `json:"title"`
				AltTitles   []map[string]string `json:"altTitles"`
				Description map[string]string   `json:"description"`
				Status      string              `json:"status"`
			} `json:"attributes"`
			Relationships []struct {
				Type       string `json:"type"`
				Attributes struct {
					FileName string `json:"fileName"`
					Name     string `json:"name"`
				} `json:"attributes"`
			} `json:"relationships"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	for _, d := range res.Data {
		if !isTitleMatch(query, d.Attributes.Title, d.Attributes.AltTitles) {
			continue
		}
		match, err := GetMangaDexByUUID(ctx, d.ID, targetLang)
		if err == nil && match != nil && match.ChapterCount > 0 {
			return match, nil
		}
	}

	return nil, nil
}

func isTitleMatch(query string, titles map[string]string, altTitles []map[string]string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if len(q) < 3 {
		return false
	}
	checkStr := func(cand string) bool {
		c := strings.ToLower(strings.TrimSpace(cand))
		if c == "" {
			return false
		}
		if strings.Contains(c, q) || strings.Contains(q, c) {
			return true
		}
		qWords := strings.Fields(q)
		candWords := strings.Fields(c)
		matchCount := 0
		for _, qw := range qWords {
			if len(qw) <= 2 {
				continue
			}
			for _, cw := range candWords {
				if strings.EqualFold(qw, cw) {
					matchCount++
					break
				}
			}
		}
		if len(qWords) > 0 && float64(matchCount)/float64(len(qWords)) >= 0.5 {
			return true
		}
		return false
	}

	for _, t := range titles {
		if checkStr(t) {
			return true
		}
	}
	for _, alt := range altTitles {
		for _, t := range alt {
			if checkStr(t) {
				return true
			}
		}
	}
	return false
}

// GetMangaDexByUUID fetches a manga directly by its UUID and checks available chapters in targetLang.
func GetMangaDexByUUID(ctx context.Context, uuid string, targetLang string) (*TargetMatch, error) {
	if uuid == "" {
		return nil, fmt.Errorf("empty uuid")
	}

	urlManga := fmt.Sprintf("https://api.mangadex.org/manga/%s?includes[]=cover_art&includes[]=author&includes[]=artist", uuid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlManga, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Tsuruyomi/1.0 (https://github.com/Suwayomi)")
	req.Header.Set("Accept", "application/json")

	resp, err := sources.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mangadex returned HTTP %d for UUID %s", resp.StatusCode, uuid)
	}

	var res struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Title       map[string]string   `json:"title"`
				AltTitles   []map[string]string `json:"altTitles"`
				Description map[string]string   `json:"description"`
				Status      string              `json:"status"`
			} `json:"attributes"`
			Relationships []struct {
				Type       string `json:"type"`
				Attributes struct {
					FileName string `json:"fileName"`
					Name     string `json:"name"`
				} `json:"attributes"`
			} `json:"relationships"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	// Check aggregate for chapters in targetLang
	aggURL := fmt.Sprintf("https://api.mangadex.org/manga/%s/aggregate", uuid)
	if targetLang == "es" {
		aggURL += "?translatedLanguage[]=es&translatedLanguage[]=es-la"
	} else if targetLang == "en" {
		aggURL += "?translatedLanguage[]=en"
	}

	aggReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, aggURL, nil)
	aggReq.Header.Set("User-Agent", "Tsuruyomi/1.0 (https://github.com/Suwayomi)")
	aggResp, errAgg := sources.DefaultClient.Do(aggReq)
	if errAgg != nil {
		return nil, errAgg
	}
	defer aggResp.Body.Close()

	var aggData struct {
		Volumes map[string]struct {
			Chapters map[string]interface{} `json:"chapters"`
		} `json:"volumes"`
	}
	_ = json.NewDecoder(aggResp.Body).Decode(&aggData)

	chCount := 0
	for _, v := range aggData.Volumes {
		chCount += len(v.Chapters)
	}

	if chCount == 0 {
		return nil, fmt.Errorf("0 chapters found in %s", targetLang)
	}

	d := res.Data
	title := ""
	if t, ok := d.Attributes.Title[targetLang]; ok && t != "" {
		title = t
	} else if t, ok := d.Attributes.Title["en"]; ok && t != "" {
		title = t
	} else if t, ok := d.Attributes.Title["ja-ro"]; ok && t != "" {
		title = t
	} else {
		for _, v := range d.Attributes.Title {
			if v != "" {
				title = v
				break
			}
		}
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

	sourceID := int64(6400665728063187402) // MangaDex ES
	if targetLang == "en" {
		sourceID = int64(2499283573021220255) // MangaDex EN
	}

	statusInt := 1
	if d.Attributes.Status == "completed" {
		statusInt = 2
	}

	return &TargetMatch{
		SourceID:     sourceID,
		URL:          "/manga/" + d.ID,
		Title:        title,
		Artist:       artist,
		Author:       author,
		Status:       statusInt,
		ThumbnailURL: thumb,
		Language:     targetLang,
		ChapterCount: chCount,
	}, nil
}

var KnownTmoUUIDs = map[string]string{
	"watashi to dasshutsu shimasen ka":                                "714a9a71-8aac-4b73-af59-342cb6c13d87",
	"yatara to tsugai ni nari gataru inukei osananajimi":              "d135bf1a-4359-48eb-9cb7-931cfcd01072",
	"the doggy childhood friend constantly trying to make me her mate": "d135bf1a-4359-48eb-9cb7-931cfcd01072",
	"una chica tsundere que cada día es menos tsun":                   "129c90ca-b997-4789-a748-e8765bc67a65",
	"una chica tsundere que cada dia es menos tsun":                   "129c90ca-b997-4789-a748-e8765bc67a65",
	"uso bitch senpai":                                                "42603601-3659-430f-8306-ca703a884afe",
	"una cuñada genial que apunta a la virginidad de su cuñado":       "60b4099e-1102-4492-be54-d33966021df9",
	"una cunada genial que apunta a la virginidad de su cunado":       "60b4099e-1102-4492-be54-d33966021df9",
	"uza kawaii kouhai to ichaicha suru dake no anthology comic":       "0fbf4b6f-6bae-479d-9ab7-a95ec7d2efb3",
	"just flirting with a cute annoying kouhai":                       "0fbf4b6f-6bae-479d-9ab7-a95ec7d2efb3",
	"ai-sensei no entiende el amor":                                   "2a6ab931-d8d4-453b-92b9-a81bf5574b59",
	"ai-sensei wa wakaranai":                                          "2a6ab931-d8d4-453b-92b9-a81bf5574b59",
	"las circunstancias de beber sangre de la familia hiiragi":         "182f72c5-1949-4b22-8335-a64cafd128cb",
	"hiiragi-san chi no kyuuketsu jijou":                              "182f72c5-1949-4b22-8335-a64cafd128cb",
	"the sword of princess gluttony":                                  "997d322b-22de-4875-a1a4-2a80dba21d4c",
	"boushoku-hi no ken":                                              "997d322b-22de-4875-a1a4-2a80dba21d4c",
	"boushoku hi no ken":                                              "997d322b-22de-4875-a1a4-2a80dba21d4c",
	"the dark maid dominates!":                                        "cf8c6030-6190-4dd3-a5b5-aedbab086bea",
	"the dark maid dominates":                                         "cf8c6030-6190-4dd3-a5b5-aedbab086bea",
	"yami maid ga shihai suru":                                        "cf8c6030-6190-4dd3-a5b5-aedbab086bea",
}

var KnownTmoAliases = map[string]string{
	"watashi to dasshutsu shimasen ka":                         "Watashi to Dasshutsu Shimasen ka?",
	"una chica tsundere que cada día es menos tsun":             "Ichinichi Goto ni Tsun",
	"una chica tsundere que cada dia es menos tsun":             "Ichinichi Goto ni Tsun",
	"una cuñada genial que apunta a la virginidad de su cuñado": "Gikei no Doutei",
	"una cunada genial que apunta a la virginidad de su cunado": "Gikei no Doutei",
	"ai-sensei no entiende el amor":                             "Ai-Sensei wa Wakaranai",
	"las circunstancias de beber sangre de la familia hiiragi":   "Hiiragi-san Chi no Kyuuketsu Jijou",
	"the sword of princess gluttony":                            "Boushoku-Hi no Ken",
	"the dark maid dominates!":                                  "Yami Maid ga Shihai suru",
	"the dark maid dominates":                                   "Yami Maid ga Shihai suru",
}

// FindBestMigrationTarget discovers the best target manga by cascading across sources and languages.
func FindBestMigrationTarget(ctx context.Context, oldTitle, oldURL, oldDesc string) (*TargetMatch, error) {
	cleanT := CleanTitle(oldTitle)
	slug := ExtractSlug(oldURL)

	// 0. Priority: Known exact UUID direct lookup
	for _, key := range []string{strings.ToLower(cleanT), strings.ToLower(slug)} {
		if key == "" {
			continue
		}
		if uuid, ok := KnownTmoUUIDs[key]; ok {
			// Check ES first
			if m, err := GetMangaDexByUUID(ctx, uuid, "es"); err == nil && m != nil && m.ChapterCount > 0 {
				return m, nil
			}
			// Check EN fallback
			if m, err := GetMangaDexByUUID(ctx, uuid, "en"); err == nil && m != nil && m.ChapterCount > 0 {
				return m, nil
			}
		}
	}

	searchTerms := []string{}
	// Check known alias
	lowerClean := strings.ToLower(cleanT)
	if alias, ok := KnownTmoAliases[lowerClean]; ok {
		searchTerms = append(searchTerms, alias)
	}
	lowerSlug := strings.ToLower(slug)
	if alias, ok := KnownTmoAliases[lowerSlug]; ok && alias != "" {
		searchTerms = append(searchTerms, alias)
	}

	searchTerms = append(searchTerms, cleanT)
	if slug != "" && !strings.EqualFold(slug, cleanT) {
		searchTerms = append(searchTerms, slug)
	}

	// 1. First priority: MangaDex (ES / es-la)
	for _, term := range searchTerms {
		match, err := SearchMangaDexOnlineWithAggregate(ctx, term, "es")
		if err == nil && match != nil && match.ChapterCount > 0 {
			return match, nil
		}
	}

	// 2. Second priority: MangaDex (EN) as fallback
	for _, term := range searchTerms {
		match, err := SearchMangaDexOnlineWithAggregate(ctx, term, "en")
		if err == nil && match != nil && match.ChapterCount > 0 {
			return match, nil
		}
	}

	return nil, fmt.Errorf("no online chapters found across MangaDex (ES/EN) for %q", oldTitle)
}

// MigrateSingleManga migrates a single manga from oldMangaID to a specified or auto-discovered target.
func MigrateSingleManga(ctx context.Context, cfg *config.Config, oldMangaID int64, manualTarget *TargetMatch) (*MigrationReport, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	report := &MigrationReport{
		OldMangaID: oldMangaID,
		Status:     "PENDING",
	}

	// 1. Load old manga info
	var oldURL, oldTitle, oldDesc, oldGenre, oldAuthor, oldArtist, oldThumb string
	var oldSource int64
	var oldStatus int
	err := database.DB.QueryRow(ctx, `
		SELECT source, url, title, description, genre, author, artist, thumbnail_url, status
		FROM suwayomi.manga WHERE id = $1
	`, oldMangaID).Scan(&oldSource, &oldURL, &oldTitle, &oldDesc, &oldGenre, &oldAuthor, &oldArtist, &oldThumb, &oldStatus)
	if err != nil {
		report.Status = "ERROR"
		report.Message = fmt.Sprintf("Failed to load old manga %d: %v", oldMangaID, err)
		return report, err
	}

	report.OldTitle = oldTitle
	report.OldSource = oldSource
	report.OldSourceName = sources.GetSourceName(oldSource)

	// Old chapters count
	_ = database.DB.QueryRow(ctx, `SELECT count(*) FROM suwayomi.chapter WHERE manga = $1`, oldMangaID).Scan(&report.OldTotalChapters)
	_ = database.DB.QueryRow(ctx, `
		SELECT count(*) FROM suwayomi.chapter c 
		JOIN suwayomi.chapteruser cu ON c.id = cu.chapter 
		WHERE c.manga = $1 AND cu.read = true
	`, oldMangaID).Scan(&report.OldReadChapters)

	// Old categories
	catRows, errCat := database.DB.Query(ctx, `
		SELECT c.name FROM suwayomi.category c
		JOIN suwayomi.categorymanga cm ON c.id = cm.category
		WHERE cm.manga = $1
	`, oldMangaID)
	if errCat == nil {
		for catRows.Next() {
			var catName string
			if catRows.Scan(&catName) == nil {
				report.CategoriesMigrated = append(report.CategoriesMigrated, catName)
			}
		}
		catRows.Close()
	}

	// 2. Discover target if not manually specified
	target := manualTarget
	if target == nil {
		foundTarget, errFind := FindBestMigrationTarget(ctx, oldTitle, oldURL, oldDesc)
		if errFind != nil || foundTarget == nil {
			report.Status = "NOT_FOUND"
			report.Message = fmt.Sprintf("No target source found with chapters for %q", oldTitle)
			return report, nil
		}
		target = foundTarget
	}

	report.NewTitle = target.Title
	report.NewSource = target.SourceID
	report.NewSourceName = sources.GetSourceName(target.SourceID)

	// 3. Find or Create target manga in database
	var targetMangaID int64
	errExist := database.DB.QueryRow(ctx, `
		SELECT id FROM suwayomi.manga WHERE source = $1 AND url = $2
	`, target.SourceID, target.URL).Scan(&targetMangaID)

	nowSec := time.Now().Unix()

	if errExist != nil || targetMangaID <= 0 {
		// Insert new manga record
		errInsert := database.DB.QueryRow(ctx, `
			INSERT INTO suwayomi.manga (source, url, title, artist, author, description, genre, status, thumbnail_url, initialized, memo)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, '{}')
			RETURNING id
		`, target.SourceID, target.URL, target.Title, target.Artist, target.Author, target.Description, target.Genre, target.Status, target.ThumbnailURL).Scan(&targetMangaID)
		if errInsert != nil {
			report.Status = "ERROR"
			report.Message = fmt.Sprintf("Failed to insert target manga into DB: %v", errInsert)
			return report, errInsert
		}
	}

	report.NewMangaID = targetMangaID

	// 4. Fetch and save chapters for target manga
	log.Printf("[MIGRATE] Fetching and saving chapters for target manga %d (source %d, url %s)...", targetMangaID, target.SourceID, target.URL)
	chapters, errCh := sources.FetchAndSaveMangaChapters(ctx, target.SourceID, targetMangaID, target.URL)
	if errCh != nil {
		log.Printf("[MIGRATE] Warning: FetchAndSaveMangaChapters returned: %v", errCh)
	}
	report.NewTotalChapters = len(chapters)

	// If no chapters returned from live fetch, check if DB already has chapters
	if report.NewTotalChapters == 0 {
		_ = database.DB.QueryRow(ctx, `SELECT count(*) FROM suwayomi.chapter WHERE manga = $1`, targetMangaID).Scan(&report.NewTotalChapters)
	}

	// 5. Transfer Category associations
	_, _ = database.DB.Exec(ctx, `
		INSERT INTO suwayomi.categorymanga (category, manga, user_id)
		SELECT category, $1, user_id
		FROM suwayomi.categorymanga
		WHERE manga = $2
		ON CONFLICT (user_id, category, manga) DO NOTHING
	`, targetMangaID, oldMangaID)

	_, _ = database.DB.Exec(ctx, `DELETE FROM suwayomi.categorymanga WHERE manga = $1`, oldMangaID)

	// 6. Transfer Reading Progress (Match chapters by number)
	resProgress, errProgress := database.DB.Exec(ctx, `
		INSERT INTO suwayomi.chapteruser (chapter, user_id, read, bookmark, last_page_read, last_read_at, is_downloaded, is_download_requested, version, is_syncing, last_modified_at)
		SELECT c_new.id, cu_old.user_id, cu_old.read, cu_old.bookmark, cu_old.last_page_read, cu_old.last_read_at, false, false, 1, false, $3
		FROM suwayomi.chapter c_old
		JOIN suwayomi.chapteruser cu_old ON c_old.id = cu_old.chapter
		JOIN suwayomi.chapter c_new ON c_new.manga = $1 AND abs(c_new.chapter_number - c_old.chapter_number) < 0.05
		WHERE c_old.manga = $2 AND cu_old.read = true
		ON CONFLICT (chapter, user_id) DO UPDATE
		SET read = true,
		    bookmark = EXCLUDED.bookmark,
		    last_page_read = EXCLUDED.last_page_read,
		    last_read_at = EXCLUDED.last_read_at,
		    last_modified_at = EXCLUDED.last_modified_at
	`, targetMangaID, oldMangaID, nowSec)

	if errProgress == nil {
		report.NewReadChapters = int(resProgress.RowsAffected())
	} else {
		log.Printf("[MIGRATE] Error transferring progress: %v", errProgress)
	}

	// 7. Update Library Membership
	_, _ = database.DB.Exec(ctx, `
		INSERT INTO suwayomi.mangauser (manga, user_id, in_library, in_library_at, viewer, chapter_flags, version, is_syncing, last_modified_at)
		VALUES ($1, 1, true, $2, 0, 0, 1, false, $2)
		ON CONFLICT (manga, user_id) DO UPDATE SET in_library = true, last_modified_at = $2
	`, targetMangaID, nowSec)

	_, _ = database.DB.Exec(ctx, `
		UPDATE suwayomi.mangauser SET in_library = false, last_modified_at = $1 WHERE manga = $2
	`, nowSec, oldMangaID)

	// 8. Clone Cover Thumbnail if present on disk
	if cfg != nil && cfg.Data.RootDir != "" {
		thumbsDir := filepath.Join(cfg.Data.RootDir, "downloads", "thumbnails")
		oldThumbFile := filepath.Join(thumbsDir, fmt.Sprintf("%d.jpg", oldMangaID))
		newThumbFile := filepath.Join(thumbsDir, fmt.Sprintf("%d.jpg", targetMangaID))
		if _, errOld := os.Stat(oldThumbFile); errOld == nil {
			if _, errNew := os.Stat(newThumbFile); os.IsNotExist(errNew) {
				in, errOpen := os.Open(oldThumbFile)
				if errOpen == nil {
					out, errCreate := os.Create(newThumbFile)
					if errCreate == nil {
						_, _ = io.Copy(out, in)
						out.Close()
					}
					in.Close()
				}
			}
		}
	}

	report.Status = "SUCCESS"
	report.Message = fmt.Sprintf("Successfully migrated %q -> %q (%d chapters, %d read transferred)", oldTitle, target.Title, report.NewTotalChapters, report.NewReadChapters)
	return report, nil
}
