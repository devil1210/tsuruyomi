package util

import (
	"archive/zip"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type zipCacheEntry struct {
	modTime time.Time
	size    int64
	files   []string
}

var zipCache sync.Map // key: string (zipPath), value: zipCacheEntry

// GetIndexedFiles retrieves a sorted list of non-directory files within the CBZ/ZIP, skipping metadata.
// It leverages an in-memory cache, checking the modification time and file size to ensure freshness.
func GetIndexedFiles(zipPath string) ([]string, error) {
	info, err := os.Stat(zipPath)
	if err != nil {
		return nil, err
	}

	if val, ok := zipCache.Load(zipPath); ok {
		entry := val.(zipCacheEntry)
		if entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
			return entry.files, nil
		}
	}

	// Not in cache, or cache is stale. Open and list zip files.
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var files []string
	for _, file := range r.File {
		nameLower := strings.ToLower(file.Name)
		// Skip directories, OS metadata, and ComicInfo.xml metadata
		if !file.FileInfo().IsDir() && nameLower != "comicinfo.xml" && !strings.HasPrefix(nameLower, "__macosx") {
			files = append(files, file.Name)
		}
	}

	// Sort files alphabetically to ensure pages are ordered correctly
	sort.Strings(files)

	// Cache the result
	zipCache.Store(zipPath, zipCacheEntry{
		modTime: info.ModTime(),
		size:    info.Size(),
		files:   files,
	})

	return files, nil
}
