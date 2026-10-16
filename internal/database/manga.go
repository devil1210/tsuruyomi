package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"tsuruyomi/internal/models"
)

// GetMangaByID retrieves basic manga details, count statistics, and source details.
func GetMangaByID(ctx context.Context, id int) (*models.MangaDataClass, error) {
	if DB == nil {
		return nil, errors.New("database pool is not initialized")
	}

	query := `
		SELECT 
			m.id, m.url, m.title, m.initialized, m.artist, m.author, m.description, 
			m.genre, m.status, m.thumbnail_url, m.thumbnail_url_last_fetched, 
			COALESCE(mu.in_library, false), COALESCE(mu.in_library_at, 0), m.source, m.real_url, m.last_fetched_at, 
			m.chapters_last_fetched_at, m.update_strategy, COALESCE(mu.last_modified_at, 0), COALESCE(mu.version, 0), m.memo,
			s.name as source_name, s.lang as source_lang,
			e.icon_url as source_icon_url
		FROM suwayomi.manga m
		LEFT JOIN suwayomi.mangauser mu ON m.id = mu.manga
		LEFT JOIN suwayomi.source s ON m.source = s.id
		LEFT JOIN suwayomi.extension e ON s.extension = e.id
		WHERE m.id = $1
	`

	var manga models.MangaDataClass
	var genreStr sql.NullString
	var statusInt int
	var sourceID int64
	var sourceName, sourceLang, sourceIconURL sql.NullString
	var memoStr string

	err := DB.QueryRow(ctx, query, id).Scan(
		&manga.ID, &manga.URL, &manga.Title, &manga.Initialized, &manga.Artist, &manga.Author, &manga.Description,
		&genreStr, &statusInt, &manga.ThumbnailURL, &manga.ThumbnailURLLastFetched,
		&manga.InLibrary, &manga.InLibraryAt, &sourceID, &manga.RealURL, &manga.LastFetchedAt,
		&manga.ChaptersLastFetchedAt, &manga.UpdateStrategy, &manga.LastModifiedAt, &manga.Version, &memoStr,
		&sourceName, &sourceLang, &sourceIconURL,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying manga: %w", err)
	}

	manga.SourceID = fmt.Sprintf("%d", sourceID)
	manga.Status = models.MangaStatusFromInt(statusInt)

	if genreStr.Valid && genreStr.String != "" {
		manga.Genre = strings.Split(genreStr.String, ",")
		for i, g := range manga.Genre {
			manga.Genre[i] = strings.TrimSpace(g)
		}
	} else {
		manga.Genre = []string{}
	}

	// Populate Source
	if sourceName.Valid {
		isNsfw := false // Fallback
		manga.Source = &models.SourceDataClass{
			ID:             manga.SourceID,
			Name:           sourceName.String,
			Lang:           sourceLang.String,
			IconURL:        sourceIconURL.String,
			SupportsLatest: true,
			IsConfigurable: false,
			IsNsfw:         isNsfw,
			DisplayName:    sourceName.String,
		}
	}

	// Fetch Counts & Stats
	countsQuery := `
		SELECT 
			COUNT(*),
			COALESCE(SUM(CASE WHEN read = false THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_downloaded = true THEN 1 ELSE 0 END), 0),
			MAX(last_read_at)
		FROM suwayomi.chapter
		WHERE manga = $1
	`
	var chapterCount, unreadCount, downloadCount int64
	var lastReadAt sql.NullInt64

	err = DB.QueryRow(ctx, countsQuery, id).Scan(&chapterCount, &unreadCount, &downloadCount, &lastReadAt)
	if err == nil {
		manga.ChapterCount = &chapterCount
		manga.UnreadCount = &unreadCount
		manga.DownloadCount = &downloadCount
		if lastReadAt.Valid && lastReadAt.Int64 > 0 {
			manga.LastReadAt = &lastReadAt.Int64
		}
	}

	// Fetch Last Chapter Read details
	if manga.LastReadAt != nil && *manga.LastReadAt > 0 {
		lastReadChapterQuery := `
			SELECT id, url, name, date_upload, chapter_number, scanlator, read, bookmark, 
			       last_page_read, last_read_at, fetched_at, source_order, real_url, 
			       is_downloaded, page_count, last_modified_at, version
			FROM suwayomi.chapter
			WHERE manga = $1 AND last_read_at = $2
			LIMIT 1
		`
		var lastCh models.ChapterDataClass
		err = DB.QueryRow(ctx, lastReadChapterQuery, id, *manga.LastReadAt).Scan(
			&lastCh.ID, &lastCh.URL, &lastCh.Name, &lastCh.UploadDate, &lastCh.ChapterNumber,
			&lastCh.Scanlator, &lastCh.Read, &lastCh.Bookmarked, &lastCh.LastPageRead,
			&lastCh.LastReadAt, &lastCh.FetchedAt, &lastCh.Index, &lastCh.RealURL,
			&lastCh.Downloaded, &lastCh.PageCount, &lastCh.LastModifiedAt, &lastCh.Version,
		)
		if err == nil {
			lastCh.MangaID = id
			manga.LastChapterRead = &lastCh
		}
	}

	// Compute Age
	now := time.Now().Unix()
	if manga.LastFetchedAt != nil {
		age := now - *manga.LastFetchedAt
		manga.Age = &age
	}
	if manga.ChaptersLastFetchedAt != nil {
		chAge := now - *manga.ChaptersLastFetchedAt
		manga.ChaptersAge = &chAge
	}

	// Trackers (empty stub)
	manga.Trackers = []models.MangaTrackerDataClass{}

	// Manga Metadata (KV Store)
	metaQuery := `SELECT key, value FROM suwayomi.manga_meta WHERE manga = $1`
	rows, err := DB.Query(ctx, metaQuery, id)
	if err == nil {
		manga.Meta = make(map[string]string)
		for rows.Next() {
			var key, val string
			if err := rows.Scan(&key, &val); err == nil {
				manga.Meta[key] = val
			}
		}
		rows.Close()
	}

	return &manga, nil
}
