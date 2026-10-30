package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"tsuruyomi/internal/models"
)

// GetChaptersByMangaID returns a sorted list of chapters for a given manga.
func GetChaptersByMangaID(ctx context.Context, mangaId int) ([]models.ChapterDataClass, error) {
	if DB == nil {
		return []models.ChapterDataClass{}, nil
	}

	query := `
		SELECT c.id, c.url, c.name, c.date_upload, c.chapter_number, c.scanlator, 
		       COALESCE(cu.read, false), COALESCE(cu.bookmark, false), 
		       COALESCE(cu.last_page_read, 0), COALESCE(cu.last_read_at, 0), c.fetched_at, c.source_order, c.real_url, 
		       COALESCE(cu.is_downloaded, false), c.page_count, COALESCE(cu.last_modified_at, 0), COALESCE(cu.version, 0)
		FROM suwayomi.chapter c
		LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
		WHERE c.manga = $1
		ORDER BY c.source_order DESC, c.id DESC
	`

	rows, err := DB.Query(ctx, query, mangaId)
	if err != nil {
		return nil, fmt.Errorf("querying chapters: %w", err)
	}
	defer rows.Close()

	chapters := []models.ChapterDataClass{}
	for rows.Next() {
		var c models.ChapterDataClass
		err := rows.Scan(
			&c.ID, &c.URL, &c.Name, &c.UploadDate, &c.ChapterNumber, &c.Scanlator, &c.Read, &c.Bookmarked,
			&c.LastPageRead, &c.LastReadAt, &c.FetchedAt, &c.Index, &c.RealURL, &c.Downloaded, &c.PageCount,
			&c.LastModifiedAt, &c.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning chapter row: %w", err)
		}
		c.MangaID = mangaId
		c.Meta = map[string]string{}
		chapters = append(chapters, c)
	}

	return chapters, nil
}

// GetChapterByMangaAndIndex retrieves a single chapter using its index and parent manga ID.
func GetChapterByMangaAndIndex(ctx context.Context, mangaId int, chapterIndex int) (*models.ChapterDataClass, error) {
	if DB == nil {
		return nil, errors.New("database pool is not initialized")
	}

	query := `
		SELECT c.id, c.url, c.name, c.date_upload, c.chapter_number, c.scanlator, 
		       COALESCE(cu.read, false), COALESCE(cu.bookmark, false), 
		       COALESCE(cu.last_page_read, 0), COALESCE(cu.last_read_at, 0), c.fetched_at, c.source_order, c.real_url, 
		       COALESCE(cu.is_downloaded, false), c.page_count, COALESCE(cu.last_modified_at, 0), COALESCE(cu.version, 0)
		FROM suwayomi.chapter c
		LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
		WHERE c.manga = $1 AND (c.source_order = $2 OR ($2 = 0 AND c.source_order = 1))
		ORDER BY c.source_order ASC
		LIMIT 1
	`

	var c models.ChapterDataClass
	err := DB.QueryRow(ctx, query, mangaId, chapterIndex).Scan(
		&c.ID, &c.URL, &c.Name, &c.UploadDate, &c.ChapterNumber, &c.Scanlator, &c.Read, &c.Bookmarked,
		&c.LastPageRead, &c.LastReadAt, &c.FetchedAt, &c.Index, &c.RealURL, &c.Downloaded, &c.PageCount,
		&c.LastModifiedAt, &c.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying chapter: %w", err)
	}

	c.MangaID = mangaId
	c.Meta = map[string]string{}
	return &c, nil
}

// GetRecentChapters retrieves recently fetched chapters.
// Replicates Chapter.getRecentChapters logic: (ChapterTable innerJoin MangaTable)
// filtered by (MangaTable.inLibrary == true) and (ChapterTable.fetchedAt > MangaTable.inLibraryAt).
func GetRecentChapters(ctx context.Context, pageNum int) (*models.PaginatedList[models.MangaChapterDataClass], error) {
	if DB == nil {
		return &models.PaginatedList[models.MangaChapterDataClass]{
			Page:        []models.MangaChapterDataClass{},
			HasNextPage: false,
		}, nil
	}

	const pageSize = 50
	offset := pageNum * pageSize

	// 1. Get total count to determine hasNextPage
	countQuery := `
		SELECT COUNT(*)
		FROM suwayomi.chapter c
		INNER JOIN suwayomi.manga m ON c.manga = m.id
		LEFT JOIN suwayomi.mangauser mu ON m.id = mu.manga
		WHERE COALESCE(mu.in_library, false) = true AND c.fetched_at > COALESCE(mu.in_library_at, 0)
	`
	var totalCount int64
	err := DB.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("counting recent chapters: %w", err)
	}

	if int64(offset) >= totalCount {
		return &models.PaginatedList[models.MangaChapterDataClass]{
			Page:        []models.MangaChapterDataClass{},
			HasNextPage: false,
		}, nil
	}

	// 2. Fetch paginated list
	query := `
		SELECT 
			c.id as c_id, c.url as c_url, c.name as c_name, c.date_upload, c.chapter_number, 
			c.scanlator, COALESCE(cu.read, false), COALESCE(cu.bookmark, false), COALESCE(cu.last_page_read, 0), COALESCE(cu.last_read_at, 0), c.fetched_at, 
			c.source_order, c.real_url as c_real_url, COALESCE(cu.is_downloaded, false), c.page_count, 
			COALESCE(cu.last_modified_at, 0) as c_last_mod, COALESCE(cu.version, 0) as c_version,
			m.id as m_id, m.url as m_url, m.title, m.initialized, m.artist, m.author, 
			m.description, m.genre, m.status, m.thumbnail_url, m.thumbnail_url_last_fetched, 
			COALESCE(mu.in_library, false), COALESCE(mu.in_library_at, 0), m.source, m.real_url as m_real_url, m.last_fetched_at, 
			m.chapters_last_fetched_at, m.update_strategy, COALESCE(mu.last_modified_at, 0) as m_last_mod, COALESCE(mu.version, 0) as m_version, m.memo
		FROM suwayomi.chapter c
		LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
		INNER JOIN suwayomi.manga m ON c.manga = m.id
		LEFT JOIN suwayomi.mangauser mu ON m.id = mu.manga
		WHERE COALESCE(mu.in_library, false) = true AND c.fetched_at > COALESCE(mu.in_library_at, 0)
		ORDER BY c.fetched_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := DB.Query(ctx, query, pageSize+1, offset) // Fetch page+1 to verify next page presence
	if err != nil {
		return nil, fmt.Errorf("querying recent chapters: %w", err)
	}
	defer rows.Close()

	list := []models.MangaChapterDataClass{}
	for rows.Next() {
		var mc models.MangaChapterDataClass
		var genreStr sql.NullString
		var statusInt int
		var sourceID int64
		var memoStr string

		err := rows.Scan(
			&mc.Chapter.ID, &mc.Chapter.URL, &mc.Chapter.Name, &mc.Chapter.UploadDate, &mc.Chapter.ChapterNumber,
			&mc.Chapter.Scanlator, &mc.Chapter.Read, &mc.Chapter.Bookmarked, &mc.Chapter.LastPageRead, &mc.Chapter.LastReadAt,
			&mc.Chapter.FetchedAt, &mc.Chapter.Index, &mc.Chapter.RealURL, &mc.Chapter.Downloaded, &mc.Chapter.PageCount,
			&mc.Chapter.LastModifiedAt, &mc.Chapter.Version,
			&mc.Manga.ID, &mc.Manga.URL, &mc.Manga.Title, &mc.Manga.Initialized, &mc.Manga.Artist, &mc.Manga.Author,
			&mc.Manga.Description, &genreStr, &statusInt, &mc.Manga.ThumbnailURL, &mc.Manga.ThumbnailURLLastFetched,
			&mc.Manga.InLibrary, &mc.Manga.InLibraryAt, &sourceID, &mc.Manga.RealURL, &mc.Manga.LastFetchedAt,
			&mc.Manga.ChaptersLastFetchedAt, &mc.Manga.UpdateStrategy, &mc.Manga.LastModifiedAt, &mc.Manga.Version, &memoStr,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning recent chapter row: %w", err)
		}

		mc.Chapter.MangaID = mc.Manga.ID
		mc.Chapter.Meta = map[string]string{}
		mc.Manga.SourceID = fmt.Sprintf("%d", sourceID)
		mc.Manga.Status = models.MangaStatusFromInt(statusInt)
		mc.Manga.Meta = map[string]string{}
		mc.Manga.Trackers = []models.MangaTrackerDataClass{}

		list = append(list, mc)
	}

	hasNextPage := false
	if len(list) > pageSize {
		hasNextPage = true
		list = list[:pageSize]
	}

	return &models.PaginatedList[models.MangaChapterDataClass]{
		Page:        list,
		HasNextPage: hasNextPage,
	}, nil
}

// EnsureCleanChapterSourceOrders checks for mangas whose chapters have duplicate or non-contiguous source_orders,
// and normalizes them so that source_order is strictly 1..N.
func EnsureCleanChapterSourceOrders(ctx context.Context) error {
	if DB == nil {
		return nil
	}

	query := `
		WITH broken_manga AS (
			SELECT manga
			FROM suwayomi.chapter
			GROUP BY manga
			HAVING min(source_order) != 1 
			   OR max(source_order) != count(*) 
			   OR count(distinct source_order) != count(*)
		),
		numbered AS (
			SELECT c.id, ROW_NUMBER() OVER (
				PARTITION BY c.manga
				ORDER BY 
					CASE WHEN c.chapter_number > 0 THEN c.chapter_number ELSE c.source_order END ASC,
					c.id ASC
			) as new_order
			FROM suwayomi.chapter c
			WHERE c.manga IN (SELECT manga FROM broken_manga)
		)
		UPDATE suwayomi.chapter c
		SET source_order = n.new_order
		FROM numbered n
		WHERE c.id = n.id;
	`
	tag, err := DB.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("normalizing chapter source orders: %w", err)
	}
	if tag.RowsAffected() > 0 {
		fmt.Printf("[DATABASE] Normalized source_order for %d chapters with gaps or collisions\n", tag.RowsAffected())
	}
	return nil
}

