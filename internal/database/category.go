package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"tsuruyomi/internal/models"
)

// GetCategoryList returns all categories along with the count of manga in each category.
func GetCategoryList(ctx context.Context) ([]models.CategoryDataClass, error) {
	if DB == nil {
		return []models.CategoryDataClass{}, nil
	}

	query := `
		SELECT 
			c.id, c.name, c.sort_order, c.is_default, c.include_in_update, 
			c.include_in_download, c.version, c.uid, c.last_modified_at,
			COUNT(DISTINCT cm.manga) as size
		FROM suwayomi.category c
		LEFT JOIN suwayomi.categorymanga cm ON c.id = cm.category
		GROUP BY c.id
		ORDER BY c.sort_order ASC
	`

	rows, err := DB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying categories: %w", err)
	}
	defer rows.Close()

	list := []models.CategoryDataClass{}
	for rows.Next() {
		var cat models.CategoryDataClass
		var updateInt, downloadInt int
		err := rows.Scan(
			&cat.ID, &cat.Name, &cat.Order, &cat.Default, &updateInt,
			&downloadInt, &cat.Version, &cat.UID, &cat.LastModifiedAt,
			&cat.Size,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning category row: %w", err)
		}
		cat.IncludeInUpdate = models.IncludeOrExclude(updateInt)
		cat.IncludeInDownload = models.IncludeOrExclude(downloadInt)
		cat.Meta = map[string]string{}
		list = append(list, cat)
	}

	return list, nil
}

// GetCategoryMangas returns all manga associated with a specific category ID.
func GetCategoryMangas(ctx context.Context, categoryId int) ([]models.MangaDataClass, error) {
	if DB == nil {
		return []models.MangaDataClass{}, nil
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
		INNER JOIN suwayomi.categorymanga cm ON m.id = cm.manga
		LEFT JOIN suwayomi.source s ON m.source = s.id
		LEFT JOIN suwayomi.extension e ON s.extension = e.id
		WHERE cm.category = $1
	`

	rows, err := DB.Query(ctx, query, categoryId)
	if err != nil {
		return nil, fmt.Errorf("querying category mangas: %w", err)
	}
	defer rows.Close()

	list := []models.MangaDataClass{}
	for rows.Next() {
		var manga models.MangaDataClass
		var genreStr sql.NullString
		var statusInt int
		var sourceID int64
		var sourceName, sourceLang, sourceIconURL sql.NullString
		var memoStr string

		err := rows.Scan(
			&manga.ID, &manga.URL, &manga.Title, &manga.Initialized, &manga.Artist, &manga.Author, &manga.Description,
			&genreStr, &statusInt, &manga.ThumbnailURL, &manga.ThumbnailURLLastFetched,
			&manga.InLibrary, &manga.InLibraryAt, &sourceID, &manga.RealURL, &manga.LastFetchedAt,
			&manga.ChaptersLastFetchedAt, &manga.UpdateStrategy, &manga.LastModifiedAt, &manga.Version, &memoStr,
			&sourceName, &sourceLang, &sourceIconURL,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning category manga row: %w", err)
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

		if sourceName.Valid {
			manga.Source = &models.SourceDataClass{
				ID:             manga.SourceID,
				Name:           sourceName.String,
				Lang:           sourceLang.String,
				IconURL:        sourceIconURL.String,
				SupportsLatest: true,
				DisplayName:    sourceName.String,
			}
		}

		manga.Meta = map[string]string{}
		manga.Trackers = []models.MangaTrackerDataClass{}

		list = append(list, manga)
	}

	return list, nil
}

// GetMangaCategories returns all category IDs that a specific manga belongs to.
func GetMangaCategories(ctx context.Context, mangaId int) ([]models.CategoryDataClass, error) {
	if DB == nil {
		return []models.CategoryDataClass{}, nil
	}

	query := `
		SELECT 
			c.id, c.name, c.sort_order, c.is_default, c.include_in_update, 
			c.include_in_download, c.version, c.uid, c.last_modified_at
		FROM suwayomi.category c
		INNER JOIN suwayomi.categorymanga cm ON c.id = cm.category
		WHERE cm.manga = $1
		ORDER BY c.sort_order ASC
	`

	rows, err := DB.Query(ctx, query, mangaId)
	if err != nil {
		return nil, fmt.Errorf("querying manga categories: %w", err)
	}
	defer rows.Close()

	list := []models.CategoryDataClass{}
	for rows.Next() {
		var cat models.CategoryDataClass
		var updateInt, downloadInt int
		err := rows.Scan(
			&cat.ID, &cat.Name, &cat.Order, &cat.Default, &updateInt,
			&downloadInt, &cat.Version, &cat.UID, &cat.LastModifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning manga category row: %w", err)
		}
		cat.IncludeInUpdate = models.IncludeOrExclude(updateInt)
		cat.IncludeInDownload = models.IncludeOrExclude(downloadInt)
		cat.Meta = map[string]string{}
		list = append(list, cat)
	}

	return list, nil
}

// CreateCategory adds a new category.
func CreateCategory(ctx context.Context, name string) (int, error) {
	if DB == nil {
		return -1, fmt.Errorf("database not initialized")
	}

	nameLower := strings.ToLower(name)
	if nameLower == "default" {
		return 0, nil
	}

	// Start a transaction
	tx, err := DB.Begin(ctx)
	if err != nil {
		return -1, err
	}
	defer tx.Rollback(ctx)

	// Check if category name already exists
	var existingId int
	err = tx.QueryRow(ctx, "SELECT id FROM suwayomi.category WHERE LOWER(name) = $1 LIMIT 1", nameLower).Scan(&existingId)
	if err == nil {
		// Already exists, return existing ID
		return existingId, nil
	}

	// Insert new category
	var newId int
	insertQuery := `
		INSERT INTO suwayomi.category (name, sort_order, is_default, include_in_update, include_in_download, version, uid, last_modified_at, is_syncing)
		VALUES ($1, 999999, false, -1, -1, 0, 0, EXTRACT(EPOCH FROM NOW())::bigint, false)
		RETURNING id
	`
	err = tx.QueryRow(ctx, insertQuery, name).Scan(&newId)
	if err != nil {
		return -1, fmt.Errorf("inserting category: %w", err)
	}

	// Normalize
	if err := normalizeCategoriesTx(ctx, tx); err != nil {
		return -1, fmt.Errorf("normalizing categories: %w", err)
	}

	return newId, tx.Commit(ctx)
}

// UpdateCategory updates category attributes.
func UpdateCategory(ctx context.Context, categoryId int, name *string, isDefault *bool, includeInUpdate *int, includeInDownload *int) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	tx, err := DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Build update query dynamically
	var setParts []string
	var args []interface{}
	argIdx := 1

	if categoryId != 0 && name != nil && strings.ToLower(*name) != "default" {
		setParts = append(setParts, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *name)
		argIdx++
	}

	if categoryId != 0 && isDefault != nil {
		setParts = append(setParts, fmt.Sprintf("is_default = $%d", argIdx))
		args = append(args, *isDefault)
		argIdx++
	}

	if includeInUpdate != nil {
		setParts = append(setParts, fmt.Sprintf("include_in_update = $%d", argIdx))
		args = append(args, *includeInUpdate)
		argIdx++
	}

	if includeInDownload != nil {
		setParts = append(setParts, fmt.Sprintf("include_in_download = $%d", argIdx))
		args = append(args, *includeInDownload)
		argIdx++
	}

	if len(setParts) == 0 {
		return nil
	}

	setParts = append(setParts, fmt.Sprintf("last_modified_at = $%d", argIdx))
	args = append(args, time.Now().Unix())
	argIdx++

	args = append(args, categoryId)
	query := fmt.Sprintf("UPDATE suwayomi.category SET %s WHERE id = $%d", strings.Join(setParts, ", "), argIdx)

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating category: %w", err)
	}

	return tx.Commit(ctx)
}

// DeleteCategory deletes a category and normalizes.
func DeleteCategory(ctx context.Context, categoryId int) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if categoryId == 0 {
		return nil
	}

	tx, err := DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "DELETE FROM suwayomi.category WHERE id = $1", categoryId)
	if err != nil {
		return fmt.Errorf("deleting category: %w", err)
	}

	if err := normalizeCategoriesTx(ctx, tx); err != nil {
		return fmt.Errorf("normalizing categories after delete: %w", err)
	}

	return tx.Commit(ctx)
}

// ReorderCategory moves order positions.
func ReorderCategory(ctx context.Context, from, to int) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if from == 0 || to == 0 {
		return nil
	}

	tx, err := DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	type catOrder struct {
		ID    int
		Order int
	}

	// Fetch all except default category (id 0) ordered by sort_order
	rows, err := tx.Query(ctx, "SELECT id, sort_order FROM suwayomi.category WHERE id != 0 ORDER BY sort_order ASC")
	if err != nil {
		return err
	}

	var list []catOrder
	for rows.Next() {
		var co catOrder
		if err := rows.Scan(&co.ID, &co.Order); err == nil {
			list = append(list, co)
		}
	}
	rows.Close()

	if len(list) == 0 {
		return nil
	}

	fromIdx := from - 1
	toIdx := to - 1

	if fromIdx < 0 || fromIdx >= len(list) || toIdx < 0 || toIdx >= len(list) {
		return fmt.Errorf("reorder index out of bounds")
	}

	// Remove and insert
	target := list[fromIdx]
	list = append(list[:fromIdx], list[fromIdx+1:]...)

	// Insert at toIdx
	list = append(list[:toIdx], append([]catOrder{target}, list[toIdx:]...)...)

	// Update order numbers in DB
	for i, item := range list {
		// we offset order by 1 (the default category is order 0)
		_, err = tx.Exec(ctx, "UPDATE suwayomi.category SET sort_order = $1 WHERE id = $2", i+1, item.ID)
		if err != nil {
			return err
		}
	}

	if err := normalizeCategoriesTx(ctx, tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func normalizeCategoriesTx(ctx context.Context, tx pgx.Tx) error {
	type catRecord struct {
		ID    int
		Order int
	}

	rows, err := tx.Query(ctx, "SELECT id, sort_order FROM suwayomi.category ORDER BY sort_order ASC")
	if err != nil {
		return err
	}

	var list []catRecord
	for rows.Next() {
		var cr catRecord
		if err := rows.Scan(&cr.ID, &cr.Order); err == nil {
			list = append(list, cr)
		}
	}
	rows.Close()

	// Sort: id = 0 first, then order by existing order
	sort.Slice(list, func(i, j int) bool {
		if list[i].ID == 0 {
			return true
		}
		if list[j].ID == 0 {
			return false
		}
		return list[i].Order < list[j].Order
	})

	// Update back to DB
	for i, item := range list {
		_, err = tx.Exec(ctx, "UPDATE suwayomi.category SET sort_order = $1 WHERE id = $2", i, item.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetCategoryByID returns a single category by ID.
func GetCategoryByID(ctx context.Context, categoryId int) (*models.CategoryDataClass, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT 
			c.id, c.name, c.sort_order, c.is_default, c.include_in_update, 
			c.include_in_download, c.version, c.uid, c.last_modified_at,
			COUNT(DISTINCT cm.manga) as size
		FROM suwayomi.category c
		LEFT JOIN suwayomi.categorymanga cm ON c.id = cm.category
		WHERE c.id = $1
		GROUP BY c.id
	`
	var cat models.CategoryDataClass
	var updateInt, downloadInt int
	err := DB.QueryRow(ctx, query, categoryId).Scan(
		&cat.ID, &cat.Name, &cat.Order, &cat.Default, &updateInt,
		&downloadInt, &cat.Version, &cat.UID, &cat.LastModifiedAt,
		&cat.Size,
	)
	if err != nil {
		return nil, err
	}
	cat.IncludeInUpdate = models.IncludeOrExclude(updateInt)
	cat.IncludeInDownload = models.IncludeOrExclude(downloadInt)
	cat.Meta = map[string]string{}
	return &cat, nil
}

