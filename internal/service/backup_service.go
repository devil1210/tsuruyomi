package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
)

// MissingSourceInfo represents a source present in a backup whose extension is not installed.
type MissingSourceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ValidateBackupResult contains the results of validating a backup against installed sources and extensions.
type ValidateBackupResult struct {
	MissingSources  []MissingSourceInfo `json:"missingSources"`
	MissingTrackers []interface{}       `json:"missingTrackers"`
}

// BackupPayload is the legacy backup format structure for exporting and importing JSON data.
type BackupPayload struct {
	Categories    []models.CategoryDataClass `json:"categories"`
	Mangas        []models.MangaDataClass    `json:"mangas"`
	Chapters      []models.ChapterDataClass  `json:"chapters"`
	CategoryManga []CategoryMangaLink        `json:"categoryManga"`
}

// CategoryMangaLink represents a binding between a category and a manga.
type CategoryMangaLink struct {
	CategoryID int `json:"categoryId"`
	MangaID    int `json:"mangaId"`
}

// ExportBackup exports database contents as standard Tachiyomi/Suwayomi .tachibk format (gzipped protobuf).
func ExportBackup(ctx context.Context) ([]byte, error) {
	return ExportTachibk(ctx)
}

// ImportBackup parses the payload and restores categories, library mangas, and chapters history.
// It supports standard Tachiyomi/Suwayomi Protobuf (.tachibk / .proto.gz) as well as legacy JSON backups.
func ImportBackup(ctx context.Context, data []byte) error {
	if database.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if len(data) == 0 {
		return fmt.Errorf("empty backup payload")
	}

	// 1. Decompress if gzipped (.tachibk / .proto.gz)
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

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var payload BackupPayload
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return fmt.Errorf("decoding legacy backup JSON: %w", err)
		}
		pb := jsonPayloadToProto(&payload)
		return RestoreFromProtoBackup(ctx, pb)
	}

	// 2. Decode standard Tachiyomi/Suwayomi Protobuf wire bytes
	pb, err := DecodeProtoBackup(data)
	if err != nil {
		return fmt.Errorf("decoding protobuf backup: %w", err)
	}

	return RestoreFromProtoBackup(ctx, pb)
}

func jsonPayloadToProto(p *BackupPayload) *ProtoBackup {
	pb := &ProtoBackup{}
	for _, c := range p.Categories {
		pb.BackupCategories = append(pb.BackupCategories, ProtoBackupCategory{
			Name:  c.Name,
			Order: int64(c.Order),
			Flags: 0,
		})
	}

	for _, m := range p.Mangas {
		sourceID, _ := strconv.ParseInt(m.SourceID, 10, 64)
		var cats []int64
		for _, link := range p.CategoryManga {
			if link.MangaID == m.ID {
				cats = append(cats, int64(link.CategoryID))
			}
		}

		var chs []ProtoBackupChapter
		var history []ProtoBackupHistory
		for _, ch := range p.Chapters {
			if ch.MangaID == m.ID {
				scan := ""
				if ch.Scanlator != nil {
					scan = *ch.Scanlator
				}
				chs = append(chs, ProtoBackupChapter{
					URL:           ch.URL,
					Name:          ch.Name,
					Scanlator:     scan,
					Read:          ch.Read,
					Bookmark:      ch.Bookmarked,
					LastPageRead:  int64(ch.LastPageRead),
					DateUpload:    ch.UploadDate,
					ChapterNumber: float32(ch.ChapterNumber),
					SourceOrder:   int64(ch.Index),
				})
				if ch.LastReadAt > 0 {
					history = append(history, ProtoBackupHistory{
						URL:      ch.URL,
						LastRead: ch.LastReadAt,
					})
				}
			}
		}

		thumb := ""
		if m.ThumbnailURL != nil {
			thumb = *m.ThumbnailURL
		}
		artist := ""
		if m.Artist != nil {
			artist = *m.Artist
		}
		author := ""
		if m.Author != nil {
			author = *m.Author
		}
		desc := ""
		if m.Description != nil {
			desc = *m.Description
		}

		pb.BackupManga = append(pb.BackupManga, ProtoBackupManga{
			Source:       sourceID,
			URL:          m.URL,
			Title:        m.Title,
			Artist:       artist,
			Author:       author,
			Description:  desc,
			Genre:        m.Genre,
			Status:       models.MangaStatusToInt(m.Status),
			ThumbnailURL: thumb,
			DateAdded:    m.InLibraryAt,
			Chapters:     chs,
			Categories:   cats,
			Favorite:     true,
			History:      history,
		})
	}

	return pb
}

// ValidateBackup inspects the backup data and identifies sources whose extensions are missing or not installed.
func ValidateBackup(ctx context.Context, data []byte) (*ValidateBackupResult, error) {
	res := &ValidateBackupResult{
		MissingSources:  []MissingSourceInfo{},
		MissingTrackers: []interface{}{},
	}
	if len(data) == 0 {
		return res, nil
	}

	// 1. Decompress if gzipped (.tachibk / .proto.gz)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decompressing gzip backup: %w", err)
		}
		defer zr.Close()
		decompressed, err := io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("reading decompressed backup data: %w", err)
		}
		data = decompressed
	}

	var pb *ProtoBackup
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var payload BackupPayload
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return nil, fmt.Errorf("decoding legacy backup JSON: %w", err)
		}
		pb = jsonPayloadToProto(&payload)
	} else {
		var err error
		pb, err = DecodeProtoBackup(data)
		if err != nil {
			return nil, fmt.Errorf("decoding protobuf backup: %w", err)
		}
	}

	sourceMap := make(map[int64]string)
	for _, s := range pb.BackupSources {
		if s.SourceID > 0 {
			sourceMap[s.SourceID] = s.Name
		}
	}
	for _, m := range pb.BackupManga {
		if m.Source > 0 {
			if _, ok := sourceMap[m.Source]; !ok {
				sourceMap[m.Source] = ""
			}
		}
	}

	if database.DB == nil {
		return res, nil
	}

	for srcID, srcName := range sourceMap {
		if srcID == 0 { // Local source is always available
			continue
		}

		var isInstalled bool
		var pkgName, dbName string
		err := database.DB.QueryRow(ctx, `
			SELECT COALESCE(e.is_installed, false), COALESCE(e.pkg_name, ''), COALESCE(s.name, '')
			FROM suwayomi.source s
			LEFT JOIN suwayomi.extension e ON s.extension = e.id
			WHERE s.id = $1
		`, srcID).Scan(&isInstalled, &pkgName, &dbName)

		if err != nil || !isInstalled || strings.HasPrefix(pkgName, "imported.source.") {
			finalName := srcName
			if finalName == "" && dbName != "" {
				finalName = dbName
			}
			if finalName == "" {
				if known, ok := database.KnownSourcesByID[srcID]; ok && known.Name != "" {
					finalName = known.Name
				}
			}
			if finalName == "" {
				finalName = fmt.Sprintf("Source %d", srcID)
			}

			res.MissingSources = append(res.MissingSources, MissingSourceInfo{
				ID:   strconv.FormatInt(srcID, 10),
				Name: finalName,
			})
		}
	}

	return res, nil
}
