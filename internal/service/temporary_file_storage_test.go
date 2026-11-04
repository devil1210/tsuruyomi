package service

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTemporaryFileStorage_SaveAndRetrieve(t *testing.T) {
	tfs := NewTemporaryFileStorage()
	content := []byte("test backup content 12345")
	filename := "test_backup.tachibk"

	token, err := tfs.SaveFile(filename, content)
	if err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}

	if !strings.HasSuffix(token, ".tachibk") {
		t.Fatalf("expected token to preserve extension .tachibk, got %q", token)
	}

	stored, ok := tfs.RetrieveFile(token)
	if !ok || stored == nil {
		t.Fatalf("RetrieveFile failed for token %q", token)
	}

	if stored.Filename != filename {
		t.Fatalf("expected filename %q, got %q", filename, stored.Filename)
	}

	readData, err := os.ReadFile(stored.Path)
	if err != nil {
		t.Fatalf("failed reading temp file: %v", err)
	}

	if !bytes.Equal(readData, content) {
		t.Fatalf("file content mismatch: expected %q, got %q", string(content), string(readData))
	}

	// Test nonexistent token
	if _, ok := tfs.RetrieveFile("nonexistent-token.tachibk"); ok {
		t.Fatalf("expected false for nonexistent token")
	}

	// Test CleanExpired
	tfs.CleanExpired(0 * time.Second)
	if _, ok := tfs.RetrieveFile(token); ok {
		t.Fatalf("expected token to be expired and removed")
	}
}
