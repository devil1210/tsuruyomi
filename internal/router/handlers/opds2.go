package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
)

// Constants for OPDS 2.0
const (
	Opds2MimeType = "application/opds+json; charset=utf-8"
)

// OPDS 2.0 Data Structures (Readium Web Publication Manifest compatible)

type Opds2Link struct {
	Rel        string                 `json:"rel,omitempty"`
	Href       string                 `json:"href"`
	Type       string                 `json:"type,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Templated  bool                   `json:"templated,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type Opds2Metadata struct {
	Title         string      `json:"title"`
	NumberOfItems *int        `json:"numberOfItems,omitempty"`
	ItemsPerPage  *int        `json:"itemsPerPage,omitempty"`
	CurrentPage   *int        `json:"currentPage,omitempty"`
	Modified      string      `json:"modified,omitempty"`
	Description   string      `json:"description,omitempty"`
	Author        interface{} `json:"author,omitempty"`
}

type Opds2Author struct {
	Name string `json:"name"`
	URI  string `json:"uri,omitempty"`
}

type Opds2Navigation struct {
	Title      string                 `json:"title"`
	Href       string                 `json:"href"`
	Type       string                 `json:"type,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type Opds2Publication struct {
	Metadata Opds2Metadata `json:"metadata"`
	Links    []Opds2Link   `json:"links"`
	Images   []Opds2Link   `json:"images,omitempty"`
}

type Opds2Feed struct {
	Metadata     Opds2Metadata      `json:"metadata"`
	Links        []Opds2Link        `json:"links"`
	Navigation   []Opds2Navigation  `json:"navigation,omitempty"`
	Publications []Opds2Publication `json:"publications,omitempty"`
}

func renderOpds2JSON(c *fiber.Ctx, feed *Opds2Feed) error {
	data, err := json.Marshal(feed)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	c.Response().Header.Set("Content-Type", Opds2MimeType)
	return c.Send(data)
}

// GetOpds2Root serves the root navigation feed for OPDS 2.0.
// GET /api/opds/v2.0 or /api/opds/2.0
func GetOpds2Root(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())
	itemsCount := 7

	feed := &Opds2Feed{
		Metadata: Opds2Metadata{
			Title:         "Tsuruyomi Catalog (OPDS 2.0)",
			NumberOfItems: &itemsCount,
			Modified:      now,
			Description:   "Tsuruyomi OPDS 2.0 JSON Catalog",
			Author: &Opds2Author{
				Name: "Tsuruyomi",
				URI:  "https://github.com/devil1210/tsuruyomi",
			},
		},
		Links: []Opds2Link{
			{Rel: "self", Href: "/api/opds/v2.0", Type: Opds2MimeType},
			{Rel: "start", Href: "/api/opds/v2.0", Type: Opds2MimeType},
			{Rel: "search", Href: "/api/opds/v2.0/search?query={searchTerms}", Type: Opds2MimeType, Templated: true},
		},
		Navigation: []Opds2Navigation{
			{Title: "All Series", Href: "/api/opds/v2.0/library/series", Type: Opds2MimeType},
			{Title: "Categories", Href: "/api/opds/v2.0/library/categories", Type: Opds2MimeType},
			{Title: "Sources", Href: "/api/opds/v2.0/library/sources", Type: Opds2MimeType},
			{Title: "Genres", Href: "/api/opds/v2.0/library/genres", Type: Opds2MimeType},
			{Title: "Statuses", Href: "/api/opds/v2.0/library/statuses", Type: Opds2MimeType},
			{Title: "Library Updates", Href: "/api/opds/v2.0/library-updates", Type: Opds2MimeType},
			{Title: "Reading History", Href: "/api/opds/v2.0/history", Type: Opds2MimeType},
		},
	}

	return renderOpds2JSON(c, feed)
}

// GetOpds2Search handles searches in OPDS 2.0.
// GET /api/opds/v2.0/search
func GetOpds2Search(c *fiber.Ctx) error {
	query := c.Query("query")
	if query == "" {
		query = c.Query("q")
	}

	if query != "" {
		return buildMangaOpds2Feed(c, "search", fmt.Sprintf("Search: %s", query), "search", query)
	}

	// If no query, redirect/return root search descriptor
	return GetOpds2Root(c)
}

// GetOpds2LibrarySeries lists all manga series in library.
// GET /api/opds/v2.0/library/series
func GetOpds2LibrarySeries(c *fiber.Ctx) error {
	return buildMangaOpds2Feed(c, "library/series", "All Series", "", nil)
}

// GetOpds2CategorySeries lists manga series in a category.
// GET /api/opds/v2.0/category/:categoryId
func GetOpds2CategorySeries(c *fiber.Ctx) error {
	catIdStr := c.Params("categoryId")
	catId, err := strconv.Atoi(catIdStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid category ID")
	}

	catName := fmt.Sprintf("Category %d", catId)
	if database.DB != nil {
		_ = database.DB.QueryRow(c.Context(), "SELECT name FROM suwayomi.category WHERE id = $1", catId).Scan(&catName)
	}

	return buildMangaOpds2Feed(c, fmt.Sprintf("category/%d", catId), catName, "category", catId)
}

// GetOpds2SourceSeries lists manga series from a source.
// GET /api/opds/v2.0/source/:sourceId
func GetOpds2SourceSeries(c *fiber.Ctx) error {
	srcIdStr := c.Params("sourceId")
	srcId, err := strconv.ParseInt(srcIdStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid source ID")
	}

	srcName := fmt.Sprintf("Source %d", srcId)
	if database.DB != nil {
		_ = database.DB.QueryRow(c.Context(), "SELECT name FROM suwayomi.source WHERE id = $1", srcId).Scan(&srcName)
	}

	return buildMangaOpds2Feed(c, fmt.Sprintf("source/%d", srcId), srcName, "source", srcId)
}

// GetOpds2GenreSeries lists manga series with a given genre.
// GET /api/opds/v2.0/genre/:genre
func GetOpds2GenreSeries(c *fiber.Ctx) error {
	genre := c.Params("genre")
	decodedGenre, err := url.PathUnescape(genre)
	if err == nil {
		genre = decodedGenre
	}

	return buildMangaOpds2Feed(c, fmt.Sprintf("genre/%s", url.PathEscape(genre)), genre, "genre", genre)
}

// GetOpds2StatusSeries lists manga series with a given publication status.
// GET /api/opds/v2.0/status/:statusId
func GetOpds2StatusSeries(c *fiber.Ctx) error {
	statusStr := c.Params("statusId")
	statusInt, err := strconv.Atoi(statusStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid status ID")
	}

	statusName := string(models.MangaStatusFromInt(statusInt))
	return buildMangaOpds2Feed(c, fmt.Sprintf("status/%d", statusInt), statusName, "status", statusInt)
}

// buildMangaOpds2Feed constructs an OPDS 2.0 Navigation Feed for mangas.
func buildMangaOpds2Feed(c *fiber.Ctx, feedSubpath, feedTitle, filterType string, filterVal interface{}) error {
	now := formatRFC3339(time.Now())

	page := 1
	if pStr := c.Query("pageNumber"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	} else if pStr := c.Query("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := OpdsItemsPerPageDefault
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	offset := (page - 1) * limit

	searchQuery := c.Query("query")
	if searchQuery == "" {
		searchQuery = c.Query("q")
	}
	if filterType == "search" && filterVal != nil {
		searchQuery = filterVal.(string)
	}

	if database.DB == nil {
		feed := &Opds2Feed{
			Metadata: Opds2Metadata{
				Title:    feedTitle,
				Modified: now,
			},
		}
		return renderOpds2JSON(c, feed)
	}

	whereClauses := []string{"COALESCE(mu.in_library, false) = true"}
	var args []interface{}

	if searchQuery != "" {
		args = append(args, "%"+searchQuery+"%")
		whereClauses = append(whereClauses, fmt.Sprintf("(m.title ILIKE $%d OR m.author ILIKE $%d)", len(args), len(args)))
	}

	switch filterType {
	case "category":
		catId := filterVal.(int)
		if catId == 0 {
			whereClauses = append(whereClauses, "NOT EXISTS (SELECT 1 FROM suwayomi.categorymanga cm WHERE cm.manga = m.id)")
		} else {
			args = append(args, catId)
			whereClauses = append(whereClauses, fmt.Sprintf("EXISTS (SELECT 1 FROM suwayomi.categorymanga cm WHERE cm.manga = m.id AND cm.category = $%d)", len(args)))
		}
	case "source":
		srcId := filterVal.(int64)
		args = append(args, srcId)
		whereClauses = append(whereClauses, fmt.Sprintf("m.source = $%d", len(args)))
	case "genre":
		genre := filterVal.(string)
		args = append(args, "%"+genre+"%")
		whereClauses = append(whereClauses, fmt.Sprintf("m.genre ILIKE $%d", len(args)))
	case "status":
		statusInt := filterVal.(int)
		args = append(args, statusInt)
		whereClauses = append(whereClauses, fmt.Sprintf("m.status = $%d", len(args)))
	}

	whereSQL := strings.Join(whereClauses, " AND ")
	orderSQL := "m.title ASC"

	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT m.id)
		FROM suwayomi.manga m
		INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga
		WHERE %s
	`, whereSQL)

	var totalResults int
	_ = database.DB.QueryRow(c.Context(), countQuery, args...).Scan(&totalResults)

	mainQuery := fmt.Sprintf(`
		SELECT 
			m.id, m.title, m.author, m.description, m.genre, m.status, m.thumbnail_url,
			m.last_fetched_at, s.name as source_name, s.lang as source_lang,
			stat.chapters_count, stat.unread_count, stat.download_count
		FROM suwayomi.manga m
		INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga
		LEFT JOIN suwayomi.source s ON m.source = s.id
		LEFT JOIN LATERAL (
			SELECT 
				COUNT(*) as chapters_count,
				COUNT(*) FILTER (WHERE COALESCE(cu.read, false) = false) as unread_count,
				COUNT(*) FILTER (WHERE COALESCE(cu.is_downloaded, c.is_downloaded, false) = true) as download_count
			FROM suwayomi.chapter c
			LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
			WHERE c.manga = m.id
		) stat ON true
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderSQL, len(args)+1, len(args)+2)

	queryArgs := append(args, limit, offset)
	rows, err := database.DB.Query(c.Context(), mainQuery, queryArgs...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	defer rows.Close()

	var navItems []Opds2Navigation
	for rows.Next() {
		var (
			mID                                    int
			mTitle                                 string
			mAuthor, mDesc, mGenre, mThumb         sql.NullString
			mStatus                                int
			mLastFetched                           sql.NullInt64
			sName, sLang                           sql.NullString
			chCount, unreadCount, dlCount          int
		)

		err := rows.Scan(
			&mID, &mTitle, &mAuthor, &mDesc, &mGenre, &mStatus, &mThumb,
			&mLastFetched, &sName, &sLang,
			&chCount, &unreadCount, &dlCount,
		)
		if err != nil {
			continue
		}

		navItems = append(navItems, Opds2Navigation{
			Title: mTitle,
			Href:  fmt.Sprintf("/api/opds/v2.0/series/%d", mID),
			Type:  Opds2MimeType,
			Properties: map[string]interface{}{
				"numberOfItems": chCount,
			},
		})
	}

	feed := &Opds2Feed{
		Metadata: Opds2Metadata{
			Title:         feedTitle,
			NumberOfItems: &totalResults,
			ItemsPerPage:  &limit,
			CurrentPage:   &page,
			Modified:      now,
		},
		Navigation: navItems,
	}

	// Pagination links
	baseFeedURL := fmt.Sprintf("/api/opds/v2.0/%s", feedSubpath)
	feed.Links = append(feed.Links,
		Opds2Link{Rel: "self", Href: fmt.Sprintf("%s?pageNumber=%d&limit=%d", baseFeedURL, page, limit), Type: Opds2MimeType},
		Opds2Link{Rel: "start", Href: "/api/opds/v2.0", Type: Opds2MimeType},
		Opds2Link{Rel: "search", Href: "/api/opds/v2.0/search?query={searchTerms}", Type: Opds2MimeType, Templated: true},
		Opds2Link{Rel: "first", Href: fmt.Sprintf("%s?pageNumber=1&limit=%d", baseFeedURL, limit), Type: Opds2MimeType},
	)

	if page > 1 {
		feed.Links = append(feed.Links, Opds2Link{
			Rel:  "previous",
			Href: fmt.Sprintf("%s?pageNumber=%d&limit=%d", baseFeedURL, page-1, limit),
			Type: Opds2MimeType,
		})
	}
	if offset+limit < totalResults {
		feed.Links = append(feed.Links, Opds2Link{
			Rel:  "next",
			Href: fmt.Sprintf("%s?pageNumber=%d&limit=%d", baseFeedURL, page+1, limit),
			Type: Opds2MimeType,
		})
	}
	totalPages := (totalResults + limit - 1) / limit
	if totalPages > 0 {
		feed.Links = append(feed.Links, Opds2Link{
			Rel:  "last",
			Href: fmt.Sprintf("%s?pageNumber=%d&limit=%d", baseFeedURL, totalPages, limit),
			Type: Opds2MimeType,
		})
	}

	return renderOpds2JSON(c, feed)
}

// GetOpds2SeriesChapters lists all chapters for a manga as an OPDS 2.0 Publication Feed.
// GET /api/opds/v2.0/series/:mangaId or /api/opds/v2.0/series/:mangaId/chapters
func GetOpds2SeriesChapters(c *fiber.Ctx) error {
	mangaIdStr := c.Params("mangaId")
	mangaId, err := strconv.Atoi(mangaIdStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga ID")
	}

	ctx := c.Context()
	manga, err := database.GetMangaByID(ctx, mangaId)
	if err != nil || manga == nil {
		return c.Status(fiber.StatusNotFound).SendString("Manga not found")
	}

	chapters, err := database.GetChaptersByMangaID(ctx, mangaId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	now := formatRFC3339(time.Now())
	total := len(chapters)

	desc := ""
	if manga.Description != nil {
		desc = *manga.Description
	}

	feed := &Opds2Feed{
		Metadata: Opds2Metadata{
			Title:         manga.Title,
			NumberOfItems: &total,
			Modified:      now,
			Description:   desc,
		},
		Links: []Opds2Link{
			{Rel: "self", Href: fmt.Sprintf("/api/opds/v2.0/series/%d", mangaId), Type: Opds2MimeType},
			{Rel: "start", Href: "/api/opds/v2.0", Type: Opds2MimeType},
		},
	}

	if total == 0 {
		// Provide informative navigation item so KOReader and other readers open the view cleanly
		feed.Navigation = []Opds2Navigation{
			{
				Title: "[Sin capitulos disponibles en biblioteca]",
				Href:  fmt.Sprintf("/api/opds/v2.0/series/%d", mangaId),
				Type:  Opds2MimeType,
			},
		}
		return renderOpds2JSON(c, feed)
	}

	for _, ch := range chapters {
		chUpdated := now
		if ch.FetchedAt > 0 {
			chUpdated = formatRFC3339(time.Unix(ch.FetchedAt, 0))
		}

		statusLabel := "[Unread]"
		if ch.Read {
			statusLabel = "[Read]"
		} else if ch.Downloaded {
			statusLabel = "[Downloaded]"
		} else if ch.LastPageRead > 0 {
			statusLabel = "[In Progress]"
		}

		entryTitle := fmt.Sprintf("%s %s", statusLabel, ch.Name)
		scan := ""
		if ch.Scanlator != nil && *ch.Scanlator != "" {
			scan = "Scanlator: " + *ch.Scanlator + " | "
		}
		desc := fmt.Sprintf("%sPage: %d/%d", scan, ch.LastPageRead, ch.PageCount)

		pub := Opds2Publication{
			Metadata: Opds2Metadata{
				Title:       entryTitle,
				Modified:    chUpdated,
				Description: desc,
				Author: &Opds2Author{
					Name: manga.Title,
				},
			},
			Images: []Opds2Link{
				{
					Href: fmt.Sprintf("/api/v1/manga/%d/chapter/%d/page/0", mangaId, ch.Index),
					Type: OpdsMimeJpeg,
				},
			},
			Links: []Opds2Link{
				{
					Rel:   "http://opds-spec.org/acquisition/open-access",
					Href:  fmt.Sprintf("/api/v1/chapter/%d/download?markAsRead=false", ch.ID),
					Type:  OpdsMimeCbz,
					Title: "Download CBZ",
				},
			},
		}

		feed.Publications = append(feed.Publications, pub)
	}

	return renderOpds2JSON(c, feed)
}

// GetOpds2Categories lists library categories in OPDS 2.0.
// GET /api/opds/v2.0/library/categories
func GetOpds2Categories(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())

	cats, err := database.GetCategoryList(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	total := len(cats)
	var navItems []Opds2Navigation
	for _, cat := range cats {
		cnt := cat.Size
		navItems = append(navItems, Opds2Navigation{
			Title: cat.Name,
			Href:  fmt.Sprintf("/api/opds/v2.0/category/%d", cat.ID),
			Type:  Opds2MimeType,
			Properties: map[string]interface{}{
				"numberOfItems": cnt,
			},
		})
	}

	feed := &Opds2Feed{
		Metadata: Opds2Metadata{
			Title:         "Library Categories",
			NumberOfItems: &total,
			Modified:      now,
		},
		Links: []Opds2Link{
			{Rel: "self", Href: "/api/opds/v2.0/library/categories", Type: Opds2MimeType},
			{Rel: "start", Href: "/api/opds/v2.0", Type: Opds2MimeType},
		},
		Navigation: navItems,
	}

	return renderOpds2JSON(c, feed)
}

// GetOpds2Sources lists library sources in OPDS 2.0.
// GET /api/opds/v2.0/library/sources
func GetOpds2Sources(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())

	query := `
		SELECT s.id, s.name, s.lang, COUNT(m.id) as manga_count
		FROM suwayomi.source s
		INNER JOIN suwayomi.manga m ON s.id = m.source
		INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga AND COALESCE(mu.in_library, false) = true
		GROUP BY s.id, s.name, s.lang
		ORDER BY s.name ASC
	`

	rows, err := database.DB.Query(c.Context(), query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	defer rows.Close()

	var navItems []Opds2Navigation
	for rows.Next() {
		var srcID int64
		var srcName, srcLang string
		var count int
		if err := rows.Scan(&srcID, &srcName, &srcLang, &count); err == nil {
			displayName := fmt.Sprintf("%s (%s)", srcName, srcLang)
			navItems = append(navItems, Opds2Navigation{
				Title: displayName,
				Href:  fmt.Sprintf("/api/opds/v2.0/source/%d", srcID),
				Type:  Opds2MimeType,
				Properties: map[string]interface{}{
					"numberOfItems": count,
				},
			})
		}
	}

	total := len(navItems)
	feed := &Opds2Feed{
		Metadata: Opds2Metadata{
			Title:         "Library Sources",
			NumberOfItems: &total,
			Modified:      now,
		},
		Links: []Opds2Link{
			{Rel: "self", Href: "/api/opds/v2.0/library/sources", Type: Opds2MimeType},
			{Rel: "start", Href: "/api/opds/v2.0", Type: Opds2MimeType},
		},
		Navigation: navItems,
	}

	return renderOpds2JSON(c, feed)
}

// GetOpds2Genres lists library genres in OPDS 2.0.
// GET /api/opds/v2.0/library/genres
func GetOpds2Genres(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())

	query := `
		SELECT TRIM(g) as genre_name, COUNT(DISTINCT m.id) as manga_count
		FROM suwayomi.manga m
		INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga AND COALESCE(mu.in_library, false) = true,
		UNNEST(string_to_array(m.genre, ',')) g
		WHERE TRIM(g) <> ''
		GROUP BY TRIM(g)
		ORDER BY TRIM(g) ASC
	`

	rows, err := database.DB.Query(c.Context(), query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	defer rows.Close()

	var navItems []Opds2Navigation
	for rows.Next() {
		var genreName string
		var count int
		if err := rows.Scan(&genreName, &count); err == nil {
			navItems = append(navItems, Opds2Navigation{
				Title: genreName,
				Href:  fmt.Sprintf("/api/opds/v2.0/genre/%s", url.PathEscape(genreName)),
				Type:  Opds2MimeType,
				Properties: map[string]interface{}{
					"numberOfItems": count,
				},
			})
		}
	}

	total := len(navItems)
	feed := &Opds2Feed{
		Metadata: Opds2Metadata{
			Title:         "Library Genres",
			NumberOfItems: &total,
			Modified:      now,
		},
		Links: []Opds2Link{
			{Rel: "self", Href: "/api/opds/v2.0/library/genres", Type: Opds2MimeType},
			{Rel: "start", Href: "/api/opds/v2.0", Type: Opds2MimeType},
		},
		Navigation: navItems,
	}

	return renderOpds2JSON(c, feed)
}

// GetOpds2Statuses lists publication statuses in OPDS 2.0.
// GET /api/opds/v2.0/library/statuses
func GetOpds2Statuses(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())

	query := `
		SELECT m.status, COUNT(m.id) as manga_count
		FROM suwayomi.manga m
		INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga AND COALESCE(mu.in_library, false) = true
		GROUP BY m.status
		ORDER BY m.status ASC
	`

	rows, err := database.DB.Query(c.Context(), query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	defer rows.Close()

	var navItems []Opds2Navigation
	for rows.Next() {
		var statusInt, count int
		if err := rows.Scan(&statusInt, &count); err == nil {
			statusName := string(models.MangaStatusFromInt(statusInt))
			navItems = append(navItems, Opds2Navigation{
				Title: statusName,
				Href:  fmt.Sprintf("/api/opds/v2.0/status/%d", statusInt),
				Type:  Opds2MimeType,
				Properties: map[string]interface{}{
					"numberOfItems": count,
				},
			})
		}
	}

	total := len(navItems)
	feed := &Opds2Feed{
		Metadata: Opds2Metadata{
			Title:         "Library Publication Statuses",
			NumberOfItems: &total,
			Modified:      now,
		},
		Links: []Opds2Link{
			{Rel: "self", Href: "/api/opds/v2.0/library/statuses", Type: Opds2MimeType},
			{Rel: "start", Href: "/api/opds/v2.0", Type: Opds2MimeType},
		},
		Navigation: navItems,
	}

	return renderOpds2JSON(c, feed)
}

// GetOpds2LibraryUpdates lists recent chapter updates in OPDS 2.0.
// GET /api/opds/v2.0/library-updates
func GetOpds2LibraryUpdates(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())

	page := 0
	if pStr := c.Query("pageNumber"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p - 1
		}
	}

	result, err := database.GetRecentChapters(c.Context(), page)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	var pubs []Opds2Publication
	for _, mc := range result.Page {
		ch := mc.Chapter
		manga := mc.Manga

		chUpdated := now
		if ch.FetchedAt > 0 {
			chUpdated = formatRFC3339(time.Unix(ch.FetchedAt, 0))
		}

		pubs = append(pubs, Opds2Publication{
			Metadata: Opds2Metadata{
				Title:       fmt.Sprintf("%s - %s", manga.Title, ch.Name),
				Modified:    chUpdated,
				Description: fmt.Sprintf("%s | Chapter %s", manga.Title, ch.Name),
				Author: &Opds2Author{
					Name: manga.Title,
				},
			},
			Images: []Opds2Link{
				{
					Href: fmt.Sprintf("/api/v1/manga/%d/thumbnail", manga.ID),
					Type: OpdsMimeJpeg,
				},
			},
			Links: []Opds2Link{
				{
					Rel:   "http://opds-spec.org/acquisition/open-access",
					Href:  fmt.Sprintf("/api/v1/chapter/%d/download?markAsRead=false", ch.ID),
					Type:  OpdsMimeCbz,
					Title: "Download CBZ",
				},
			},
		})
	}

	feed := &Opds2Feed{
		Metadata: Opds2Metadata{
			Title:    "Library Updates",
			Modified: now,
		},
		Links: []Opds2Link{
			{Rel: "self", Href: fmt.Sprintf("/api/opds/v2.0/library-updates?pageNumber=%d", page+1), Type: Opds2MimeType},
			{Rel: "start", Href: "/api/opds/v2.0", Type: Opds2MimeType},
		},
		Publications: pubs,
	}

	return renderOpds2JSON(c, feed)
}

// GetOpds2History lists reading history in OPDS 2.0.
// GET /api/opds/v2.0/history
func GetOpds2History(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())

	page := 1
	if pStr := c.Query("pageNumber"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	limit := OpdsItemsPerPageDefault
	offset := (page - 1) * limit

	query := `
		SELECT 
			c.id, c.name, c.chapter_number, c.source_order, c.scanlator, c.page_count,
			COALESCE(cu.read, false), COALESCE(cu.last_page_read, 0), COALESCE(cu.last_read_at, 0),
			m.id, m.title, m.author, m.thumbnail_url
		FROM suwayomi.chapter c
		INNER JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
		INNER JOIN suwayomi.manga m ON c.manga = m.id
		WHERE cu.last_read_at > 0
		ORDER BY cu.last_read_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := database.DB.Query(c.Context(), query, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	defer rows.Close()

	var pubs []Opds2Publication
	for rows.Next() {
		var (
			chID, sourceOrder, pageCount, lastPage int
			chNumber                               float64
			chName, mTitle                         string
			scanlator, mAuthor, mThumb             sql.NullString
			isRead                                 bool
			lastReadAt                             int64
			mID                                    int
		)

		err := rows.Scan(
			&chID, &chName, &chNumber, &sourceOrder, &scanlator, &pageCount,
			&isRead, &lastPage, &lastReadAt,
			&mID, &mTitle, &mAuthor, &mThumb,
		)
		if err != nil {
			continue
		}

		readDate := formatRFC3339(time.Unix(lastReadAt, 0))
		pubs = append(pubs, Opds2Publication{
			Metadata: Opds2Metadata{
				Title:       fmt.Sprintf("%s: %s", mTitle, chName),
				Modified:    readDate,
				Description: fmt.Sprintf("Last read at %s (page %d/%d)", readDate, lastPage, pageCount),
				Author: &Opds2Author{
					Name: mTitle,
				},
			},
			Images: []Opds2Link{
				{
					Href: fmt.Sprintf("/api/v1/manga/%d/thumbnail", mID),
					Type: OpdsMimeJpeg,
				},
			},
			Links: []Opds2Link{
				{
					Rel:   "http://opds-spec.org/acquisition/open-access",
					Href:  fmt.Sprintf("/api/v1/chapter/%d/download?markAsRead=false", chID),
					Type:  OpdsMimeCbz,
					Title: "Download CBZ",
				},
			},
		})
	}

	feed := &Opds2Feed{
		Metadata: Opds2Metadata{
			Title:    "Reading History",
			Modified: now,
		},
		Links: []Opds2Link{
			{Rel: "self", Href: fmt.Sprintf("/api/opds/v2.0/history?pageNumber=%d", page), Type: Opds2MimeType},
			{Rel: "start", Href: "/api/opds/v2.0", Type: Opds2MimeType},
		},
		Publications: pubs,
	}

	return renderOpds2JSON(c, feed)
}
