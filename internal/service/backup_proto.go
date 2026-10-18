package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protowire"

	"tsuruyomi/internal/database"
)

func appendBool(b []byte, v bool) []byte {
	if v {
		return protowire.AppendVarint(b, 1)
	}
	return protowire.AppendVarint(b, 0)
}

// Tachiyomi/Suwayomi Protobuf models

type ProtoBackup struct {
	BackupManga      []ProtoBackupManga
	BackupCategories []ProtoBackupCategory
	BackupSources    []ProtoBackupSource
}

type ProtoBackupCategory struct {
	Name  string
	Order int64
	Flags int64
}

type ProtoBackupChapter struct {
	URL           string
	Name          string
	Scanlator     string
	Read          bool
	Bookmark      bool
	LastPageRead  int64
	DateFetch     int64
	DateUpload    int64
	ChapterNumber float32
	SourceOrder   int64
}

type ProtoBackupHistory struct {
	URL          string
	LastRead     int64
	ReadDuration int64
}

type ProtoBackupManga struct {
	Source         int64
	URL            string
	Title          string
	Artist         string
	Author         string
	Description    string
	Genre          []string
	Status         int32
	ThumbnailURL   string
	DateAdded      int64
	Viewer         int32
	Chapters       []ProtoBackupChapter
	Categories     []int64
	Favorite       bool
	ChapterFlags   int32
	History        []ProtoBackupHistory
	UpdateStrategy int32
}

type ProtoBackupSource struct {
	Name     string
	SourceID int64
}

// DecodeProtoBackup decodes raw protobuf bytes into ProtoBackup structure.
func DecodeProtoBackup(b []byte) (*ProtoBackup, error) {
	pb := &ProtoBackup{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		b = b[n:]

		switch num {
		case 1: // backupManga
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			b = b[n:]
			manga, err := decodeBackupManga(v)
			if err != nil {
				return nil, err
			}
			pb.BackupManga = append(pb.BackupManga, manga)

		case 2: // backupCategories
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			b = b[n:]
			cat, err := decodeBackupCategory(v)
			if err != nil {
				return nil, err
			}
			pb.BackupCategories = append(pb.BackupCategories, cat)

		case 101: // backupSources
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			b = b[n:]
			src, err := decodeBackupSource(v)
			if err != nil {
				return nil, err
			}
			pb.BackupSources = append(pb.BackupSources, src)

		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return pb, nil
}

func decodeBackupCategory(b []byte) (ProtoBackupCategory, error) {
	var cat ProtoBackupCategory
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return cat, protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return cat, protowire.ParseError(n)
			}
			b = b[n:]
			cat.Name = v
		case 2:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return cat, protowire.ParseError(n)
			}
			b = b[n:]
			cat.Order = int64(v)
		case 100:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return cat, protowire.ParseError(n)
			}
			b = b[n:]
			cat.Flags = int64(v)
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return cat, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return cat, nil
}

func decodeBackupChapter(b []byte) (ProtoBackupChapter, error) {
	var ch ProtoBackupChapter
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return ch, protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.URL = v
		case 2:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.Name = v
		case 3:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.Scanlator = v
		case 4:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.Read = protowire.DecodeBool(v)
		case 5:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.Bookmark = protowire.DecodeBool(v)
		case 6:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.LastPageRead = int64(v)
		case 7:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.DateFetch = int64(v)
		case 8:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.DateUpload = int64(v)
		case 9:
			v, n := protowire.ConsumeFixed32(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.ChapterNumber = math.Float32frombits(v)
		case 10:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
			ch.SourceOrder = int64(v)
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return ch, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return ch, nil
}

func decodeBackupHistory(b []byte) (ProtoBackupHistory, error) {
	var h ProtoBackupHistory
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return h, protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return h, protowire.ParseError(n)
			}
			b = b[n:]
			h.URL = v
		case 2:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return h, protowire.ParseError(n)
			}
			b = b[n:]
			h.LastRead = int64(v)
		case 3:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return h, protowire.ParseError(n)
			}
			b = b[n:]
			h.ReadDuration = int64(v)
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return h, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return h, nil
}

func decodeBackupManga(b []byte) (ProtoBackupManga, error) {
	var m ProtoBackupManga
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return m, protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.Source = int64(v)
		case 2:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.URL = v
		case 3:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.Title = v
		case 4:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.Artist = v
		case 5:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.Author = v
		case 6:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.Description = v
		case 7:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.Genre = append(m.Genre, v)
		case 8:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.Status = int32(v)
		case 9:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.ThumbnailURL = v
		case 13:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.DateAdded = int64(v)
		case 14:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.Viewer = int32(v)
		case 16:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			ch, err := decodeBackupChapter(v)
			if err == nil {
				m.Chapters = append(m.Chapters, ch)
			}
		case 17:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeBytes(b)
				if n < 0 {
					return m, protowire.ParseError(n)
				}
				b = b[n:]
				for len(v) > 0 {
					catID, n := protowire.ConsumeVarint(v)
					if n < 0 {
						break
					}
					v = v[n:]
					m.Categories = append(m.Categories, int64(catID))
				}
			} else {
				v, n := protowire.ConsumeVarint(b)
				if n < 0 {
					return m, protowire.ParseError(n)
				}
				b = b[n:]
				m.Categories = append(m.Categories, int64(v))
			}
		case 100:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.Favorite = protowire.DecodeBool(v)
		case 101:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			m.ChapterFlags = int32(v)
		case 104:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
			h, err := decodeBackupHistory(v)
			if err == nil {
				m.History = append(m.History, h)
			}
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return m, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return m, nil
}

func decodeBackupSource(b []byte) (ProtoBackupSource, error) {
	var src ProtoBackupSource
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return src, protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, n := protowire.ConsumeString(b)
			if n < 0 {
				return src, protowire.ParseError(n)
			}
			b = b[n:]
			src.Name = v
		case 2:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return src, protowire.ParseError(n)
			}
			b = b[n:]
			src.SourceID = int64(v)
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return src, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return src, nil
}

// EncodeProtoBackup encodes ProtoBackup structure into raw protobuf bytes.
func EncodeProtoBackup(pb *ProtoBackup) []byte {
	var b []byte

	// 1. Encode backupManga (tag 1)
	for _, m := range pb.BackupManga {
		mb := encodeBackupManga(&m)
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, mb)
	}

	// 2. Encode backupCategories (tag 2)
	for _, cat := range pb.BackupCategories {
		cb := encodeBackupCategory(&cat)
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, cb)
	}

	// 3. Encode backupSources (tag 101)
	for _, src := range pb.BackupSources {
		sb := encodeBackupSource(&src)
		b = protowire.AppendTag(b, 101, protowire.BytesType)
		b = protowire.AppendBytes(b, sb)
	}

	return b
}

func encodeBackupCategory(cat *ProtoBackupCategory) []byte {
	var b []byte
	if cat.Name != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, cat.Name)
	}
	if cat.Order != 0 {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(cat.Order))
	}
	if cat.Flags != 0 {
		b = protowire.AppendTag(b, 100, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(cat.Flags))
	}
	return b
}

func encodeBackupChapter(ch *ProtoBackupChapter) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, ch.URL)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, ch.Name)
	if ch.Scanlator != "" {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendString(b, ch.Scanlator)
	}
	if ch.Read {
		b = protowire.AppendTag(b, 4, protowire.VarintType)
		b = appendBool(b, ch.Read)
	}
	if ch.Bookmark {
		b = protowire.AppendTag(b, 5, protowire.VarintType)
		b = appendBool(b, ch.Bookmark)
	}
	if ch.LastPageRead > 0 {
		b = protowire.AppendTag(b, 6, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(ch.LastPageRead))
	}
	if ch.DateFetch > 0 {
		b = protowire.AppendTag(b, 7, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(ch.DateFetch))
	}
	if ch.DateUpload > 0 {
		b = protowire.AppendTag(b, 8, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(ch.DateUpload))
	}
	if ch.ChapterNumber != 0 {
		b = protowire.AppendTag(b, 9, protowire.Fixed32Type)
		b = protowire.AppendFixed32(b, math.Float32bits(ch.ChapterNumber))
	}
	if ch.SourceOrder > 0 {
		b = protowire.AppendTag(b, 10, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(ch.SourceOrder))
	}
	return b
}

func encodeBackupHistory(h *ProtoBackupHistory) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, h.URL)
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(h.LastRead))
	if h.ReadDuration > 0 {
		b = protowire.AppendTag(b, 3, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(h.ReadDuration))
	}
	return b
}

func encodeBackupManga(m *ProtoBackupManga) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(m.Source))

	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, m.URL)

	if m.Title != "" {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendString(b, m.Title)
	}
	if m.Artist != "" {
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendString(b, m.Artist)
	}
	if m.Author != "" {
		b = protowire.AppendTag(b, 5, protowire.BytesType)
		b = protowire.AppendString(b, m.Author)
	}
	if m.Description != "" {
		b = protowire.AppendTag(b, 6, protowire.BytesType)
		b = protowire.AppendString(b, m.Description)
	}
	for _, g := range m.Genre {
		b = protowire.AppendTag(b, 7, protowire.BytesType)
		b = protowire.AppendString(b, g)
	}
	if m.Status != 0 {
		b = protowire.AppendTag(b, 8, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(m.Status))
	}
	if m.ThumbnailURL != "" {
		b = protowire.AppendTag(b, 9, protowire.BytesType)
		b = protowire.AppendString(b, m.ThumbnailURL)
	}
	if m.DateAdded > 0 {
		b = protowire.AppendTag(b, 13, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(m.DateAdded))
	}
	if m.Viewer != 0 {
		b = protowire.AppendTag(b, 14, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(m.Viewer))
	}
	for _, ch := range m.Chapters {
		chb := encodeBackupChapter(&ch)
		b = protowire.AppendTag(b, 16, protowire.BytesType)
		b = protowire.AppendBytes(b, chb)
	}
	for _, catID := range m.Categories {
		b = protowire.AppendTag(b, 17, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(catID))
	}
	if m.Favorite {
		b = protowire.AppendTag(b, 100, protowire.VarintType)
		b = appendBool(b, m.Favorite)
	}
	if m.ChapterFlags != 0 {
		b = protowire.AppendTag(b, 101, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(m.ChapterFlags))
	}
	for _, h := range m.History {
		hb := encodeBackupHistory(&h)
		b = protowire.AppendTag(b, 104, protowire.BytesType)
		b = protowire.AppendBytes(b, hb)
	}
	return b
}

func encodeBackupSource(src *ProtoBackupSource) []byte {
	var b []byte
	if src.Name != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, src.Name)
	}
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(src.SourceID))
	return b
}

// RestoreFromProtoBackup restores all categories, mangas, chapters, and history from a ProtoBackup into the database.
func RestoreFromProtoBackup(ctx context.Context, pb *ProtoBackup) error {
	if database.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 0. Ensure default user 1 exists
	_, _ = tx.Exec(ctx, `
		INSERT INTO suwayomi.useraccount (id, username, password) 
		VALUES (1, 'admin', '') 
		ON CONFLICT (id) DO NOTHING
	`)

	// 0b. Restore Sources from backup metadata
	for _, src := range pb.BackupSources {
		if src.SourceID <= 0 {
			continue
		}
		var exists bool
		_ = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM suwayomi.source WHERE id = $1)", src.SourceID).Scan(&exists)
		if !exists {
			var extID int
			errExt := tx.QueryRow(ctx, `
				INSERT INTO suwayomi.extension (apk_name, icon_url, name, pkg_name, version_name, version_code, lang, is_installed, has_update, is_obsolete, class_name, store_index_url, extension_lib, apk_url, content_warning)
				VALUES ('imported.apk', '', $1, $2, '1.0', 1, 'all', true, false, false, '', '', '1.0', '', 0)
				ON CONFLICT (pkg_name) DO UPDATE SET is_installed = true
				RETURNING id
			`, src.Name, fmt.Sprintf("imported.source.%d", src.SourceID)).Scan(&extID)
			if errExt == nil && extID > 0 {
				_, _ = tx.Exec(ctx, `
					INSERT INTO suwayomi.source (id, name, lang, extension, content_warning)
					VALUES ($1, $2, 'all', $3, 0)
					ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name
				`, src.SourceID, src.Name, extID)
			}
		}
	}

	// 1. Restore Categories and build name/order mapping
	catIDMap := make(map[int64]int64) // map backup index/order to DB category ID
	for idx, cat := range pb.BackupCategories {
		var catID int64
		err := tx.QueryRow(ctx, "SELECT id FROM suwayomi.category WHERE name = $1 AND user_id = 1", cat.Name).Scan(&catID)
		if err != nil || catID == 0 {
			err = tx.QueryRow(ctx, `
				INSERT INTO suwayomi.category (name, sort_order, is_default, include_in_update, include_in_download, version, uid, last_modified_at, is_syncing, flags, user_id, is_default_category)
				VALUES ($1, $2, false, -1, -1, 1, 0, $3, false, $4, 1, false)
				RETURNING id
			`, cat.Name, cat.Order, time.Now().UnixMilli(), cat.Flags).Scan(&catID)
			if err != nil {
				log.Printf("[BACKUP] Warning creating category %q: %v\n", cat.Name, err)
			}
		}
		if catID > 0 {
			catIDMap[int64(idx)] = catID
			catIDMap[cat.Order] = catID
		}
	}

	// 2. Restore Mangas and Chapters
	for _, m := range pb.BackupManga {
		var mangaID int64
		err := tx.QueryRow(ctx, "SELECT id FROM suwayomi.manga WHERE source = $1 AND url = $2", m.Source, m.URL).Scan(&mangaID)
		if err != nil || mangaID == 0 {
			genreStr := strings.Join(m.Genre, ", ")
			err = tx.QueryRow(ctx, `
				INSERT INTO suwayomi.manga (source, url, title, artist, author, description, genre, status, thumbnail_url, initialized, memo)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, '{}')
				RETURNING id
			`, m.Source, m.URL, m.Title, m.Artist, m.Author, m.Description, genreStr, m.Status, m.ThumbnailURL).Scan(&mangaID)
			if err != nil {
				log.Printf("[BACKUP] Error inserting manga %q: %v\n", m.Title, err)
				continue
			}
		}

		// Update or insert into suwayomi.mangauser
		dateAdded := m.DateAdded
		if dateAdded == 0 {
			dateAdded = time.Now().UnixMilli()
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO suwayomi.mangauser (manga, user_id, in_library, in_library_at, viewer, chapter_flags, version, last_modified_at)
			VALUES ($1, 1, true, $2, $3, $4, 1, $5)
			ON CONFLICT (manga, user_id) 
			DO UPDATE SET 
				in_library = true,
				in_library_at = CASE WHEN suwayomi.mangauser.in_library_at = 0 THEN EXCLUDED.in_library_at ELSE suwayomi.mangauser.in_library_at END,
				viewer = EXCLUDED.viewer,
				chapter_flags = EXCLUDED.chapter_flags,
				last_modified_at = EXCLUDED.last_modified_at
		`, mangaID, dateAdded, m.Viewer, m.ChapterFlags, time.Now().UnixMilli())
		if err != nil {
			log.Printf("[BACKUP] Error updating mangauser for manga %d: %v\n", mangaID, err)
		}

		// Link manga to categories
		for _, catIdx := range m.Categories {
			if targetCatID, ok := catIDMap[catIdx]; ok {
				_, _ = tx.Exec(ctx, `
					INSERT INTO suwayomi.categorymanga (category, manga, user_id) 
					VALUES ($1, $2, 1) ON CONFLICT (user_id, category, manga) DO NOTHING
				`, targetCatID, mangaID)
			}
		}

		// Restore chapters
		for _, ch := range m.Chapters {
			var chapterID int64
			err := tx.QueryRow(ctx, "SELECT id FROM suwayomi.chapter WHERE manga = $1 AND url = $2", mangaID, ch.URL).Scan(&chapterID)
			if err != nil || chapterID == 0 {
				err = tx.QueryRow(ctx, `
					INSERT INTO suwayomi.chapter (manga, url, name, scanlator, date_upload, chapter_number, source_order, fetched_at, memo)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '{}')
					ON CONFLICT (url, manga) DO UPDATE SET name = EXCLUDED.name
					RETURNING id
				`, mangaID, ch.URL, ch.Name, ch.Scanlator, ch.DateUpload, ch.ChapterNumber, ch.SourceOrder, ch.DateFetch).Scan(&chapterID)
				if err != nil {
					log.Printf("[BACKUP] Error inserting chapter %q: %v\n", ch.Name, err)
					continue
				}
			}

			// Upsert chapteruser progress
			_, err = tx.Exec(ctx, `
				INSERT INTO suwayomi.chapteruser (chapter, user_id, read, bookmark, last_page_read, last_read_at, version, last_modified_at)
				VALUES ($1, 1, $2, $3, $4, 0, 1, $5)
				ON CONFLICT (chapter, user_id) 
				DO UPDATE SET 
					read = EXCLUDED.read,
					bookmark = EXCLUDED.bookmark,
					last_page_read = EXCLUDED.last_page_read,
					last_modified_at = EXCLUDED.last_modified_at
			`, chapterID, ch.Read, ch.Bookmark, ch.LastPageRead, time.Now().UnixMilli())
			if err != nil {
				log.Printf("[BACKUP] Error updating chapteruser for chapter %d: %v\n", chapterID, err)
			}
		}

		// Restore reading history timestamps
		for _, h := range m.History {
			if h.LastRead > 0 {
				_, _ = tx.Exec(ctx, `
					UPDATE suwayomi.chapteruser 
					SET last_read_at = $1 
					WHERE chapter = (SELECT id FROM suwayomi.chapter WHERE manga = $2 AND url = $3 LIMIT 1)
					  AND user_id = 1
				`, h.LastRead, mangaID, h.URL)
			}
		}
	}

	return tx.Commit(ctx)
}

// ExportProtoBackup builds a ProtoBackup from the current PostgreSQL database.
func ExportProtoBackup(ctx context.Context) (*ProtoBackup, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	pb := &ProtoBackup{}

	// 1. Export Categories
	rowsCats, err := database.DB.Query(ctx, `
		SELECT id, name, sort_order, coalesce(flags, 0) 
		FROM suwayomi.category 
		WHERE id != 0 AND user_id = 1 
		ORDER BY sort_order ASC
	`)
	catDBToIdx := make(map[int64]int64)
	if err == nil {
		defer rowsCats.Close()
		var idx int64 = 0
		for rowsCats.Next() {
			var id, order, flags int64
			var name string
			if err := rowsCats.Scan(&id, &name, &order, &flags); err == nil {
				pb.BackupCategories = append(pb.BackupCategories, ProtoBackupCategory{
					Name:  name,
					Order: order,
					Flags: flags,
				})
				catDBToIdx[id] = idx
				idx++
			}
		}
	}

	// 2. Export Library Mangas
	rowsMangas, err := database.DB.Query(ctx, `
		SELECT m.id, m.source, m.url, m.title, coalesce(m.artist, ''), coalesce(m.author, ''), 
		       coalesce(m.description, ''), coalesce(m.genre, ''), m.status, coalesce(m.thumbnail_url, ''), 
		       coalesce(mu.in_library_at, 0), coalesce(mu.viewer, 0), coalesce(mu.chapter_flags, 0)
		FROM suwayomi.manga m 
		JOIN suwayomi.mangauser mu ON m.id = mu.manga AND mu.user_id = 1
		WHERE mu.in_library = true
		ORDER BY m.id ASC
	`)
	if err == nil {
		defer rowsMangas.Close()
		for rowsMangas.Next() {
			var id, source, inLibAt int64
			var viewer, chapterFlags int32
			var url, title, artist, author, desc, genreStr, thumb string
			var status int32
			if err := rowsMangas.Scan(&id, &source, &url, &title, &artist, &author, &desc, &genreStr, &status, &thumb, &inLibAt, &viewer, &chapterFlags); err == nil {
				var genres []string
				if genreStr != "" {
					for _, g := range strings.Split(genreStr, ",") {
						if trimmed := strings.TrimSpace(g); trimmed != "" {
							genres = append(genres, trimmed)
						}
					}
				}

				m := ProtoBackupManga{
					Source:       source,
					URL:          url,
					Title:        title,
					Artist:       artist,
					Author:       author,
					Description:  desc,
					Genre:        genres,
					Status:       status,
					ThumbnailURL: thumb,
					DateAdded:    inLibAt,
					Viewer:       viewer,
					Favorite:     true,
					ChapterFlags: chapterFlags,
				}

				// Fetch categories for this manga
				rowsMCat, err := database.DB.Query(ctx, "SELECT category FROM suwayomi.categorymanga WHERE manga = $1 AND user_id = 1", id)
				if err == nil {
					for rowsMCat.Next() {
						var cID int64
						if err := rowsMCat.Scan(&cID); err == nil {
							if catIdx, ok := catDBToIdx[cID]; ok {
								m.Categories = append(m.Categories, catIdx)
							}
						}
					}
					rowsMCat.Close()
				}

				// Fetch chapters for this manga
				rowsCh, err := database.DB.Query(ctx, `
					SELECT c.url, c.name, coalesce(c.scanlator, ''), coalesce(cu.read, false), coalesce(cu.bookmark, false), 
					       coalesce(cu.last_page_read, 0), coalesce(c.fetched_at, 0), coalesce(c.date_upload, 0), 
					       c.chapter_number, c.source_order, coalesce(cu.last_read_at, 0)
					FROM suwayomi.chapter c
					LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter AND cu.user_id = 1
					WHERE c.manga = $1 
					ORDER BY c.source_order ASC
				`, id)
				if err == nil {
					for rowsCh.Next() {
						var chURL, chName, scanlator string
						var read, bookmark bool
						var lastPage, dateFetch, dateUpload, srcOrder, lastRead int64
						var chNum float32
						if err := rowsCh.Scan(&chURL, &chName, &scanlator, &read, &bookmark, &lastPage, &dateFetch, &dateUpload, &chNum, &srcOrder, &lastRead); err == nil {
							m.Chapters = append(m.Chapters, ProtoBackupChapter{
								URL:           chURL,
								Name:          chName,
								Scanlator:     scanlator,
								Read:          read,
								Bookmark:      bookmark,
								LastPageRead:  lastPage,
								DateFetch:     dateFetch,
								DateUpload:    dateUpload,
								ChapterNumber: chNum,
								SourceOrder:   srcOrder,
							})

							if lastRead > 0 {
								m.History = append(m.History, ProtoBackupHistory{
									URL:      chURL,
									LastRead: lastRead,
								})
							}
						}
					}
					rowsCh.Close()
				}

				pb.BackupManga = append(pb.BackupManga, m)
			}
		}
	}

	// 3. Export Sources used by library manga
	rowsSrc, err := database.DB.Query(ctx, `
		SELECT DISTINCT s.id, coalesce(s.name, '') 
		FROM suwayomi.source s
		JOIN suwayomi.manga m ON s.id = m.source
		JOIN suwayomi.mangauser mu ON m.id = mu.manga AND mu.user_id = 1
		WHERE mu.in_library = true
	`)
	if err == nil {
		for rowsSrc.Next() {
			var sid int64
			var sname string
			if err := rowsSrc.Scan(&sid, &sname); err == nil {
				pb.BackupSources = append(pb.BackupSources, ProtoBackupSource{
					Name:     sname,
					SourceID: sid,
				})
			}
		}
		rowsSrc.Close()
	}

	return pb, nil
}

// ExportTachibk creates a gzipped protobuf (.tachibk / .proto.gz) archive of the library.
func ExportTachibk(ctx context.Context) ([]byte, error) {
	pb, err := ExportProtoBackup(ctx)
	if err != nil {
		return nil, err
	}

	protoBytes := EncodeProtoBackup(pb)

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(protoBytes); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
