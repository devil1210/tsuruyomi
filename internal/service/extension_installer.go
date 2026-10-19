package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/extensions"
	"tsuruyomi/internal/sources"
)

// RepoIndexEntry represents an extension item inside an external repository index.json.
type RepoIndexEntry struct {
	Name        string                      `json:"name"`
	Pkg         string                      `json:"pkg"`
	Apk         string                      `json:"apk"`
	Lang        string                      `json:"lang"`
	Code        int64                       `json:"code"`
	Version     string                      `json:"version"`
	Nsfw        int                         `json:"nsfw"`
	Icon        string                      `json:"icon"`
	Sources     []extensions.ManifestSource `json:"sources"`
}

var (
	repoCacheMu   sync.RWMutex
	repoCache     = make(map[string][]RepoIndexEntry)
	repoCacheTime = make(map[string]time.Time)
)

var extensionPackageAliases = map[string]string{
	"eu.kanade.tachiyomi.extension.es.cerberuseries":        "eu.kanade.tachiyomi.extension.es.cerberusseries",
	"eu.kanade.tachiyomi.extension.es.cerberusseries":       "eu.kanade.tachiyomi.extension.es.cerberuseries",
	"eu.kanade.tachiyomi.extension.es.dragontranslation":    "eu.kanade.tachiyomi.extension.es.dragontranslationorg",
	"eu.kanade.tachiyomi.extension.es.dragontranslationorg": "eu.kanade.tachiyomi.extension.es.dragontranslation",
	"eu.kanade.tachiyomi.extension.es.mangaoni":             "eu.kanade.tachiyomi.extension.es.mangamx",
	"eu.kanade.tachiyomi.extension.es.mangamx":              "eu.kanade.tachiyomi.extension.es.mangaoni",
	"eu.kanade.tachiyomi.extension.es.lectormanga":          "eu.kanade.tachiyomi.extension.es.lectormangalat",
	"eu.kanade.tachiyomi.extension.es.lectormangalat":       "eu.kanade.tachiyomi.extension.es.lectormanga",
}

// FetchRepoIndex retrieves the index.json catalog from a repository URL with caching.
func FetchRepoIndex(ctx context.Context, repoURL string) ([]RepoIndexEntry, error) {
	cleanURL := strings.TrimSpace(repoURL)
	if cleanURL == "" {
		return nil, fmt.Errorf("empty repo URL")
	}

	repoCacheMu.RLock()
	cached, ok := repoCache[cleanURL]
	lastTime := repoCacheTime[cleanURL]
	repoCacheMu.RUnlock()

	if ok && time.Since(lastTime) < 5*time.Minute {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cleanURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Tsuruyomi/1.0 (https://github.com/Suwayomi)")
	req.Header.Set("Accept", "application/json")

	resp, err := sources.DefaultClient.Do(req)
	if err != nil {
		if ok {
			return cached, nil // fallback to cached on temporary network glitch
		}
		return nil, fmt.Errorf("fetching repo index %s: %w", cleanURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if ok {
			return cached, nil
		}
		return nil, fmt.Errorf("repo index %s returned HTTP %d", cleanURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var entries []RepoIndexEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("decoding repo index %s: %w", cleanURL, err)
	}

	repoCacheMu.Lock()
	repoCache[cleanURL] = entries
	repoCacheTime[cleanURL] = time.Now()
	repoCacheMu.Unlock()

	return entries, nil
}

// SyncExtensionRepositories fetches catalogs from all configured stores and updates suwayomi.extension.
func SyncExtensionRepositories(ctx context.Context, cfg *config.Config) error {
	if database.DB == nil {
		return nil
	}

	stores, err := database.GetExtensionStores(ctx)
	if err != nil || len(stores) == 0 {
		return nil
	}

	for _, store := range stores {
		// Only attempt JSON repositories (e.g. index.json)
		if !strings.HasSuffix(store.IndexURL, ".json") {
			continue
		}

		entries, err := FetchRepoIndex(ctx, store.IndexURL)
		if err != nil {
			log.Printf("[EXTENSIONS] Warning: sync failed for store %s: %v", store.Name, err)
			continue
		}

		for _, entry := range entries {
			if entry.Pkg == "" {
				continue
			}

			// Check if already in DB
			var isInstalled bool
			var installedCode int64
			err := database.DB.QueryRow(ctx, "SELECT is_installed, version_code FROM suwayomi.extension WHERE pkg_name = $1", entry.Pkg).Scan(&isInstalled, &installedCode)
			if err != nil {
				// Not in DB -> Insert as available (uninstalled)
				_, _ = database.DB.Exec(ctx, `
					INSERT INTO suwayomi.extension (apk_name, icon_url, name, pkg_name, version_name, version_code, lang, is_installed, has_update, is_obsolete, class_name, store_index_url, extension_lib, apk_url, content_warning)
					VALUES ($1, $2, $3, $4, $5, $6, $7, false, false, false, '', $8, '1.5', '', $9)
					ON CONFLICT (pkg_name) DO NOTHING
				`, entry.Pkg+".apk", entry.Icon, entry.Name, entry.Pkg, entry.Version, entry.Code, entry.Lang, store.IndexURL, entry.Nsfw)
			} else if isInstalled && entry.Code > installedCode {
				// Installed but newer version exists in repo -> flag has_update
				_, _ = database.DB.Exec(ctx, `
					UPDATE suwayomi.extension SET has_update = true WHERE pkg_name = $1
				`, entry.Pkg)
			}
		}
	}

	return nil
}

// InstallExtension downloads, registers in database, and activates an extension dynamically.
func InstallExtension(ctx context.Context, cfg *config.Config, pkgName string) error {
	if database.DB == nil {
		return fmt.Errorf("database not connected")
	}

	cleanPkg := strings.TrimSpace(pkgName)
	if cleanPkg == "" {
		return fmt.Errorf("empty package name")
	}

	// 1. Locate extension entry in configured extension stores
	stores, _ := database.GetExtensionStores(ctx)

	var foundEntry *RepoIndexEntry
	var foundStoreURL string

	for _, store := range stores {
		if !strings.HasSuffix(store.IndexURL, ".json") {
			continue
		}
		entries, err := FetchRepoIndex(ctx, store.IndexURL)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Pkg == cleanPkg {
				foundEntry = &e
				foundStoreURL = store.IndexURL
				break
			}
		}
		if foundEntry != nil {
			break
		}
	}

	// If not found in remote repo, check if it's already seeded natively
	if foundEntry == nil {
		for _, seed := range database.NativeExtensionsSeed {
			if seed.PkgName == cleanPkg {
				var sourcesList []extensions.ManifestSource
				for _, s := range seed.Sources {
					sourcesList = append(sourcesList, extensions.ManifestSource{
						ID:             s.ID,
						Name:           s.Name,
						DisplayName:    s.DisplayName,
						Lang:           s.Lang,
						ContentWarning: s.ContentWarning,
					})
				}
				foundEntry = &RepoIndexEntry{
					Name:    seed.Name,
					Pkg:     seed.PkgName,
					Lang:    seed.Lang,
					Code:    100,
					Version: "1.0-go",
					Icon:    seed.IconURL,
					Sources: sourcesList,
				}
				foundStoreURL = database.DefaultExtensionStoreURL
				break
			}
		}
	}

	if foundEntry == nil {
		return fmt.Errorf("extension package %s not found in any repository", cleanPkg)
	}

	// 2. Prepare extensions directory in root dir
	var rootDir string
	if cfg != nil && cfg.Data.RootDir != "" {
		rootDir = cfg.Data.RootDir
	} else {
		rootDir = "data"
	}
	extDir := filepath.Join(rootDir, "extensions")
	_ = os.MkdirAll(extDir, 0755)

	// 3. If the extension has an external file (e.g. .json or .js), download it
	if foundEntry.Apk != "" && (strings.HasSuffix(foundEntry.Apk, ".json") || strings.HasSuffix(foundEntry.Apk, ".js")) {
		apkURL := foundEntry.Apk
		if !strings.HasPrefix(apkURL, "http://") && !strings.HasPrefix(apkURL, "https://") {
			// Resolve relative to repo index URL
			base := foundStoreURL
			if idx := strings.LastIndex(base, "/"); idx != -1 {
				apkURL = base[:idx+1] + foundEntry.Apk
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apkURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Tsuruyomi/1.0")
			resp, err := sources.DefaultClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				data, errRead := io.ReadAll(resp.Body)
				if errRead == nil {
					extFile := filepath.Join(extDir, filepath.Base(foundEntry.Apk))
					_ = os.WriteFile(extFile, data, 0644)
					// If it's a JSON manifest, merge sources if needed
					if strings.HasSuffix(foundEntry.Apk, ".json") {
						var subManifest extensions.Manifest
						if errU := json.Unmarshal(data, &subManifest); errU == nil && len(subManifest.Sources) > 0 {
							foundEntry.Sources = subManifest.Sources
						}
					}
				}
			}
		}
	}

	// 4. Save declarative manifest to data/extensions/<pkgName>.json
	manifest := extensions.Manifest{
		Name:        foundEntry.Name,
		PkgName:     foundEntry.Pkg,
		Lang:        foundEntry.Lang,
		Version:     foundEntry.Version,
		VersionCode: foundEntry.Code,
		Icon:        foundEntry.Icon,
		Sources:     foundEntry.Sources,
	}
	mBytes, _ := json.MarshalIndent(manifest, "", "  ")
	manifestPath := filepath.Join(extDir, cleanPkg+".json")
	_ = os.WriteFile(manifestPath, mBytes, 0644)

	// 5. Update database suwayomi.extension
	var extID int
	err := database.DB.QueryRow(ctx, `
		INSERT INTO suwayomi.extension (apk_name, icon_url, name, pkg_name, version_name, version_code, lang, is_installed, has_update, is_obsolete, class_name, store_index_url, extension_lib, apk_url, content_warning)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true, false, false, '', $8, '1.5', '', $9)
		ON CONFLICT (pkg_name) DO UPDATE SET
			is_installed = true,
			has_update = false,
			name = EXCLUDED.name,
			version_name = EXCLUDED.version_name,
			version_code = EXCLUDED.version_code,
			lang = EXCLUDED.lang,
			icon_url = EXCLUDED.icon_url
		RETURNING id
	`, cleanPkg+".apk", foundEntry.Icon, foundEntry.Name, cleanPkg, foundEntry.Version, foundEntry.Code, foundEntry.Lang, foundStoreURL, foundEntry.Nsfw).Scan(&extID)
	if err != nil {
		return fmt.Errorf("updating suwayomi.extension for %s: %w", cleanPkg, err)
	}

	// 6. Insert / Update each source in suwayomi.source
	for _, s := range foundEntry.Sources {
		if s.ID == 0 {
			continue
		}
		_, err := database.DB.Exec(ctx, `
			INSERT INTO suwayomi.source (id, name, lang, extension, content_warning)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				lang = EXCLUDED.lang,
				extension = EXCLUDED.extension,
				content_warning = EXCLUDED.content_warning
		`, s.ID, s.Name, s.Lang, extID, s.ContentWarning)
		if err != nil {
			log.Printf("[EXTENSIONS] Warning inserting source %s (%d): %v", s.Name, s.ID, err)
		}

		// Also register in KnownSourcesByID lookup table
		dispName := s.DisplayName
		if dispName == "" {
			dispName = s.Name
		}
		database.KnownSourcesByID[s.ID] = database.KnownSourceInfo{
			Name:        s.Name,
			DisplayName: dispName,
			Lang:        s.Lang,
			PkgName:     cleanPkg,
			ExtName:     foundEntry.Name,
			IconURL:     foundEntry.Icon,
		}
	}

	// 7. Register sources into memory runtime
	extensions.RegisterManifestSources(&manifest)

	alias := extensionPackageAliases[cleanPkg]
	if alias == "" {
		alias = cleanPkg
	}
	_, _ = database.DB.Exec(ctx, "UPDATE suwayomi.extension SET is_installed = true WHERE pkg_name = $1 OR pkg_name = $2", cleanPkg, alias)

	// Restore any compiled native sources associated with this extension
	rowsRestore, errRestore := database.DB.Query(ctx, `
		SELECT s.id FROM suwayomi.source s
		JOIN suwayomi.extension e ON s.extension = e.id
		WHERE e.pkg_name = $1 OR e.pkg_name = $2
	`, cleanPkg, alias)
	if errRestore == nil {
		defer rowsRestore.Close()
		for rowsRestore.Next() {
			var sid int64
			if err := rowsRestore.Scan(&sid); err == nil {
				sources.RestoreNative(sid)
			}
		}
	}

	log.Printf("[EXTENSIONS] Successfully installed and activated extension %s (%s) with %d sources", foundEntry.Name, cleanPkg, len(foundEntry.Sources))

	return nil
}

// UninstallExtension marks an extension uninstalled and removes runtime handlers.
func UninstallExtension(ctx context.Context, cfg *config.Config, pkgName string) error {
	if database.DB == nil {
		return fmt.Errorf("database not connected")
	}

	cleanPkg := strings.TrimSpace(pkgName)
	if cleanPkg == "" {
		return fmt.Errorf("empty package name")
	}

	var rootDir string
	if cfg != nil && cfg.Data.RootDir != "" {
		rootDir = cfg.Data.RootDir
	} else {
		rootDir = "data"
	}
	manifestPath := filepath.Join(rootDir, "extensions", cleanPkg+".json")

	// Unregister from runtime memory
	if m, err := extensions.LoadManifestFile(manifestPath); err == nil {
		extensions.UnregisterManifestSources(m)
	}

	// Remove manifest file
	_ = os.Remove(manifestPath)
	_ = os.Remove(filepath.Join(rootDir, "extensions", cleanPkg+".js"))

	alias := extensionPackageAliases[cleanPkg]
	if alias == "" {
		alias = cleanPkg
	}

	// Mark as uninstalled in DB for both package name and any alias
	_, err := database.DB.Exec(ctx, "UPDATE suwayomi.extension SET is_installed = false WHERE pkg_name = $1 OR pkg_name = $2", cleanPkg, alias)
	if err != nil {
		return fmt.Errorf("marking %s uninstalled in DB: %w", cleanPkg, err)
	}

	// Unregister any active sources associated with this extension
	rows, errQuery := database.DB.Query(ctx, `
		SELECT s.id FROM suwayomi.source s
		JOIN suwayomi.extension e ON s.extension = e.id
		WHERE e.pkg_name = $1 OR e.pkg_name = $2
	`, cleanPkg, alias)
	if errQuery == nil {
		defer rows.Close()
		for rows.Next() {
			var sid int64
			if err := rows.Scan(&sid); err == nil {
				sources.Unregister(sid)
			}
		}
	}

	log.Printf("[EXTENSIONS] Successfully uninstalled extension %s (alias: %s)", cleanPkg, alias)
	return nil
}

// UpdateExtension updates an existing installed extension.
func UpdateExtension(ctx context.Context, cfg *config.Config, pkgName string) error {
	return InstallExtension(ctx, cfg, pkgName)
}

// InstallExtensionsForSources looks up extension package names for the given source IDs across native seeds and configured stores, and installs them.
func InstallExtensionsForSources(ctx context.Context, cfg *config.Config, sourceIDs []int64) ([]string, []string) {
	if len(sourceIDs) == 0 {
		return nil, nil
	}

	pkgSet := make(map[string]bool)

	// 1. Check in KnownSourcesByID
	for _, srcID := range sourceIDs {
		if known, ok := database.KnownSourcesByID[srcID]; ok && known.PkgName != "" {
			pkgSet[known.PkgName] = true
		}
	}

	// 2. Check in configured extension stores
	stores, _ := database.GetExtensionStores(ctx)
	for _, store := range stores {
		if !strings.HasSuffix(store.IndexURL, ".json") {
			continue
		}
		entries, err := FetchRepoIndex(ctx, store.IndexURL)
		if err != nil {
			continue
		}
		for _, e := range entries {
			for _, s := range e.Sources {
				for _, srcID := range sourceIDs {
					if s.ID == srcID && e.Pkg != "" {
						pkgSet[e.Pkg] = true
					}
				}
			}
		}
	}

	var installed []string
	var failed []string
	for pkg := range pkgSet {
		if err := InstallExtension(ctx, cfg, pkg); err != nil {
			log.Printf("[EXTENSIONS] Failed to auto-install %s: %v", pkg, err)
			failed = append(failed, pkg)
		} else {
			log.Printf("[EXTENSIONS] Successfully auto-installed %s", pkg)
			installed = append(installed, pkg)
		}
	}

	return installed, failed
}
