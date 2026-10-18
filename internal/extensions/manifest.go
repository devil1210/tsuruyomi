package extensions

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tsuruyomi/internal/sources"
)

// Manifest represents the structure of an extension manifest.
type Manifest struct {
	Name        string           `json:"name"`
	PkgName     string           `json:"pkgName"`
	Lang        string           `json:"lang"`
	Version     string           `json:"version"`
	VersionCode int64            `json:"versionCode"`
	Icon        string           `json:"icon"`
	Sources     []ManifestSource `json:"sources"`
}

// ManifestSource defines a manga source declared inside an extension manifest.
type ManifestSource struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Lang           string `json:"lang"`
	BaseURL        string `json:"baseUrl"`
	Engine         string `json:"engine"` // "madara", "mangathemesia", "js"
	Script         string `json:"script"` // optional script filename or code
	ContentWarning int    `json:"contentWarning"`
}

// UnmarshalJSON handles both string and numeric IDs gracefully.
func (s *ManifestSource) UnmarshalJSON(data []byte) error {
	type Alias ManifestSource
	aux := struct {
		IDVal interface{} `json:"id"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch v := aux.IDVal.(type) {
	case float64:
		s.ID = int64(v)
	case string:
		s.ID, _ = strconv.ParseInt(v, 10, 64)
	case json.Number:
		s.ID, _ = v.Int64()
	}
	return nil
}

// LoadManifestFile loads a JSON extension manifest from disk.
func LoadManifestFile(filePath string) (*Manifest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", filePath, err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", filePath, err)
	}

	return &m, nil
}

// RegisterManifestSources dynamically activates all sources defined in the manifest.
func RegisterManifestSources(m *Manifest) {
	for _, s := range m.Sources {
		if s.ID == 0 {
			continue
		}
		switch strings.ToLower(s.Engine) {
		case "madara":
			sources.Register(s.ID, sources.NewMadaraHandler(s.Name, s.BaseURL))
			log.Printf("[EXTENSIONS] Registered dynamic Madara source %s (%d) from %s", s.Name, s.ID, m.PkgName)
		case "mangathemesia":
			sources.Register(s.ID, sources.NewMangaThemesiaHandler(s.Name, s.BaseURL))
			log.Printf("[EXTENSIONS] Registered dynamic MangaThemesia source %s (%d) from %s", s.Name, s.ID, m.PkgName)
		}
	}
}

// UnregisterManifestSources removes all sources defined in the manifest from memory.
func UnregisterManifestSources(m *Manifest) {
	for _, s := range m.Sources {
		if s.ID != 0 {
			sources.Unregister(s.ID)
			log.Printf("[EXTENSIONS] Unregistered dynamic source %s (%d)", s.Name, s.ID)
		}
	}
}

// LoadInstalledExtensions scans the root extensions directory and loads all manifests and JS scrapers.
func LoadInstalledExtensions(rootDir string) error {
	extDir := filepath.Join(rootDir, "extensions")
	if _, err := os.Stat(extDir); os.IsNotExist(err) {
		_ = os.MkdirAll(extDir, 0755)
		return nil
	}

	entries, err := os.ReadDir(extDir)
	if err != nil {
		return fmt.Errorf("reading extensions directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(extDir, entry.Name())
		if strings.HasSuffix(entry.Name(), ".json") {
			m, err := LoadManifestFile(path)
			if err != nil {
				log.Printf("[EXTENSIONS] Warning: failed to load manifest %s: %v", entry.Name(), err)
				continue
			}
			RegisterManifestSources(m)
			log.Printf("[EXTENSIONS] Successfully loaded dynamic extension manifest: %s", m.PkgName)
		} else if strings.HasSuffix(entry.Name(), ".js") {
			pkgName := strings.TrimSuffix(entry.Name(), ".js")
			if err := GlobalEngine.LoadExtension(path, pkgName); err != nil {
				log.Printf("[EXTENSIONS] Warning: failed to load JS extension %s: %v", entry.Name(), err)
			} else {
				log.Printf("[EXTENSIONS] Successfully loaded JS extension: %s", pkgName)
			}
		}
	}

	return nil
}
