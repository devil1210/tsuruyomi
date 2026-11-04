package service

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StoredTempFile represents a temporary file mapped to an opaque secure token.
type StoredTempFile struct {
	Path      string
	Filename  string
	CreatedAt time.Time
}

// TemporaryFileStorage manages ephemeral files (backups, exports) with random tokens.
type TemporaryFileStorage struct {
	mu      sync.RWMutex
	baseDir string
	files   map[string]StoredTempFile
}

// GlobalTempFileStorage is the singleton instance for ephemeral files.
var GlobalTempFileStorage = NewTemporaryFileStorage()

// NewTemporaryFileStorage initializes storage in system temp directory.
func NewTemporaryFileStorage() *TemporaryFileStorage {
	tmpDir := filepath.Join(os.TempDir(), "tsuruyomi_temp_files")
	_ = os.MkdirAll(tmpDir, 0755)

	tfs := &TemporaryFileStorage{
		baseDir: tmpDir,
		files:   make(map[string]StoredTempFile),
	}

	// Periodic cleanup routine for files older than 24h
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			tfs.CleanExpired(24 * time.Hour)
		}
	}()

	return tfs
}

// SaveFile writes the given data to a temporary file associated with an opaque token.
func (tfs *TemporaryFileStorage) SaveFile(filename string, data []byte) (string, error) {
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	token := generateToken(filename)
	filePath := filepath.Join(tfs.baseDir, token)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("writing temp file: %w", err)
	}

	tfs.files[token] = StoredTempFile{
		Path:      filePath,
		Filename:  filename,
		CreatedAt: time.Now(),
	}

	return token, nil
}

// RetrieveFile retrieves the file metadata by its token.
func (tfs *TemporaryFileStorage) RetrieveFile(token string) (*StoredTempFile, bool) {
	tfs.mu.RLock()
	defer tfs.mu.RUnlock()

	stored, ok := tfs.files[token]
	if !ok {
		// Fallback check on disk directly
		safeToken := filepath.Base(token)
		diskPath := filepath.Join(tfs.baseDir, safeToken)
		if info, err := os.Stat(diskPath); err == nil && !info.IsDir() {
			return &StoredTempFile{
				Path:      diskPath,
				Filename:  safeToken,
				CreatedAt: info.ModTime(),
			}, true
		}
		return nil, false
	}

	if _, err := os.Stat(stored.Path); err != nil {
		return nil, false
	}

	return &stored, true
}

// CleanExpired removes stored files older than maxAge.
func (tfs *TemporaryFileStorage) CleanExpired(maxAge time.Duration) {
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	now := time.Now()
	for token, stored := range tfs.files {
		if now.Sub(stored.CreatedAt) > maxAge {
			_ = os.Remove(stored.Path)
			delete(tfs.files, token)
		}
	}
}

func generateToken(filename string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := base64.RawURLEncoding.EncodeToString(b)
	ext := filepath.Ext(filename)
	if ext != "" {
		token = token + ext
	}
	return token
}
