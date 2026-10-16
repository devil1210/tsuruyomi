package util

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strconv"

	"tsuruyomi/internal/config"
)

// GetMangaThumbnailPath searches for a downloaded cover image file matching the manga ID.
// Replicates ThumbnailFileProvider.getFilePath by looking for the file directly.
func GetMangaThumbnailPath(cfg *config.Config, mangaId int) (string, string, error) {
	thumbnailDir := filepath.Join(cfg.Data.RootDir, "downloads", "thumbnails")
	mangaIdStr := strconv.Itoa(mangaId)

	// Check common extensions directly to avoid slow directory listings (ReadDir)
	extensions := []string{".jpg", ".jpeg", ".png", ".webp"}
	for _, ext := range extensions {
		fullPath := filepath.Join(thumbnailDir, mangaIdStr+ext)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			mimeType := mime.TypeByExtension(ext)
			if mimeType == "" {
				mimeType = "image/jpeg"
			}
			return fullPath, mimeType, nil
		}
	}

	return "", "", fmt.Errorf("no thumbnail found for manga ID: %d", mangaId)
}
