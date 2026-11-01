package util

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// CleanOrphanedPageCache removes temporary page cache files older than maxAge,
// or completely if maxAge <= 0.
func CleanOrphanedPageCache(rootDir string, maxAge time.Duration) {
	cacheDir := filepath.Join(rootDir, "cache", "pages")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return
	}
	if maxAge <= 0 {
		log.Printf("[CACHE] Purging all page cache in %s...", cacheDir)
		_ = os.RemoveAll(cacheDir)
		_ = os.MkdirAll(cacheDir, 0755)
		return
	}
	now := time.Now()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	for _, mangaEntry := range entries {
		if !mangaEntry.IsDir() {
			continue
		}
		mangaPath := filepath.Join(cacheDir, mangaEntry.Name())
		chEntries, err := os.ReadDir(mangaPath)
		if err != nil {
			continue
		}
		for _, chEntry := range chEntries {
			chPath := filepath.Join(mangaPath, chEntry.Name())
			info, err := chEntry.Info()
			if err == nil && now.Sub(info.ModTime()) > maxAge {
				_ = os.RemoveAll(chPath)
			}
		}
		if remaining, err := os.ReadDir(mangaPath); err == nil && len(remaining) == 0 {
			_ = os.Remove(mangaPath)
		}
	}
}

// EnsureSufficientDiskSpace checks available disk space and purges page cache if free space is below minBytes.
func EnsureSufficientDiskSpace(rootDir string, minBytes uint64) {
	if rootDir == "" {
		return
	}
	free := DiskFreeBytes(rootDir)
	if free < minBytes {
		log.Printf("[DISK] Low disk space detected (%d MB free, threshold: %d MB). Pruning page cache...", free/(1024*1024), minBytes/(1024*1024))
		CleanOrphanedPageCache(rootDir, 0)
	}
}
