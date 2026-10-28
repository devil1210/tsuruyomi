package handlers

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
)

// Constants for OPDS 1.2
const (
	OpdsNsAtom       = "http://www.w3.org/2005/Atom"
	OpdsNsOpds       = "http://opds-spec.org/2010/catalog"
	OpdsNsDc         = "http://purl.org/dc/terms/"
	OpdsNsPse        = "http://vaemendis.net/opds-pse/ns"
	OpdsNsThr        = "http://purl.org/syndication/thread/1.0"
	OpdsNsOpenSearch = "http://a9.com/-/spec/opensearch/1.1/"

	OpdsRelAcquisition           = "http://opds-spec.org/acquisition"
	OpdsRelAcquisitionOpenAccess = "http://opds-spec.org/acquisition/open-access"
	OpdsRelImage                 = "http://opds-spec.org/image"
	OpdsRelImageThumbnail        = "http://opds-spec.org/image/thumbnail"
	OpdsRelSelf                  = "self"
	OpdsRelStart                 = "start"
	OpdsRelSubsection            = "subsection"
	OpdsRelAlternate             = "alternate"
	OpdsRelFacet                 = "http://opds-spec.org/facet"
	OpdsRelSearch                = "search"
	OpdsRelNext                  = "next"
	OpdsRelPrevious              = "previous"
	OpdsRelFirst                 = "first"
	OpdsRelLast                  = "last"
	OpdsRelPseStream             = "http://vaemendis.net/opds-pse/stream"

	OpdsMimeNavigation = "application/atom+xml;profile=opds-catalog;kind=navigation;charset=utf-8"
	OpdsMimeAcquisition = "application/atom+xml;profile=opds-catalog;kind=acquisition;charset=utf-8"
	OpdsMimeCbz        = "application/vnd.comicbook+zip"
	OpdsMimeJpeg       = "image/jpeg"
	OpdsMimeOpenSearch = "application/opensearchdescription+xml"

	OpdsItemsPerPageDefault = 50
)

// XML Data Structures for OPDS 1.2

type OpdsFeed struct {
	XMLName          xml.Name      `xml:"feed"`
	Xmlns            string        `xml:"xmlns,attr"`
	XmlnsDc          string        `xml:"xmlns:dc,attr,omitempty"`
	XmlnsOpds        string        `xml:"xmlns:opds,attr,omitempty"`
	XmlnsPse         string        `xml:"xmlns:pse,attr,omitempty"`
	XmlnsThr         string        `xml:"xmlns:thr,attr,omitempty"`
	XmlnsOpenSearch  string        `xml:"xmlns:opensearch,attr,omitempty"`
	ID               string        `xml:"id"`
	Title            string        `xml:"title"`
	Updated          string        `xml:"updated"`
	Author           *OpdsAuthor   `xml:"author,omitempty"`
	TotalResults     *int          `xml:"opensearch:totalResults,omitempty"`
	ItemsPerPage     *int          `xml:"opensearch:itemsPerPage,omitempty"`
	StartIndex       *int          `xml:"opensearch:startIndex,omitempty"`
	Links            []OpdsLink    `xml:"link"`
	Entries          []OpdsEntry   `xml:"entry"`
}

type OpdsAuthor struct {
	Name string `xml:"name"`
	URI  string `xml:"uri,omitempty"`
}

type OpdsLink struct {
	Rel             string `xml:"rel,attr"`
	Href            string `xml:"href,attr"`
	Type            string `xml:"type,attr,omitempty"`
	Title           string `xml:"title,attr,omitempty"`
	FacetGroup      string `xml:"opds:facetGroup,attr,omitempty"`
	ActiveFacet     string `xml:"opds:activeFacet,attr,omitempty"`
	ThrCount        *int   `xml:"thr:count,attr,omitempty"`
	PseCount        *int   `xml:"pse:count,attr,omitempty"`
	PseLastRead     *int   `xml:"pse:lastRead,attr,omitempty"`
	PseLastReadDate string `xml:"pse:lastReadDate,attr,omitempty"`
}

type OpdsEntry struct {
	ID         string         `xml:"id"`
	Title      string         `xml:"title"`
	Updated    string         `xml:"updated"`
	Summary    *OpdsTextElem  `xml:"summary,omitempty"`
	Content    *OpdsTextElem  `xml:"content,omitempty"`
	Authors    []OpdsAuthor   `xml:"author,omitempty"`
	Categories []OpdsCategory `xml:"category,omitempty"`
	Links      []OpdsLink     `xml:"link"`
	Publisher  string         `xml:"dc:publisher,omitempty"`
	Language   string         `xml:"dc:language,omitempty"`
	Issued     string         `xml:"dc:issued,omitempty"`
}

type OpdsTextElem struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",chardata"`
}

type OpdsCategory struct {
	Term   string `xml:"term,attr"`
	Label  string `xml:"label,attr,omitempty"`
	Scheme string `xml:"scheme,attr,omitempty"`
}

// OpenSearchDescription XML document
type OpenSearchDescription struct {
	XMLName        xml.Name `xml:"OpenSearchDescription"`
	Xmlns          string   `xml:"xmlns,attr"`
	ShortName      string   `xml:"ShortName"`
	Description    string   `xml:"Description"`
	InputEncoding  string   `xml:"InputEncoding"`
	OutputEncoding string   `xml:"OutputEncoding"`
	Url            SearchUrl `xml:"Url"`
}

type SearchUrl struct {
	Type     string `xml:"type,attr"`
	Template string `xml:"template,attr"`
}

func formatRFC3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

var xmlAttrRegex = regexp.MustCompile(`([a-zA-Z0-9_:-]+)="([^"]*)"`)

// sanitizeAttrToASCII converts accented Latin characters to their ASCII base
// and strips any remaining non-ASCII characters to keep XML attributes safe for
// strict lexers (such as KOReader's luxl in versions <= 2024.11).
func sanitizeAttrToASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'á', 'à', 'ä', 'â', 'ã', 'å':
			b.WriteByte('a')
		case 'Á', 'À', 'Ä', 'Â', 'Ã', 'Å':
			b.WriteByte('A')
		case 'é', 'è', 'ë', 'ê':
			b.WriteByte('e')
		case 'É', 'È', 'Ë', 'Ê':
			b.WriteByte('E')
		case 'í', 'ì', 'ï', 'î':
			b.WriteByte('i')
		case 'Í', 'Ì', 'Ï', 'Î':
			b.WriteByte('I')
		case 'ó', 'ò', 'ö', 'ô', 'õ', 'ø':
			b.WriteByte('o')
		case 'Ó', 'Ò', 'Ö', 'Ô', 'Õ', 'Ø':
			b.WriteByte('O')
		case 'ú', 'ù', 'ü', 'û':
			b.WriteByte('u')
		case 'Ú', 'Ù', 'Ü', 'Û':
			b.WriteByte('U')
		case 'ñ':
			b.WriteByte('n')
		case 'Ñ':
			b.WriteByte('N')
		case 'ç':
			b.WriteByte('c')
		case 'Ç':
			b.WriteByte('C')
		case 'ß':
			b.WriteString("ss")
		default:
			if r <= 127 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// slugifyTerm creates an ASCII-only identifier slug for OPDS category terms.
func slugifyTerm(s string) string {
	ascii := strings.ToLower(sanitizeAttrToASCII(s))
	var b strings.Builder
	for _, r := range ascii {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' || r == '-' {
			b.WriteByte('_')
		}
	}
	res := strings.Trim(b.String(), "_")
	if res == "" {
		return "general"
	}
	return res
}

func sanitizeXMLAttributes(xmlStr string) string {
	return xmlAttrRegex.ReplaceAllStringFunc(xmlStr, func(m string) string {
		sub := xmlAttrRegex.FindStringSubmatch(m)
		if len(sub) == 3 {
			attrName := sub[1]
			attrVal := sub[2]
			hasNonASCII := false
			for i := 0; i < len(attrVal); i++ {
				if attrVal[i] >= 128 {
					hasNonASCII = true
					break
				}
			}
			if hasNonASCII {
				return fmt.Sprintf(`%s="%s"`, attrName, sanitizeAttrToASCII(attrVal))
			}
		}
		return m
	})
}

func renderOpdsXML(c *fiber.Ctx, feed *OpdsFeed, isAcquisition bool) error {
	mime := OpdsMimeNavigation
	if isAcquisition {
		mime = OpdsMimeAcquisition
	}
	c.Set(fiber.HeaderContentType, mime)

	data, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Response().Header.Set("Content-Type", mime)
	cleanXML := sanitizeXMLAttributes(xml.Header + string(data))
	return c.SendString(cleanXML)
}

// GetOpdsRoot serves the root navigation feed.
// GET /api/opds/v1.2 or /opds
func GetOpdsRoot(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())

	feed := &OpdsFeed{
		Xmlns:           OpdsNsAtom,
		XmlnsDc:         OpdsNsDc,
		XmlnsOpds:       OpdsNsOpds,
		XmlnsOpenSearch: OpdsNsOpenSearch,
		ID:              "urn:tsuruyomi:opds:root",
		Title:           "Tsuruyomi Catalog",
		Updated:         now,
		Author: &OpdsAuthor{
			Name: "Tsuruyomi",
			URI:  "https://github.com/tsuruyomi/tsuruyomi",
		},
		Links: []OpdsLink{
			{Rel: OpdsRelSelf, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
			{Rel: OpdsRelStart, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
			{Rel: OpdsRelSearch, Href: "/api/opds/v1.2/search", Type: OpdsMimeOpenSearch, Title: "Search"},
		},
		Entries: []OpdsEntry{
			{
				ID:      "urn:tsuruyomi:navigation:series",
				Title:   "All Series",
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: "Browse all manga series in your library"},
				Links: []OpdsLink{
					{Rel: OpdsRelSubsection, Href: "/api/opds/v1.2/library/series", Type: OpdsMimeAcquisition, Title: "All Series"},
				},
			},
			{
				ID:      "urn:tsuruyomi:navigation:categories",
				Title:   "Categories",
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: "Browse library organized by categories"},
				Links: []OpdsLink{
					{Rel: OpdsRelSubsection, Href: "/api/opds/v1.2/library/categories", Type: OpdsMimeNavigation, Title: "Categories"},
				},
			},
			{
				ID:      "urn:tsuruyomi:navigation:sources",
				Title:   "Sources",
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: "Browse library series grouped by source"},
				Links: []OpdsLink{
					{Rel: OpdsRelSubsection, Href: "/api/opds/v1.2/library/sources", Type: OpdsMimeNavigation, Title: "Sources"},
				},
			},
			{
				ID:      "urn:tsuruyomi:navigation:genres",
				Title:   "Genres",
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: "Browse library series by genre/tags"},
				Links: []OpdsLink{
					{Rel: OpdsRelSubsection, Href: "/api/opds/v1.2/library/genres", Type: OpdsMimeNavigation, Title: "Genres"},
				},
			},
			{
				ID:      "urn:tsuruyomi:navigation:statuses",
				Title:   "Statuses",
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: "Browse library series by publication status"},
				Links: []OpdsLink{
					{Rel: OpdsRelSubsection, Href: "/api/opds/v1.2/library/statuses", Type: OpdsMimeNavigation, Title: "Statuses"},
				},
			},
			{
				ID:      "urn:tsuruyomi:navigation:updates",
				Title:   "Library Updates",
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: "Recent chapter updates for series in library"},
				Links: []OpdsLink{
					{Rel: OpdsRelSubsection, Href: "/api/opds/v1.2/library-updates", Type: OpdsMimeAcquisition, Title: "Library Updates"},
				},
			},
			{
				ID:      "urn:tsuruyomi:navigation:history",
				Title:   "Reading History",
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: "Recently read chapters"},
				Links: []OpdsLink{
					{Rel: OpdsRelSubsection, Href: "/api/opds/v1.2/history", Type: OpdsMimeAcquisition, Title: "Reading History"},
				},
			},
		},
	}

	return renderOpdsXML(c, feed, false)
}

// GetOpdsSearch serves the OpenSearch Description or query results.
// GET /api/opds/v1.2/search
func GetOpdsSearch(c *fiber.Ctx) error {
	query := c.Query("query")
	if query == "" {
		query = c.Query("q")
	}

	if query != "" {
		// Forward search query directly to library series handler
		return GetOpdsLibrarySeries(c)
	}

	// Serve OpenSearch XML description
	desc := OpenSearchDescription{
		Xmlns:          OpdsNsOpenSearch,
		ShortName:      "Tsuruyomi",
		Description:    "Search Manga in Tsuruyomi Library",
		InputEncoding:  "UTF-8",
		OutputEncoding: "UTF-8",
		Url: SearchUrl{
			Type:     OpdsMimeAcquisition,
			Template: "/api/opds/v1.2/library/series?query={searchTerms}",
		},
	}

	c.Set(fiber.HeaderContentType, OpdsMimeOpenSearch)
	data, err := xml.MarshalIndent(desc, "", "  ")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.SendString(xml.Header + string(data))
}

// GetOpdsLibrarySeries serves the acquisition feed of library mangas.
// GET /api/opds/v1.2/library/series
func GetOpdsLibrarySeries(c *fiber.Ctx) error {
	return buildMangaAcquisitionFeed(c, "library/series", "All Series", "", nil)
}

// GetOpdsCategorySeries serves mangas within a category.
// GET /api/opds/v1.2/category/:categoryId
func GetOpdsCategorySeries(c *fiber.Ctx) error {
	catIdStr := c.Params("categoryId")
	catId, err := strconv.Atoi(catIdStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid category ID")
	}

	catName := fmt.Sprintf("Category %d", catId)
	if database.DB != nil {
		_ = database.DB.QueryRow(c.Context(), "SELECT name FROM suwayomi.category WHERE id = $1", catId).Scan(&catName)
	}

	return buildMangaAcquisitionFeed(c, fmt.Sprintf("category/%d", catId), catName, "category", catId)
}

// GetOpdsSourceSeries serves mangas from a specific source.
// GET /api/opds/v1.2/source/:sourceId
func GetOpdsSourceSeries(c *fiber.Ctx) error {
	srcIdStr := c.Params("sourceId")
	srcId, err := strconv.ParseInt(srcIdStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid source ID")
	}

	srcName := fmt.Sprintf("Source %d", srcId)
	if database.DB != nil {
		_ = database.DB.QueryRow(c.Context(), "SELECT name FROM suwayomi.source WHERE id = $1", srcId).Scan(&srcName)
	}

	return buildMangaAcquisitionFeed(c, fmt.Sprintf("source/%d", srcId), srcName, "source", srcId)
}

// GetOpdsGenreSeries serves mangas with a specific genre.
// GET /api/opds/v1.2/genre/:genre
func GetOpdsGenreSeries(c *fiber.Ctx) error {
	genre := c.Params("genre")
	decodedGenre, err := url.PathUnescape(genre)
	if err == nil {
		genre = decodedGenre
	}

	return buildMangaAcquisitionFeed(c, fmt.Sprintf("genre/%s", url.PathEscape(genre)), genre, "genre", genre)
}

// GetOpdsStatusSeries serves mangas with a specific status.
// GET /api/opds/v1.2/status/:statusId
func GetOpdsStatusSeries(c *fiber.Ctx) error {
	statusStr := c.Params("statusId")
	statusInt, err := strconv.Atoi(statusStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid status ID")
	}

	statusName := string(models.MangaStatusFromInt(statusInt))
	return buildMangaAcquisitionFeed(c, fmt.Sprintf("status/%d", statusInt), statusName, "status", statusInt)
}

// buildMangaAcquisitionFeed constructs an OPDS acquisition feed with pagination and filters.
func buildMangaAcquisitionFeed(c *fiber.Ctx, feedSubpath, feedTitle, filterType string, filterVal interface{}) error {
	now := formatRFC3339(time.Now())

	// Pagination
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
	if sVal := database.GetServerSetting(c.Context(), "opdsItemsPerPage", ""); sVal != "" {
		if parsed, err := strconv.Atoi(sVal); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	offset := (page - 1) * limit

	// Search & Filters
	searchQuery := c.Query("query")
	if searchQuery == "" {
		searchQuery = c.Query("q")
	}
	sortParam := c.Query("sort")
	filterParam := c.Query("filter")

	if database.DB == nil {
		feed := &OpdsFeed{
			Xmlns:   OpdsNsAtom,
			ID:      "urn:tsuruyomi:feed:" + feedSubpath,
			Title:   feedTitle,
			Updated: now,
		}
		return renderOpdsXML(c, feed, true)
	}

	// Dynamic SQL Query Builder
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

	if filterParam == "unread" {
		whereClauses = append(whereClauses, "stat.unread_count > 0")
	} else if filterParam == "downloaded" {
		whereClauses = append(whereClauses, "stat.download_count > 0")
	} else if filterParam == "read" {
		whereClauses = append(whereClauses, "stat.unread_count = 0")
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Sorting
	orderSQL := "m.title ASC"
	switch sortParam {
	case "alpha_desc":
		orderSQL = "m.title DESC"
	case "unread_desc":
		orderSQL = "stat.unread_count DESC, m.title ASC"
	case "date_added_desc":
		orderSQL = "mu.in_library_at DESC"
	case "last_read_desc":
		orderSQL = "stat.max_last_read_at DESC"
	}

	// Count Total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT m.id)
		FROM suwayomi.manga m
		INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga
		LEFT JOIN LATERAL (
			SELECT 
				COUNT(*) FILTER (WHERE COALESCE(cu.read, false) = false) as unread_count,
				COUNT(*) FILTER (WHERE COALESCE(cu.is_downloaded, c.is_downloaded, false) = true) as download_count,
				MAX(cu.last_read_at) as max_last_read_at
			FROM suwayomi.chapter c
			LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
			WHERE c.manga = m.id
		) stat ON true
		WHERE %s
	`, whereSQL)

	var totalResults int
	_ = database.DB.QueryRow(c.Context(), countQuery, args...).Scan(&totalResults)

	// Fetch Page
	fetchQuery := fmt.Sprintf(`
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
				COUNT(*) FILTER (WHERE COALESCE(cu.is_downloaded, c.is_downloaded, false) = true) as download_count,
				MAX(cu.last_read_at) as max_last_read_at
			FROM suwayomi.chapter c
			LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
			WHERE c.manga = m.id
		) stat ON true
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderSQL, len(args)+1, len(args)+2)

	fetchArgs := append(args, limit, offset)
	rows, err := database.DB.Query(c.Context(), fetchQuery, fetchArgs...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	defer rows.Close()

	entries := []OpdsEntry{}
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

		updatedTime := now
		if mLastFetched.Valid && mLastFetched.Int64 > 0 {
			updatedTime = formatRFC3339(time.Unix(mLastFetched.Int64, 0))
		}

		statusStr := string(models.MangaStatusFromInt(mStatus))
		sourceName := "Unknown"
		if sName.Valid && sName.String != "" {
			sourceName = sName.String
		}
		sourceLang := ""
		if sLang.Valid && sLang.String != "" {
			sourceLang = sLang.String
		}

		summaryText := fmt.Sprintf("Status: %s | Source: %s (%s) | %d Chapters (%d unread, %d downloaded)",
			statusStr, sourceName, sourceLang, chCount, unreadCount, dlCount)

		var authors []OpdsAuthor
		if mAuthor.Valid && mAuthor.String != "" {
			authors = append(authors, OpdsAuthor{Name: mAuthor.String})
		}

		var categories []OpdsCategory
		if mGenre.Valid && mGenre.String != "" {
			for _, g := range strings.Split(mGenre.String, ",") {
				g = strings.TrimSpace(g)
				if g != "" {
					categories = append(categories, OpdsCategory{
						Term:   slugifyTerm(g),
						Label:  sanitizeAttrToASCII(g),
						Scheme: "/api/opds/v1.2/library/genres",
					})
				}
			}
		}

		var contentElem *OpdsTextElem
		if mDesc.Valid && mDesc.String != "" {
			contentElem = &OpdsTextElem{Type: "text", Value: mDesc.String}
		}

		links := []OpdsLink{
			{
				Rel:  OpdsRelSubsection,
				Href: fmt.Sprintf("/api/opds/v1.2/series/%d", mID),
				Type: OpdsMimeAcquisition,
			},
			{
				Rel:  OpdsRelImage,
				Href: fmt.Sprintf("/api/v1/manga/%d/thumbnail", mID),
				Type: OpdsMimeJpeg,
			},
			{
				Rel:  OpdsRelImageThumbnail,
				Href: fmt.Sprintf("/api/v1/manga/%d/thumbnail", mID),
				Type: OpdsMimeJpeg,
			},
			{
				Rel:   OpdsRelAlternate,
				Href:  fmt.Sprintf("/library/manga/%d", mID),
				Type:  "text/html",
				Title: "View in WebUI",
			},
		}

		entries = append(entries, OpdsEntry{
			ID:         fmt.Sprintf("urn:tsuruyomi:manga:%d", mID),
			Title:      mTitle,
			Updated:    updatedTime,
			Summary:    &OpdsTextElem{Type: "text", Value: summaryText},
			Content:    contentElem,
			Authors:    authors,
			Categories: categories,
			Links:      links,
			Publisher:  sourceName,
			Language:   sourceLang,
		})
	}

	startIndex := offset + 1
	feed := &OpdsFeed{
		Xmlns:           OpdsNsAtom,
		XmlnsDc:         OpdsNsDc,
		XmlnsOpds:       OpdsNsOpds,
		XmlnsOpenSearch: OpdsNsOpenSearch,
		XmlnsThr:        OpdsNsThr,
		ID:              fmt.Sprintf("urn:tsuruyomi:feed:%s", feedSubpath),
		Title:           feedTitle,
		Updated:         now,
		TotalResults:    &totalResults,
		ItemsPerPage:    &limit,
		StartIndex:      &startIndex,
		Entries:         entries,
	}

	// Feed Navigation Links (pagination)
	baseFeedURL := fmt.Sprintf("/api/opds/v1.2/%s", feedSubpath)
	feed.Links = append(feed.Links,
		OpdsLink{Rel: OpdsRelSelf, Href: fmt.Sprintf("%s?pageNumber=%d&limit=%d", baseFeedURL, page, limit), Type: OpdsMimeAcquisition},
		OpdsLink{Rel: OpdsRelStart, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
		OpdsLink{Rel: OpdsRelSearch, Href: "/api/opds/v1.2/search", Type: OpdsMimeOpenSearch, Title: "Search"},
		OpdsLink{Rel: OpdsRelFirst, Href: fmt.Sprintf("%s?pageNumber=1&limit=%d", baseFeedURL, limit), Type: OpdsMimeAcquisition},
	)

	if page > 1 {
		feed.Links = append(feed.Links, OpdsLink{
			Rel:  OpdsRelPrevious,
			Href: fmt.Sprintf("%s?pageNumber=%d&limit=%d", baseFeedURL, page-1, limit),
			Type: OpdsMimeAcquisition,
		})
	}
	if offset+limit < totalResults {
		feed.Links = append(feed.Links, OpdsLink{
			Rel:  OpdsRelNext,
			Href: fmt.Sprintf("%s?pageNumber=%d&limit=%d", baseFeedURL, page+1, limit),
			Type: OpdsMimeAcquisition,
		})
	}
	totalPages := (totalResults + limit - 1) / limit
	if totalPages > 0 {
		feed.Links = append(feed.Links, OpdsLink{
			Rel:  OpdsRelLast,
			Href: fmt.Sprintf("%s?pageNumber=%d&limit=%d", baseFeedURL, totalPages, limit),
			Type: OpdsMimeAcquisition,
		})
	}

	return renderOpdsXML(c, feed, true)
}

// GetOpdsSeriesChapters lists all chapters for a manga as an acquisition feed.
// GET /api/opds/v1.2/series/:mangaId or /api/opds/v1.2/series/:mangaId/chapters
func GetOpdsSeriesChapters(c *fiber.Ctx) error {
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

	markAsRead := "false"
	showOnlyUnread := false
	showOnlyDownloaded := false
	sortAsc := false
	if sMap, err := database.GetAllServerSettings(ctx); err == nil && sMap != nil {
		if sMap["opdsMarkAsReadOnDownload"] == "true" {
			markAsRead = "true"
		}
		if sMap["opdsShowOnlyUnreadChapters"] == "true" {
			showOnlyUnread = true
		}
		if sMap["opdsShowOnlyDownloadedChapters"] == "true" {
			showOnlyDownloaded = true
		}
		if strings.EqualFold(sMap["opdsChapterSortOrder"], "ASC") {
			sortAsc = true
		}
	}

	var filteredChapters []models.ChapterDataClass
	for _, ch := range chapters {
		if showOnlyUnread && ch.Read {
			continue
		}
		if showOnlyDownloaded && !ch.Downloaded {
			continue
		}
		filteredChapters = append(filteredChapters, ch)
	}
	if sortAsc {
		for i, j := 0, len(filteredChapters)-1; i < j; i, j = i+1, j-1 {
			filteredChapters[i], filteredChapters[j] = filteredChapters[j], filteredChapters[i]
		}
	}
	chapters = filteredChapters

	now := formatRFC3339(time.Now())
	feed := &OpdsFeed{
		Xmlns:           OpdsNsAtom,
		XmlnsDc:         OpdsNsDc,
		XmlnsOpds:       OpdsNsOpds,
		XmlnsPse:        OpdsNsPse,
		XmlnsOpenSearch: OpdsNsOpenSearch,
		ID:              fmt.Sprintf("urn:tsuruyomi:series:%d:chapters", mangaId),
		Title:           manga.Title,
		Updated:         now,
		Links: []OpdsLink{
			{Rel: OpdsRelSelf, Href: fmt.Sprintf("/api/opds/v1.2/series/%d", mangaId), Type: OpdsMimeAcquisition},
			{Rel: OpdsRelStart, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
			{Rel: OpdsRelImage, Href: fmt.Sprintf("/api/v1/manga/%d/thumbnail", mangaId), Type: OpdsMimeJpeg},
			{Rel: OpdsRelImageThumbnail, Href: fmt.Sprintf("/api/v1/manga/%d/thumbnail", mangaId), Type: OpdsMimeJpeg},
		},
	}

	total := len(chapters)
	feed.TotalResults = &total

	if len(chapters) == 0 {
		feed.Entries = append(feed.Entries, OpdsEntry{
			ID:      fmt.Sprintf("urn:tsuruyomi:series:%d:no-chapters", mangaId),
			Title:   "[Sin capitulos disponibles en biblioteca]",
			Updated: now,
			Summary: &OpdsTextElem{Type: "text", Value: fmt.Sprintf("No hay capitulos sincronizados para %s.", manga.Title)},
			Content: &OpdsTextElem{Type: "text", Value: "Esta serie aun no tiene capitulos sincronizados en la base de datos de Tsuruyomi. Actualiza la serie desde la WebUI para descargar la lista de capitulos."},
		})
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
		summary := fmt.Sprintf("%s - %s", manga.Title, ch.Name)

		scan := ""
		if ch.Scanlator != nil && *ch.Scanlator != "" {
			scan = "Scanlator: " + *ch.Scanlator + " | "
		}
		progressInfo := fmt.Sprintf("Page: %d/%d", ch.LastPageRead, ch.PageCount)
		content := scan + progressInfo

		pageCountInt := ch.PageCount
		lastPageInt := ch.LastPageRead
		var lastReadDateStr string
		if ch.LastReadAt > 0 {
			lastReadDateStr = formatRFC3339(time.Unix(ch.LastReadAt, 0))
		}

		links := []OpdsLink{
			// Direct CBZ Acquisition Link
			{
				Rel:   OpdsRelAcquisitionOpenAccess,
				Href:  fmt.Sprintf("/api/v1/chapter/%d/download?markAsRead=%s", ch.ID, markAsRead),
				Type:  OpdsMimeCbz,
				Title: "Download CBZ",
			},
			// OPDS Page Streaming (PSE) Link
			{
				Rel:             OpdsRelPseStream,
				Href:            fmt.Sprintf("/api/v1/manga/%d/chapter/%d/page/{pageNumber}", mangaId, ch.Index),
				Type:            OpdsMimeJpeg,
				Title:           "Stream Chapter Pages",
				PseCount:        &pageCountInt,
				PseLastRead:     &lastPageInt,
				PseLastReadDate: lastReadDateStr,
			},
			// Chapter First Page Image Link
			{
				Rel:  OpdsRelImage,
				Href: fmt.Sprintf("/api/v1/manga/%d/chapter/%d/page/0", mangaId, ch.Index),
				Type: OpdsMimeJpeg,
			},
			{
				Rel:  OpdsRelImageThumbnail,
				Href: fmt.Sprintf("/api/v1/manga/%d/chapter/%d/page/0", mangaId, ch.Index),
				Type: OpdsMimeJpeg,
			},
		}

		feed.Entries = append(feed.Entries, OpdsEntry{
			ID:        fmt.Sprintf("urn:tsuruyomi:chapter:%d", ch.ID),
			Title:     entryTitle,
			Updated:   chUpdated,
			Summary:   &OpdsTextElem{Type: "text", Value: summary},
			Content:   &OpdsTextElem{Type: "text", Value: content},
			Links:     links,
			Publisher: manga.Title,
		})
	}

	return renderOpdsXML(c, feed, true)
}

// GetOpdsCategories serves the categories navigation feed.
// GET /api/opds/v1.2/library/categories
func GetOpdsCategories(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())

	cats, err := database.GetCategoryList(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	feed := &OpdsFeed{
		Xmlns:           OpdsNsAtom,
		XmlnsOpds:       OpdsNsOpds,
		XmlnsThr:        OpdsNsThr,
		XmlnsOpenSearch: OpdsNsOpenSearch,
		ID:              "urn:tsuruyomi:library:categories",
		Title:           "Library Categories",
		Updated:         now,
		Links: []OpdsLink{
			{Rel: OpdsRelSelf, Href: "/api/opds/v1.2/library/categories", Type: OpdsMimeNavigation},
			{Rel: OpdsRelStart, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
		},
	}

	total := len(cats)
	feed.TotalResults = &total

	for _, cat := range cats {
		cnt := cat.Size
		feed.Entries = append(feed.Entries, OpdsEntry{
			ID:      fmt.Sprintf("urn:tsuruyomi:category:%d", cat.ID),
			Title:   cat.Name,
			Updated: now,
			Content: &OpdsTextElem{Type: "text", Value: fmt.Sprintf("%d series in this category", cat.Size)},
			Links: []OpdsLink{
				{
					Rel:      OpdsRelSubsection,
					Href:     fmt.Sprintf("/api/opds/v1.2/category/%d", cat.ID),
					Type:     OpdsMimeAcquisition,
					ThrCount: &cnt,
				},
			},
		})
	}

	return renderOpdsXML(c, feed, false)
}

// GetOpdsSources serves the sources navigation feed.
// GET /api/opds/v1.2/library/sources
func GetOpdsSources(c *fiber.Ctx) error {
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

	feed := &OpdsFeed{
		Xmlns:           OpdsNsAtom,
		XmlnsOpds:       OpdsNsOpds,
		XmlnsThr:        OpdsNsThr,
		XmlnsOpenSearch: OpdsNsOpenSearch,
		ID:              "urn:tsuruyomi:library:sources",
		Title:           "Library Sources",
		Updated:         now,
		Links: []OpdsLink{
			{Rel: OpdsRelSelf, Href: "/api/opds/v1.2/library/sources", Type: OpdsMimeNavigation},
			{Rel: OpdsRelStart, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
		},
	}

	for rows.Next() {
		var srcID int64
		var srcName, srcLang string
		var count int
		if err := rows.Scan(&srcID, &srcName, &srcLang, &count); err == nil {
			feed.Entries = append(feed.Entries, OpdsEntry{
				ID:      fmt.Sprintf("urn:tsuruyomi:source:%d", srcID),
				Title:   srcName,
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: fmt.Sprintf("%d series from %s (%s)", count, srcName, srcLang)},
				Links: []OpdsLink{
					{
						Rel:      OpdsRelSubsection,
						Href:     fmt.Sprintf("/api/opds/v1.2/source/%d", srcID),
						Type:     OpdsMimeAcquisition,
						ThrCount: &count,
					},
					{
						Rel:  OpdsRelImage,
						Href: fmt.Sprintf("/api/v1/source/%d/icon", srcID),
						Type: OpdsMimeJpeg,
					},
				},
			})
		}
	}

	total := len(feed.Entries)
	feed.TotalResults = &total

	return renderOpdsXML(c, feed, false)
}

// GetOpdsGenres serves the genres navigation feed.
// GET /api/opds/v1.2/library/genres
func GetOpdsGenres(c *fiber.Ctx) error {
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

	feed := &OpdsFeed{
		Xmlns:           OpdsNsAtom,
		XmlnsOpds:       OpdsNsOpds,
		XmlnsThr:        OpdsNsThr,
		XmlnsOpenSearch: OpdsNsOpenSearch,
		ID:              "urn:tsuruyomi:library:genres",
		Title:           "Library Genres",
		Updated:         now,
		Links: []OpdsLink{
			{Rel: OpdsRelSelf, Href: "/api/opds/v1.2/library/genres", Type: OpdsMimeNavigation},
			{Rel: OpdsRelStart, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
		},
	}

	for rows.Next() {
		var genreName string
		var count int
		if err := rows.Scan(&genreName, &count); err == nil {
			feed.Entries = append(feed.Entries, OpdsEntry{
				ID:      fmt.Sprintf("urn:tsuruyomi:genre:%s", url.PathEscape(genreName)),
				Title:   genreName,
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: fmt.Sprintf("%d series tagged '%s'", count, genreName)},
				Links: []OpdsLink{
					{
						Rel:      OpdsRelSubsection,
						Href:     fmt.Sprintf("/api/opds/v1.2/genre/%s", url.PathEscape(genreName)),
						Type:     OpdsMimeAcquisition,
						ThrCount: &count,
					},
				},
			})
		}
	}

	total := len(feed.Entries)
	feed.TotalResults = &total

	return renderOpdsXML(c, feed, false)
}

// GetOpdsStatuses serves the publication statuses navigation feed.
// GET /api/opds/v1.2/library/statuses
func GetOpdsStatuses(c *fiber.Ctx) error {
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

	feed := &OpdsFeed{
		Xmlns:           OpdsNsAtom,
		XmlnsOpds:       OpdsNsOpds,
		XmlnsThr:        OpdsNsThr,
		XmlnsOpenSearch: OpdsNsOpenSearch,
		ID:              "urn:tsuruyomi:library:statuses",
		Title:           "Library Publication Statuses",
		Updated:         now,
		Links: []OpdsLink{
			{Rel: OpdsRelSelf, Href: "/api/opds/v1.2/library/statuses", Type: OpdsMimeNavigation},
			{Rel: OpdsRelStart, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
		},
	}

	for rows.Next() {
		var statusInt, count int
		if err := rows.Scan(&statusInt, &count); err == nil {
			statusName := string(models.MangaStatusFromInt(statusInt))
			feed.Entries = append(feed.Entries, OpdsEntry{
				ID:      fmt.Sprintf("urn:tsuruyomi:status:%d", statusInt),
				Title:   statusName,
				Updated: now,
				Content: &OpdsTextElem{Type: "text", Value: fmt.Sprintf("%d series with status '%s'", count, statusName)},
				Links: []OpdsLink{
					{
						Rel:      OpdsRelSubsection,
						Href:     fmt.Sprintf("/api/opds/v1.2/status/%d", statusInt),
						Type:     OpdsMimeAcquisition,
						ThrCount: &count,
					},
				},
			})
		}
	}

	total := len(feed.Entries)
	feed.TotalResults = &total

	return renderOpdsXML(c, feed, false)
}

// GetOpdsLibraryUpdates serves recent chapter updates as an acquisition feed.
// GET /api/opds/v1.2/library-updates
func GetOpdsLibraryUpdates(c *fiber.Ctx) error {
	now := formatRFC3339(time.Now())

	page := 0
	if pStr := c.Query("pageNumber"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p - 1 // GetRecentChapters uses 0-indexed page
		}
	}

	result, err := database.GetRecentChapters(c.Context(), page)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	feed := &OpdsFeed{
		Xmlns:     OpdsNsAtom,
		XmlnsOpds: OpdsNsOpds,
		XmlnsPse:  OpdsNsPse,
		ID:        "urn:tsuruyomi:library:updates",
		Title:     "Library Updates",
		Updated:   now,
		Links: []OpdsLink{
			{Rel: OpdsRelSelf, Href: fmt.Sprintf("/api/opds/v1.2/library-updates?pageNumber=%d", page+1), Type: OpdsMimeAcquisition},
			{Rel: OpdsRelStart, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
		},
	}

	for _, mc := range result.Page {
		ch := mc.Chapter
		manga := mc.Manga

		chUpdated := now
		if ch.FetchedAt > 0 {
			chUpdated = formatRFC3339(time.Unix(ch.FetchedAt, 0))
		}

		pageCountInt := ch.PageCount
		lastPageInt := ch.LastPageRead

		feed.Entries = append(feed.Entries, OpdsEntry{
			ID:      fmt.Sprintf("urn:tsuruyomi:chapter:%d", ch.ID),
			Title:   fmt.Sprintf("%s - %s", manga.Title, ch.Name),
			Updated: chUpdated,
			Summary: &OpdsTextElem{Type: "text", Value: fmt.Sprintf("%s | Chapter %s", manga.Title, ch.Name)},
			Links: []OpdsLink{
				{
					Rel:   OpdsRelAcquisitionOpenAccess,
					Href:  fmt.Sprintf("/api/v1/chapter/%d/download?markAsRead=false", ch.ID),
					Type:  OpdsMimeCbz,
					Title: "Download CBZ",
				},
				{
					Rel:         OpdsRelPseStream,
					Href:        fmt.Sprintf("/api/v1/manga/%d/chapter/%d/page/{pageNumber}", manga.ID, ch.Index),
					Type:        OpdsMimeJpeg,
					Title:       "Stream Chapter Pages",
					PseCount:    &pageCountInt,
					PseLastRead: &lastPageInt,
				},
				{
					Rel:  OpdsRelImage,
					Href: fmt.Sprintf("/api/v1/manga/%d/thumbnail", manga.ID),
					Type: OpdsMimeJpeg,
				},
			},
			Publisher: manga.Title,
		})
	}

	return renderOpdsXML(c, feed, true)
}

// GetOpdsHistory serves reading history as an acquisition feed.
// GET /api/opds/v1.2/history
func GetOpdsHistory(c *fiber.Ctx) error {
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

	feed := &OpdsFeed{
		Xmlns:     OpdsNsAtom,
		XmlnsOpds: OpdsNsOpds,
		XmlnsPse:  OpdsNsPse,
		ID:        "urn:tsuruyomi:history",
		Title:     "Reading History",
		Updated:   now,
		Links: []OpdsLink{
			{Rel: OpdsRelSelf, Href: fmt.Sprintf("/api/opds/v1.2/history?pageNumber=%d", page), Type: OpdsMimeAcquisition},
			{Rel: OpdsRelStart, Href: "/api/opds/v1.2", Type: OpdsMimeNavigation},
		},
	}

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
		feed.Entries = append(feed.Entries, OpdsEntry{
			ID:      fmt.Sprintf("urn:tsuruyomi:history:%d", chID),
			Title:   fmt.Sprintf("%s: %s", mTitle, chName),
			Updated: readDate,
			Summary: &OpdsTextElem{Type: "text", Value: fmt.Sprintf("Last read at %s (page %d/%d)", readDate, lastPage, pageCount)},
			Links: []OpdsLink{
				{
					Rel:   OpdsRelAcquisitionOpenAccess,
					Href:  fmt.Sprintf("/api/v1/chapter/%d/download?markAsRead=false", chID),
					Type:  OpdsMimeCbz,
					Title: "Download CBZ",
				},
				{
					Rel:         OpdsRelPseStream,
					Href:        fmt.Sprintf("/api/v1/manga/%d/chapter/%d/page/{pageNumber}", mID, sourceOrder),
					Type:        OpdsMimeJpeg,
					Title:       "Continue Reading",
					PseCount:    &pageCount,
					PseLastRead: &lastPage,
				},
				{
					Rel:  OpdsRelImage,
					Href: fmt.Sprintf("/api/v1/manga/%d/thumbnail", mID),
					Type: OpdsMimeJpeg,
				},
			},
			Publisher: mTitle,
		})
	}

	return renderOpdsXML(c, feed, true)
}
