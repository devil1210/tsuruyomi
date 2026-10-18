package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
)

// BackupPayload is the unified backup format structure for exporting and importing data.
type BackupPayload struct {
	Categories     []models.CategoryDataClass `json:"categories"`
	Mangas         []models.MangaDataClass    `json:"mangas"`
	Chapters       []models.ChapterDataClass  `json:"chapters"`
	CategoryManga  []CategoryMangaLink        `json:"categoryManga"`
}

// CategoryMangaLink represents a binding between a category and a manga.
type CategoryMangaLink struct {
	CategoryID int `json:"categoryId"`
	MangaID    int `json:"mangaId"`
}

// ExportBackup dumps categories, library mangas, chapters history, and links.
func ExportBackup(ctx context.Context) ([]byte, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	payload := BackupPayload{
		Categories:    []models.CategoryDataClass{},
		Mangas:        []models.MangaDataClass{},
		Chapters:      []models.ChapterDataClass{},
		CategoryManga: []CategoryMangaLink{},
	}

	// 1. Export Categories
	cats, err := database.GetCategoryList(ctx)
	if err == nil {
		payload.Categories = cats
	}

	// 2. Export Library Mangas
	rows, err := database.DB.Query(ctx, "SELECT id FROM suwayomi.manga WHERE in_library = true")
	if err == nil {
		var ids []int
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()

		for _, id := range ids {
			manga, err := database.GetMangaByID(ctx, id)
			if err == nil && manga != nil {
				payload.Mangas = append(payload.Mangas, *manga)
			}
		}
	}

	// 3. Export Chapters with Reading History (read/bookmarked/lastReadAt > 0)
	chapterQuery := `
		SELECT id, url, name, date_upload, chapter_number, scanlator, manga, read, bookmark, 
		       last_page_read, last_read_at, fetched_at, source_order, real_url, 
		       is_downloaded, page_count, last_modified_at, version
		FROM suwayomi.chapter
		WHERE read = true OR bookmark = true OR last_read_at > 0
	`
	rowsCh, err := database.DB.Query(ctx, chapterQuery)
	if err == nil {
		for rowsCh.Next() {
			var c models.ChapterDataClass
			err := rowsCh.Scan(
				&c.ID, &c.URL, &c.Name, &c.UploadDate, &c.ChapterNumber, &c.Scanlator, &c.MangaID, &c.Read, &c.Bookmarked,
				&c.LastPageRead, &c.LastReadAt, &c.FetchedAt, &c.Index, &c.RealURL, &c.Downloaded, &c.PageCount,
				&c.LastModifiedAt, &c.Version,
			)
			if err == nil {
				payload.Chapters = append(payload.Chapters, c)
			}
		}
		rowsCh.Close()
	}

	// 4. Export Category Manga links
	rowsLinks, err := database.DB.Query(ctx, "SELECT category, manga FROM suwayomi.categorymanga")
	if err == nil {
		for rowsLinks.Next() {
			var l CategoryMangaLink
			if err := rowsLinks.Scan(&l.CategoryID, &l.MangaID); err == nil {
				payload.CategoryManga = append(payload.CategoryManga, l)
			}
		}
		rowsLinks.Close()
	}

	return json.Marshal(payload)
}

// ImportBackup parses the payload and restores categories, library mangas, and chapters history.
func ImportBackup(ctx context.Context, data []byte) error {
	if database.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// Support gzip decompression if data is compressed (.tachibk / .proto.gz format)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("decompressing gzip backup: %w", err)
		}
		defer zr.Close()
		decompressed, err := io.ReadAll(zr)
		if err != nil {
			return fmt.Errorf("reading decompressed backup data: %w", err)
		}
		data = decompressed
	}

	var payload BackupPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decoding backup JSON: %w", err)
	}

	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Restore Categories
	for _, cat := range payload.Categories {
		if cat.ID == 0 {
			continue // Skip default category ID 0
		}
		var exists bool
		_ = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM suwayomi.category WHERE id = $1)", cat.ID).Scan(&exists)
		if !exists {
			insertQuery := `
				INSERT INTO suwayomi.category (id, name, sort_order, is_default, include_in_update, include_in_download, version, uid, last_modified_at, is_syncing)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false)
			`
			_, err = tx.Exec(ctx, insertQuery, cat.ID, cat.Name, cat.Order, cat.Default, int(cat.IncludeInUpdate), int(cat.IncludeInDownload), cat.Version, cat.UID, cat.LastModifiedAt)
			if err != nil {
				return fmt.Errorf("importing category %s: %w", cat.Name, err)
			}
		}
	}

	// 2. Restore Library Mangas (and update in_library fields)
	for _, m := range payload.Mangas {
		var exists bool
		_ = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM suwayomi.manga WHERE id = $1)", m.ID).Scan(&exists)
		if !exists {
			// If it does not exist, we insert it
			insertQuery := `
				INSERT INTO suwayomi.manga (id, url, title, initialized, artist, author, description, genre, status, thumbnail_url, in_library, in_library_at, source, last_modified_at, version, memo)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, '')
			`
			genreStr := ""
			if len(m.Genre) > 0 {
				genreStr = fmt.Sprintf("%v", m.Genre)
			}
			_, err = tx.Exec(ctx, insertQuery, m.ID, m.URL, m.Title, m.Initialized, m.Artist, m.Author, m.Description, genreStr, 0, m.ThumbnailURL, m.InLibrary, m.InLibraryAt, m.SourceID, m.LastModifiedAt, m.Version)
			if err != nil {
				return fmt.Errorf("importing manga %s: %w", m.Title, err)
			}
		} else {
			// Update inLibrary flags
			_, err = tx.Exec(ctx, "UPDATE suwayomi.manga SET in_library = $1, in_library_at = $2 WHERE id = $3", m.InLibrary, m.InLibraryAt, m.ID)
			if err != nil {
				return err
			}
		}
	}

	// 3. Restore Chapters reading history
	for _, c := range payload.Chapters {
		var exists bool
		_ = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM suwayomi.chapter WHERE id = $1)", c.ID).Scan(&exists)
		if exists {
			updateQuery := `
				UPDATE suwayomi.chapter 
				SET read = $1, bookmark = $2, last_page_read = $3, last_read_at = $4 
				WHERE id = $5
			`
			_, err = tx.Exec(ctx, updateQuery, c.Read, c.Bookmarked, c.LastPageRead, c.LastReadAt, c.ID)
			if err != nil {
				return fmt.Errorf("updating chapter history: %w", err)
			}
		}
	}

	// 4. Restore Category Manga links
	for _, link := range payload.CategoryManga {
		var exists bool
		_ = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM suwayomi.categorymanga WHERE category = $1 AND manga = $2)", link.CategoryID, link.MangaID).Scan(&exists)
		if !exists {
			_, err = tx.Exec(ctx, "INSERT INTO suwayomi.categorymanga (category, manga) VALUES ($1, $2)", link.CategoryID, link.MangaID)
			if err != nil {
				return fmt.Errorf("linking manga to category: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}
