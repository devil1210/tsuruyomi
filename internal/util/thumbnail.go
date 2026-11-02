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
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() && info.Size() > 0 {
			mimeType := mime.TypeByExtension(ext)
			if f, err := os.Open(fullPath); err == nil {
				buf := make([]byte, 32)
				if n, rErr := f.Read(buf); rErr == nil && n > 0 {
					mimeType = DetectImageType(buf[:n])
				}
				_ = f.Close()
			}
			if mimeType == "" {
				mimeType = "image/jpeg"
			}
			return fullPath, mimeType, nil
		} else if err == nil && info.Size() == 0 {
			// Remove zero-byte stub files left by failed previous downloads
			_ = os.Remove(fullPath)
		}
	}

	return "", "", fmt.Errorf("no thumbnail found for manga ID: %d", mangaId)
}
