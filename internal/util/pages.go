package util

import (
	"archive/zip"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tsuruyomi/internal/config"
)

// GetChapterPath returns the local path to a downloaded chapter folder.
func GetChapterPath(cfg *config.Config, sourceName, mangaTitle, chapterName, scanlator string) string {
	sanSourceName := SanitizeFilename(sourceName)
	sanMangaTitle := SanitizeFilename(mangaTitle)

	var rawChapterName string
	if scanlator != "" {
		rawChapterName = scanlator + "_" + chapterName
	} else {
		rawChapterName = chapterName
	}
	sanChapterName := SanitizeFilename(rawChapterName)

	return filepath.Join(cfg.Data.RootDir, "downloads", "mangas", sanSourceName, sanMangaTitle, sanChapterName)
}

// ServePage retrieves a page image from local storage (directory or CBZ archive).
func ServePage(cfg *config.Config, sourceName, mangaTitle, chapterName, scanlator string, index int) ([]byte, string, error) {
	basePath := GetChapterPath(cfg, sourceName, mangaTitle, chapterName, scanlator)
	cbzPath := basePath + ".cbz"

	// 1. Try serving from CBZ
	if fileExists(cbzPath) {
		return servePageFromCBZ(cbzPath, index)
	}

	// 2. Try serving from Folder
	if dirExists(basePath) {
		return servePageFromFolder(basePath, index)
	}

	return nil, "", fmt.Errorf("downloaded chapter files not found at: %s", basePath)
}

// servePageFromFolder lists the folder, sorts, and serves index-th file.
func servePageFromFolder(dirPath string, index int) ([]byte, string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, "", err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.ToLower(entry.Name()) != "comicinfo.xml" {
			files = append(files, entry.Name())
		}
	}

	if len(files) == 0 {
		return nil, "", fmt.Errorf("no files found in chapter directory")
	}

	sort.Strings(files)

	if index < 0 || index >= len(files) {
		return nil, "", fmt.Errorf("page index %d out of bounds (0-%d)", index, len(files)-1)
	}

	targetPath := filepath.Join(dirPath, files[index])
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, "", err
	}

	ext := filepath.Ext(files[index])
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "image/jpeg" // Fallback
	}

	return data, mimeType, nil
}

// servePageFromCBZ reads zip archive, sorts entries, and serves index-th entry bytes.
func servePageFromCBZ(zipPath string, index int) ([]byte, string, error) {
	files, err := GetIndexedFiles(zipPath)
	if err != nil {
		return nil, "", err
	}

	if index < 0 || index >= len(files) {
		return nil, "", fmt.Errorf("page index %d out of bounds (0-%d)", index, len(files)-1)
	}

	targetName := files[index]

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, "", err
	}
	defer r.Close()

	var targetFile *zip.File
	for _, f := range r.File {
		if f.Name == targetName {
			targetFile = f
			break
		}
	}

	if targetFile == nil {
		return nil, "", fmt.Errorf("target file %s not found in zip archive", targetName)
	}

	rc, err := targetFile.Open()
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", err
	}

	ext := filepath.Ext(targetFile.Name)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "image/jpeg" // Fallback
	}

	return data, mimeType, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
