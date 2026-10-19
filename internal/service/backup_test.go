package service

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"testing"
)

func TestProtoBackup_EncodeDecodeRoundTrip(t *testing.T) {
	original := &ProtoBackup{
		BackupCategories: []ProtoBackupCategory{
			{Name: "Reading", Order: 0, Flags: 1},
			{Name: "Plan to Read", Order: 1, Flags: 0},
		},
		BackupManga: []ProtoBackupManga{
			{
				Source:       123456789,
				URL:          "/manga/one-piece",
				Title:        "One Piece",
				Artist:       "Eiichiro Oda",
				Author:       "Eiichiro Oda",
				Description:  "Pirate adventure",
				Genre:        []string{"Action", "Adventure", "Comedy"},
				Status:       1,
				ThumbnailURL: "https://example.com/cover.jpg",
				DateAdded:    1700000000000,
				Viewer:       1,
				Favorite:     true,
				ChapterFlags: 0,
				Categories:   []int64{0},
				Chapters: []ProtoBackupChapter{
					{
						URL:           "/manga/one-piece/1",
						Name:          "Chapter 1 - Romance Dawn",
						Scanlator:     "Official",
						Read:          true,
						Bookmark:      true,
						LastPageRead:  54,
						DateFetch:     1700000001000,
						DateUpload:    1700000000500,
						ChapterNumber: 1.0,
						SourceOrder:   0,
					},
				},
				History: []ProtoBackupHistory{
					{
						URL:          "/manga/one-piece/1",
						LastRead:     1700000050000,
						ReadDuration: 300,
					},
				},
			},
		},
		BackupSources: []ProtoBackupSource{
			{Name: "MangaDex", SourceID: 123456789},
		},
	}

	encoded := EncodeProtoBackup(original)
	if len(encoded) == 0 {
		t.Fatalf("EncodeProtoBackup produced empty bytes")
	}

	// Decompress and check round-trip
	decoded, err := DecodeProtoBackup(encoded)
	if err != nil {
		t.Fatalf("DecodeProtoBackup failed: %v", err)
	}

	if len(decoded.BackupCategories) != len(original.BackupCategories) {
		t.Errorf("Categories count mismatch: expected %d, got %d", len(original.BackupCategories), len(decoded.BackupCategories))
	} else {
		if decoded.BackupCategories[0].Name != original.BackupCategories[0].Name {
			t.Errorf("Category 0 name mismatch: expected %s, got %s", original.BackupCategories[0].Name, decoded.BackupCategories[0].Name)
		}
	}

	if len(decoded.BackupManga) != len(original.BackupManga) {
		t.Fatalf("Manga count mismatch: expected %d, got %d", len(original.BackupManga), len(decoded.BackupManga))
	}

	m := decoded.BackupManga[0]
	if m.Title != "One Piece" || m.Source != 123456789 || m.URL != "/manga/one-piece" {
		t.Errorf("Manga fields mismatch: %+v", m)
	}

	if len(m.Chapters) != 1 {
		t.Fatalf("Chapters count mismatch: expected 1, got %d", len(m.Chapters))
	}
	ch := m.Chapters[0]
	if ch.Name != "Chapter 1 - Romance Dawn" || !ch.Read || !ch.Bookmark || ch.LastPageRead != 54 {
		t.Errorf("Chapter fields mismatch: %+v", ch)
	}

	if len(m.History) != 1 || m.History[0].LastRead != 1700000050000 {
		t.Errorf("History mismatch: %+v", m.History)
	}

	if len(decoded.BackupSources) != 1 || decoded.BackupSources[0].Name != "MangaDex" {
		t.Errorf("Sources mismatch: %+v", decoded.BackupSources)
	}
}

func TestGzipDecompression(t *testing.T) {
	originalData := []byte("hello protobuf test payload")
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(originalData)
	_ = zw.Close()

	compressed := buf.Bytes()
	if len(compressed) < 2 || compressed[0] != 0x1f || compressed[1] != 0x8b {
		t.Fatalf("Compressed data missing gzip magic bytes")
	}

	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip reader creation failed: %v", err)
	}
	decompressed, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompression failed: %v", err)
	}
	if string(decompressed) != string(originalData) {
		t.Errorf("Decompressed content mismatch: got %q, want %q", string(decompressed), string(originalData))
	}
}

func TestDecodeExportedBackup(t *testing.T) {
	data, err := os.ReadFile("../../test_exported.tachibk")
	if err != nil {
		t.Skip("test_exported.tachibk not found, skipping file decode test")
	}

	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Failed to open gzip reader: %v", err)
	}
	defer zr.Close()

	protoBytes, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("Failed to read decompressed bytes: %v", err)
	}

	pb, err := DecodeProtoBackup(protoBytes)
	if err != nil {
		t.Fatalf("Failed to decode proto backup: %v", err)
	}

	t.Logf("Successfully decoded exported backup!")
	t.Logf("Categories: %d", len(pb.BackupCategories))
	t.Logf("Mangas: %d", len(pb.BackupManga))
	t.Logf("Sources: %d", len(pb.BackupSources))
	totalChapters := 0
	for _, m := range pb.BackupManga {
		totalChapters += len(m.Chapters)
	}
	t.Logf("Total Chapters in backup: %d", totalChapters)
}

func TestValidateBackup_EmptyAndProtobuf(t *testing.T) {
	// Empty data should return empty missing sources without error
	res, err := ValidateBackup(t.Context(), []byte{})
	if err != nil {
		t.Fatalf("ValidateBackup empty data error: %v", err)
	}
	if len(res.MissingSources) != 0 {
		t.Errorf("Expected 0 missing sources for empty data, got %d", len(res.MissingSources))
	}

	// Create backup with a source
	original := &ProtoBackup{
		BackupManga: []ProtoBackupManga{
			{Source: 99999999, Title: "Test Manga", URL: "/test"},
		},
		BackupSources: []ProtoBackupSource{
			{Name: "Uninstalled Source", SourceID: 99999999},
		},
	}
	encoded := EncodeProtoBackup(original)

	// Since DB is nil in unit tests, ValidateBackup returns empty without panic
	res, err = ValidateBackup(t.Context(), encoded)
	if err != nil {
		t.Fatalf("ValidateBackup encoded data error: %v", err)
	}
	if res == nil {
		t.Fatalf("Expected non-nil result from ValidateBackup")
	}
}

