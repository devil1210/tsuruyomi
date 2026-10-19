package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
	"tsuruyomi/internal/service"
	"tsuruyomi/internal/sources"
	"tsuruyomi/internal/util"
)

// NativeGoExtensionPackages lists extension package names implemented natively in Go
var NativeGoExtensionPackages = map[string]bool{
	"eu.kanade.tachiyomi.extension.es.mangamx":              true, // MangaOni
	"eu.kanade.tachiyomi.extension.es.mangaoni":             true, // MangaOni (alias)
	"eu.kanade.tachiyomi.extension.es.lectormangalat":        true, // LectorManga.lat
	"eu.kanade.tachiyomi.extension.es.lectormanga":           true, // LectorManga (alias)
	"eu.kanade.tachiyomi.extension.all.mangadex":             true, // MangaDex
	"eu.kanade.tachiyomi.extension.es.uchuujinprojects":      true, // Uchuujin Projects
	"eu.kanade.tachiyomi.extension.es.shadowmanga":           true, // Shadow Manga
	"eu.kanade.tachiyomi.extension.es.manhwaweb":             true, // ManhwaWeb
	"eu.kanade.tachiyomi.extension.es.submanhwa":             true, // Submanhwa
	"eu.kanade.tachiyomi.extension.es.mangasnosekai":         true, // Mangas No Sekai
	"eu.kanade.tachiyomi.extension.es.ravenmanga":            true, // RavenManga
	"eu.kanade.tachiyomi.extension.es.leermanhwas":           true, // LeerManhwas
	"eu.kanade.tachiyomi.extension.es.bokugentranslation":     true, // BokugenTranslation
	"eu.kanade.tachiyomi.extension.es.apollcomics":           true, // ApollComics
	"eu.kanade.tachiyomi.extension.es.dragontranslation":     true, // DragonTranslation (alias)
	"eu.kanade.tachiyomi.extension.es.dragontranslationorg":  true, // DragonTranslation
	"eu.kanade.tachiyomi.extension.es.cerberuseries":         true, // Cerberus Series (alias)
	"eu.kanade.tachiyomi.extension.es.cerberusseries":        true, // Cerberus Series
	"eu.kanade.tachiyomi.extension.es.manhwalatino":          true, // Manhwa-Latino
	"eu.kanade.tachiyomi.extension.es.hadesnofansub":         true, // Hades no Fansub
	"eu.kanade.tachiyomi.extension.es.begatranslation":        true, // Bega Translation
	"eu.kanade.tachiyomi.extension.es.lmtoonline":           true, // LMTO Online
	"eu.kanade.tachiyomi.extension.es.ninemanga":            true, // NineManga (ES)
	"eu.kanade.tachiyomi.extension.en.ninemanga":            true, // NineManga (EN)
	"eu.kanade.tachiyomi.extension.es.manhwasnet":           true, // Manhwas.net
	"eu.kanade.tachiyomi.extension.all.batoto":              true, // Bato.to
	"eu.kanade.tachiyomi.extension.es.jobsibe":              true, // Jobsibe
	"eu.kanade.tachiyomi.extension.es.zevep":                true, // Zevep
	"eu.kanade.tachiyomi.source.local":                      true, // Local source
}

// ExtensionPackageAliases maps alias extension packages to their canonical counterparts
var ExtensionPackageAliases = map[string]string{
	"eu.kanade.tachiyomi.extension.es.cerberuseries":     "eu.kanade.tachiyomi.extension.es.cerberusseries",
	"eu.kanade.tachiyomi.extension.es.dragontranslation": "eu.kanade.tachiyomi.extension.es.dragontranslationorg",
	"eu.kanade.tachiyomi.extension.es.mangaoni":          "eu.kanade.tachiyomi.extension.es.mangamx",
	"eu.kanade.tachiyomi.extension.es.lectormanga":       "eu.kanade.tachiyomi.extension.es.lectormangalat",
}

func getCanonicalPkg(pkgName string) string {
	if canon, ok := ExtensionPackageAliases[pkgName]; ok {
		return canon
	}
	return pkgName
}

// NativeExtInfo holds basic display metadata for native Go extensions
type NativeExtInfo struct {
	Name string
	Lang string
}

var NativeGoExtensionInfo = map[string]NativeExtInfo{
	"eu.kanade.tachiyomi.extension.es.mangamx":             {Name: "MangaOni", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.mangaoni":            {Name: "MangaOni", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.lectormangalat":       {Name: "LectorManga.lat", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.lectormanga":          {Name: "LectorManga", Lang: "es"},
	"eu.kanade.tachiyomi.extension.all.mangadex":            {Name: "MangaDex", Lang: "all"},
	"eu.kanade.tachiyomi.extension.es.uchuujinprojects":     {Name: "Uchuujin Projects", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.shadowmanga":          {Name: "Shadow Manga", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.manhwaweb":            {Name: "ManhwaWeb", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.submanhwa":            {Name: "Submanhwa", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.mangasnosekai":        {Name: "Mangas No Sekai", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.ravenmanga":           {Name: "RavenManga", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.leermanhwas":          {Name: "LeerManhwas", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.bokugentranslation":    {Name: "BokugenTranslation", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.apollcomics":          {Name: "ApollComics", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.dragontranslation":    {Name: "DragonTranslation", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.dragontranslationorg": {Name: "DragonTranslation", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.cerberuseries":        {Name: "Cerberus Series", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.cerberusseries":       {Name: "Cerberus Series", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.manhwalatino":         {Name: "Manhwa-Latino", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.hadesnofansub":        {Name: "Hades no Fansub", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.begatranslation":       {Name: "Bega Translation", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.lmtoonline":           {Name: "LMTO Online", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.ninemanga":            {Name: "NineManga", Lang: "es"},
	"eu.kanade.tachiyomi.extension.en.ninemanga":            {Name: "NineManga (EN)", Lang: "en"},
	"eu.kanade.tachiyomi.extension.es.manhwasnet":           {Name: "Manhwas.net", Lang: "es"},
	"eu.kanade.tachiyomi.extension.all.batoto":              {Name: "Bato.to", Lang: "all"},
	"eu.kanade.tachiyomi.extension.es.jobsibe":              {Name: "Jobsibe", Lang: "es"},
	"eu.kanade.tachiyomi.extension.es.zevep":                {Name: "Zevep", Lang: "es"},
	"eu.kanade.tachiyomi.source.local":                     {Name: "Local Source", Lang: "all"},
}

// DefaultExtensionStoreURL is the canonical repository URL for Tsuruyomi native Go extensions
const DefaultExtensionStoreURL = "https://raw.githubusercontent.com/devil1210/tsuruyomi-extensions/main/index.json"
const DefaultExtensionStoreName = "Tsuruyomi Native Go Extensions"

func getExtensionMap(ctx context.Context, pkgName string) map[string]interface{} {
	canon := getCanonicalPkg(pkgName)
	name := pkgName
	lang := "all"
	cw := "SAFE"
	isInstalled := false
	hasUpdate := false
	isObsolete := false
	versionName := "1.0-go"
	versionCode := int64(100)
	iconURL := "/api/v1/extension/icon/" + pkgName
	storeIndexURL := DefaultExtensionStoreURL
	storeName := DefaultExtensionStoreName

	if pkgName == "eu.kanade.tachiyomi.source.local" {
		isInstalled = true
		name = "Local Source"
	}

	if database.DB != nil {
		var isInst, hasUpd, isObs bool
		var n, l, vName, icon, storeURL string
		var vCode int64
		var cwInt int
		err := database.DB.QueryRow(ctx, `
			SELECT name, lang, version_name, version_code, is_installed, has_update, is_obsolete, COALESCE(icon_url, ''), COALESCE(store_index_url, $2), COALESCE(content_warning, 0)
			FROM suwayomi.extension WHERE pkg_name = $1 OR pkg_name = $3
			ORDER BY is_installed DESC LIMIT 1
		`, pkgName, DefaultExtensionStoreURL, canon).Scan(&n, &l, &vName, &vCode, &isInst, &hasUpd, &isObs, &icon, &storeURL, &cwInt)
		if err == nil {
			name = n
			isInstalled = isInst
			hasUpdate = hasUpd
			isObsolete = isObs
			if vName != "" {
				versionName = vName
			}
			if vCode > 0 {
				versionCode = vCode
			}
			if l != "" {
				lang = l
			}
			if storeURL != "" {
				storeIndexURL = storeURL
			}
			if cwInt > 0 {
				cw = "NSFW"
			}
		}
	}

	if name == pkgName {
		if info, ok := NativeGoExtensionInfo[pkgName]; ok {
			name = info.Name
			lang = info.Lang
			if strings.Contains(name, "+18") || strings.Contains(name, "NSFW") {
				cw = "NSFW"
			}
		} else if info, ok := NativeGoExtensionInfo[canon]; ok {
			name = info.Name
			lang = info.Lang
		}
	}

	return map[string]interface{}{
		"pkgName":         pkgName,
		"name":            name,
		"lang":            lang,
		"versionCodeLong": versionCode,
		"versionName":     versionName,
		"iconUrl":         iconURL,
		"storeIndexUrl":   storeIndexURL,
		"contentWarning":  cw,
		"isInstalled":     isInstalled,
		"isObsolete":      isObsolete,
		"hasUpdate":       hasUpdate,
		"extensionLib":    "1.5",
		"extensionStore": map[string]interface{}{
			"indexUrl":   storeIndexURL,
			"name":       storeName,
			"__typename": "ExtensionStoreType",
		},
		"__typename": "ExtensionType",
	}
}

func getExtensionNodes(ctx context.Context, filterStoreUrl string) []map[string]interface{} {
	nodes := []map[string]interface{}{}
	seenPkg := make(map[string]bool)
	seenNameLang := make(map[string]bool)

	// 1. If database has extension records, query them
	if database.DB != nil {
		rows, err := database.DB.Query(ctx, `
			SELECT name, pkg_name, version_name, version_code, lang, COALESCE(icon_url, ''), is_installed, has_update, is_obsolete, COALESCE(store_index_url, $1), COALESCE(content_warning, 0), COALESCE(extension_lib, '1.5')
			FROM suwayomi.extension
		`, DefaultExtensionStoreURL)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name, pkgName, versionName, lang, iconUrl, storeIndexUrl, extLib string
				var versionCode int64
				var isInstalled, hasUpdate, isObsolete bool
				var cwInt int
				err := rows.Scan(&name, &pkgName, &versionName, &versionCode, &lang, &iconUrl, &isInstalled, &hasUpdate, &isObsolete, &storeIndexUrl, &cwInt, &extLib)
				if err == nil {
					if strings.HasPrefix(pkgName, "imported.source.") || name == "Imported Source" {
						continue
					}
					canon := getCanonicalPkg(pkgName)
					nameKey := strings.ToLower(strings.TrimSpace(name)) + "::" + strings.ToLower(strings.TrimSpace(lang))

					// Deduplicate if canonical package or same name+lang already processed
					if seenPkg[pkgName] || seenPkg[canon] || seenNameLang[nameKey] {
						continue
					}

					if pkgName == "eu.kanade.tachiyomi.source.local" {
						isInstalled = true
						hasUpdate = false
						isObsolete = false
						versionName = "1.0-go (Nativa)"
					}
					seenPkg[pkgName] = true
					seenPkg[canon] = true
					seenNameLang[nameKey] = true

					if filterStoreUrl != "" && storeIndexUrl != filterStoreUrl && !NativeGoExtensionPackages[pkgName] && !NativeGoExtensionPackages[canon] {
						continue
					}

					cw := "SAFE"
					if cwInt > 0 {
						cw = "NSFW"
					}
					storeName := DefaultExtensionStoreName
					if strings.Contains(storeIndexUrl, "keiyoushi") {
						storeName = "Keiyoushi"
					} else if strings.Contains(storeIndexUrl, "suwayomi") {
						storeName = "Suwayomi Extension Repo"
					}

					nodes = append(nodes, map[string]interface{}{
						"pkgName":         pkgName,
						"name":            name,
						"lang":            lang,
						"versionCodeLong": versionCode,
						"versionName":     versionName,
						"iconUrl":         "/api/v1/extension/icon/" + pkgName,
						"storeIndexUrl":   storeIndexUrl,
						"contentWarning":  cw,
						"isInstalled":     isInstalled,
						"isObsolete":      isObsolete,
						"hasUpdate":       hasUpdate,
						"extensionLib":    extLib,
						"extensionStore": map[string]interface{}{
							"indexUrl":   storeIndexUrl,
							"name":       storeName,
							"__typename": "ExtensionStoreType",
						},
						"__typename": "ExtensionType",
					})
				}
			}
		}
	}

	// 2. Add all native Go extensions
	type extItem struct {
		pkgName string
		info    NativeExtInfo
	}
	var sortedExts []extItem
	for pkg, info := range NativeGoExtensionInfo {
		if _, isAlias := ExtensionPackageAliases[pkg]; isAlias {
			continue
		}
		sortedExts = append(sortedExts, extItem{pkgName: pkg, info: info})
	}
	sort.Slice(sortedExts, func(i, j int) bool {
		return sortedExts[i].info.Name < sortedExts[j].info.Name
	})

	for _, item := range sortedExts {
		canon := getCanonicalPkg(item.pkgName)
		nameKey := strings.ToLower(strings.TrimSpace(item.info.Name)) + "::" + strings.ToLower(strings.TrimSpace(item.info.Lang))
		if !seenPkg[item.pkgName] && !seenPkg[canon] && !seenNameLang[nameKey] {
			seenPkg[item.pkgName] = true
			seenPkg[canon] = true
			seenNameLang[nameKey] = true
			nodes = append(nodes, getExtensionMap(ctx, item.pkgName))
		}
	}

	return nodes
}

// buildServerSettingsMap merges default settings, active config, and DB persisted settings.
func buildServerSettingsMap(ctx context.Context, cfg *config.Config) map[string]interface{} {
	extRepos := []string{}
	if stores, err := database.GetExtensionStores(ctx); err == nil && len(stores) > 0 {
		for _, s := range stores {
			extRepos = append(extRepos, s.IndexURL)
		}
	} else {
		extRepos = []string{DefaultExtensionStoreURL, "https://raw.githubusercontent.com/keiyoushi/extensions/repo/index.pb"}
	}

	dbSettings, _ := database.GetAllServerSettings(ctx)
	if dbSettings == nil {
		dbSettings = make(map[string]string)
	}

	getStr := func(k string, fallback string) string {
		if v, ok := dbSettings[k]; ok {
			return v
		}
		return fallback
	}
	getBool := func(k string, fallback bool) bool {
		if v, ok := dbSettings[k]; ok {
			return strings.ToLower(v) == "true" || v == "1"
		}
		return fallback
	}
	getInt := func(k string, fallback int) int {
		if v, ok := dbSettings[k]; ok {
			var n int
			if _, err := fmt.Sscan(v, &n); err == nil {
				return n
			}
		}
		return fallback
	}
	getFloat := func(k string, fallback float64) float64 {
		if v, ok := dbSettings[k]; ok {
			var f float64
			if _, err := fmt.Sscan(v, &f); err == nil {
				return f
			}
		}
		return fallback
	}

	fsEnabled := getBool("flareSolverrEnabled", false)
	fsURL := getStr("flareSolverrUrl", "http://127.0.0.1:8191")
	fsTimeout := getInt("flareSolverrTimeout", 60)
	authMode := getStr("authMode", "NONE")
	authUsername := getStr("authUsername", "")
	authPassword := getStr("authPassword", "")
	port := getInt("port", 4567)
	ip := getStr("ip", "0.0.0.0")

	if cfg != nil {
		if _, ok := dbSettings["flareSolverrEnabled"]; !ok {
			fsEnabled = cfg.FlareSolverr.Enabled
		}
		if _, ok := dbSettings["flareSolverrUrl"]; !ok && cfg.FlareSolverr.URL != "" {
			fsURL = cfg.FlareSolverr.URL
		}
		if _, ok := dbSettings["flareSolverrTimeout"]; !ok && cfg.FlareSolverr.Timeout > 0 {
			fsTimeout = cfg.FlareSolverr.Timeout
		}
		if _, ok := dbSettings["authMode"]; !ok && cfg.Auth.Mode != "" {
			authMode = strings.ToUpper(cfg.Auth.Mode)
		}
		if _, ok := dbSettings["authUsername"]; !ok {
			authUsername = cfg.Auth.Username
		}
		if _, ok := dbSettings["authPassword"]; !ok {
			authPassword = cfg.Auth.Password
		}
		if _, ok := dbSettings["port"]; !ok && cfg.Server.Port > 0 {
			port = cfg.Server.Port
		}
		if _, ok := dbSettings["ip"]; !ok && cfg.Server.Host != "" {
			ip = cfg.Server.Host
		}
	}

	var browseLangs []string
	if bStr, ok := dbSettings["browseLanguages"]; ok && bStr != "" {
		_ = json.Unmarshal([]byte(bStr), &browseLangs)
	}
	if len(browseLangs) == 0 {
		browseLangs = []string{"all", "es", "en", "es-419"}
	}

	return map[string]interface{}{
		"ip":                              ip,
		"port":                            port,
		"socksProxyEnabled":               getBool("socksProxyEnabled", false),
		"socksProxyVersion":               getInt("socksProxyVersion", 5),
		"socksProxyHost":                  getStr("socksProxyHost", ""),
		"socksProxyPort":                  getStr("socksProxyPort", ""),
		"socksProxyUsername":              getStr("socksProxyUsername", ""),
		"socksProxyPassword":              getStr("socksProxyPassword", ""),
		"webUIFlavor":                     "WEBUI",
		"initialOpenInBrowserEnabled":     getBool("initialOpenInBrowserEnabled", false),
		"webUIInterface":                  getStr("webUIInterface", "BROWSER"),
		"electronPath":                    getStr("electronPath", ""),
		"webUIChannel":                    getStr("webUIChannel", "PREVIEW"),
		"webUIUpdateCheckInterval":        getFloat("webUIUpdateCheckInterval", 3.0),
		"downloadAsCbz":                   getBool("downloadAsCbz", false),
		"downloadsPath":                   getStr("downloadsPath", ""),
		"autoDownloadNewChapters":         getBool("autoDownloadNewChapters", false),
		"excludeEntryWithUnreadChapters":  getBool("excludeEntryWithUnreadChapters", true),
		"autoDownloadNewChaptersLimit":    getInt("autoDownloadNewChaptersLimit", 1),
		"autoDownloadIgnoreReUploads":     getBool("autoDownloadIgnoreReUploads", false),
		"downloadConversions":             []interface{}{},
		"serveConversions":                []interface{}{},
		"extensionRepos":                  extRepos,
		"browseLanguages":                 browseLangs,
		"maxSourcesInParallel":            getInt("maxSourcesInParallel", 6),
		"excludeUnreadChapters":           getBool("excludeUnreadChapters", true),
		"excludeNotStarted":               getBool("excludeNotStarted", true),
		"excludeCompleted":                getBool("excludeCompleted", true),
		"globalUpdateInterval":            getFloat("globalUpdateInterval", 12.0),
		"updateMangas":                    getBool("updateMangas", false),
		"authMode":                        authMode,
		"authUsername":                    authUsername,
		"authPassword":                    authPassword,
		"jwtAudience":                     "suwayomi-server-api",
		"jwtTokenExpiry":                  "PT5M",
		"jwtRefreshExpiry":                "PT1440H",
		"debugLogsEnabled":                getBool("debugLogsEnabled", true),
		"systemTrayEnabled":               getBool("systemTrayEnabled", false),
		"maxLogFileSize":                  getStr("maxLogFileSize", "10mb"),
		"maxLogFiles":                     getInt("maxLogFiles", 31),
		"maxLogFolderSize":                getStr("maxLogFolderSize", "100mb"),
		"backupPath":                      getStr("backupPath", ""),
		"backupTime":                      getStr("backupTime", "00:00"),
		"backupInterval":                  getInt("backupInterval", 1),
		"backupTTL":                       getInt("backupTTL", 14),
		"autoBackupIncludeCategories":     getBool("autoBackupIncludeCategories", true),
		"autoBackupIncludeChapters":       getBool("autoBackupIncludeChapters", true),
		"autoBackupIncludeClientData":     getBool("autoBackupIncludeClientData", true),
		"autoBackupIncludeHistory":        getBool("autoBackupIncludeHistory", true),
		"autoBackupIncludeManga":          getBool("autoBackupIncludeManga", true),
		"autoBackupIncludeServerSettings": getBool("autoBackupIncludeServerSettings", true),
		"autoBackupIncludeTracking":       getBool("autoBackupIncludeTracking", true),
		"localSourcePath":                 getStr("localSourcePath", ""),
		"flareSolverrEnabled":             fsEnabled,
		"flareSolverrUrl":                 fsURL,
		"flareSolverrTimeout":             fsTimeout,
		"flareSolverrSessionName":         getStr("flareSolverrSessionName", "suwayomi"),
		"flareSolverrSessionTtl":          getInt("flareSolverrSessionTtl", 15),
		"flareSolverrAsResponseFallback":  getBool("flareSolverrAsResponseFallback", true),
		"opdsUseBinaryFileSizes":          getBool("opdsUseBinaryFileSizes", false),
		"opdsItemsPerPage":                getInt("opdsItemsPerPage", 50),
		"opdsEnablePageReadProgress":      getBool("opdsEnablePageReadProgress", true),
		"opdsMarkAsReadOnDownload":        getBool("opdsMarkAsReadOnDownload", false),
		"opdsShowOnlyUnreadChapters":      getBool("opdsShowOnlyUnreadChapters", false),
		"opdsShowOnlyDownloadedChapters":  getBool("opdsShowOnlyDownloadedChapters", false),
		"opdsChapterSortOrder":            getStr("opdsChapterSortOrder", "DESC"),
		"opdsCbzMimetype":                 getStr("opdsCbzMimetype", "MODERN"),
		"opdsSkipChapterMetadataFeed":     getBool("opdsSkipChapterMetadataFeed", false),
		"koreaderSyncChecksumMethod":      getStr("koreaderSyncChecksumMethod", "BINARY"),
		"koreaderSyncStrategyBackward":    getStr("koreaderSyncStrategyBackward", "DISABLED"),
		"koreaderSyncStrategyForward":     getStr("koreaderSyncStrategyForward", "PROMPT"),
		"koreaderSyncPercentageTolerance": getFloat("koreaderSyncPercentageTolerance", 1e-15),
		"databaseType":                    "POSTGRESQL",
		"databaseUrl":                     "postgresql://localhost:5432/suwayomi",
		"databaseUsername":                "postgres",
		"databasePassword":                "",
		"useHikariConnectionPool":         true,
		"kcefEnabled":                     getBool("kcefEnabled", true),
		"syncInterval":                    getStr("syncInterval", "PT0S"),
		"syncYomiEnabled":                 getBool("syncYomiEnabled", false),
		"syncYomiHost":                    getStr("syncYomiHost", ""),
		"syncYomiApiKey":                  getStr("syncYomiApiKey", ""),
		"syncDataManga":                   getBool("syncDataManga", true),
		"syncDataChapters":                getBool("syncDataChapters", true),
		"syncDataTracking":                getBool("syncDataTracking", true),
		"syncDataHistory":                 getBool("syncDataHistory", true),
		"syncDataCategories":              getBool("syncDataCategories", true),
		"__typename":                      "SettingsType",
	}
}

// buildGraphQLDownloadStatus maps the active DownloadManager state and queue to GraphQL schema.
func buildGraphQLDownloadStatus(ctx context.Context) map[string]interface{} {
	status := service.GlobalDownloadManager.GetStatus()
	state := "STOPPED"
	if strings.EqualFold(status.Status, "Started") {
		state = "STARTED"
	}

	queue := make([]map[string]interface{}, 0, len(status.Queue))
	for _, item := range status.Queue {
		var chName string
		var chNumber float64
		var chURL, mTitle, mThumb string
		var mSourceID int64
		var mSourceName string
		var isDownloaded bool
		var pageCount int

		if database.DB != nil {
			query := `
				SELECT c.name, c.chapter_number, c.url, c.is_downloaded, COALESCE(c.page_count, -1),
				       m.title, COALESCE(m.thumbnail_url, ''), m.source, COALESCE(s.name, 'Unknown')
				FROM suwayomi.chapter c
				JOIN suwayomi.manga m ON c.manga = m.id
				LEFT JOIN suwayomi.source s ON m.source = s.id
				WHERE c.id = $1
				LIMIT 1`
			_ = database.DB.QueryRow(ctx, query, item.ChapterID).Scan(
				&chName, &chNumber, &chURL, &isDownloaded, &pageCount,
				&mTitle, &mThumb, &mSourceID, &mSourceName,
			)
		}

		itemState := strings.ToUpper(string(item.State))
		if itemState == "" {
			itemState = "QUEUED"
		}

		queue = append(queue, map[string]interface{}{
			"chapter": map[string]interface{}{
				"id":            item.ChapterID,
				"name":          chName,
				"chapterNumber": chNumber,
				"url":           chURL,
				"isDownloaded":  isDownloaded,
				"pageCount":     pageCount,
				"__typename":    "ChapterType",
			},
			"manga": map[string]interface{}{
				"id":            item.MangaID,
				"title":         mTitle,
				"thumbnailUrl":  mThumb,
				"downloadCount": 0,
				"source": map[string]interface{}{
					"id":         fmt.Sprintf("%d", mSourceID),
					"name":       mSourceName,
					"__typename": "SourceType",
				},
				"__typename": "MangaType",
			},
			"state":      itemState,
			"progress":   item.Progress,
			"tries":      item.Tries,
			"position":   item.Position,
			"__typename": "DownloadType",
		})
	}

	return map[string]interface{}{
		"state":      state,
		"queue":      queue,
		"__typename": "DownloadStatus",
	}
}

// GraphQLRequest maps incoming query parameters or POST bodies.
type GraphQLRequest struct {
	OperationName string                 `json:"operationName"`
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
}

// HandleGraphQL resolves GraphQL calls for Tachidesk-WebUI compatibility.
func HandleGraphQL(c *fiber.Ctx) error {
	cfg, _ := c.Locals("config").(*config.Config)
	webuiVersion := "r1411"
	if cfg != nil {
		webuiVersion = util.GetWebUIVersion(cfg.Data.RootDir)
	}

	var req GraphQLRequest
	if c.Method() == fiber.MethodGet {
		req.OperationName = c.Query("operationName")
		req.Query = c.Query("query")
		vars := c.Query("variables")
		if vars != "" {
			_ = json.Unmarshal([]byte(vars), &req.Variables)
		}
	} else {
		contentType := c.Get(fiber.HeaderContentType)
		if strings.Contains(contentType, "multipart/form-data") {
			ops := c.FormValue("operations")
			if ops != "" {
				_ = json.Unmarshal([]byte(ops), &req)
			}
		}

		if req.OperationName == "" && req.Query == "" {
			if len(c.Body()) > 0 {
				_ = json.Unmarshal(c.Body(), &req)
			}
			if req.OperationName == "" && req.Query == "" {
				if err := c.BodyParser(&req); err != nil {
					req.OperationName = c.Query("operationName")
					req.Query = c.Query("query")
					vars := c.Query("variables")
					if vars != "" {
						_ = json.Unmarshal([]byte(vars), &req.Variables)
					}
				}
			}
		}
	}

	if req.OperationName == "" && req.Query != "" {
		fields := strings.Fields(req.Query)
		for i, f := range fields {
			if (f == "query" || f == "mutation") && i+1 < len(fields) {
				op := fields[i+1]
				if idx := strings.IndexAny(op, "({"); idx != -1 {
					op = op[:idx]
				}
				req.OperationName = strings.TrimSpace(op)
				break
			}
		}
	}

	if req.OperationName == "" && req.Query == "" {
		return c.JSON(fiber.Map{"data": nil})
	}

	log.Printf("GraphQL Dispatcher — Operation: %q, Query: %q, Variables: %v\n", req.OperationName, req.Query, req.Variables)

	var data interface{}
	switch req.OperationName {
	case "GET_SERVER_SETTINGS":
		data = map[string]interface{}{
			"settings": buildServerSettingsMap(c.Context(), cfg),
			"koSyncStatus": map[string]interface{}{
				"serverAddress": "",
				"username":      "",
				"isLoggedIn":    false,
				"__typename":    "KoSyncStatusPayload",
			},
		}

	case "UPDATE_SERVER_SETTINGS", "setSettings":
		var settingsInput map[string]interface{}
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if inner, ok := input["settings"].(map[string]interface{}); ok {
				settingsInput = inner
			} else {
				settingsInput = input
			}
		} else if settingsRaw, ok := req.Variables["settings"].(map[string]interface{}); ok {
			settingsInput = settingsRaw
		}

		if settingsInput != nil {
			for k, v := range settingsInput {
				if k == "__typename" || k == "clientMutationId" {
					continue
				}

				if k == "extensionRepos" {
					if reposRaw, ok := v.([]interface{}); ok {
						var urls []string
						for _, r := range reposRaw {
							if u, ok := r.(string); ok && strings.TrimSpace(u) != "" {
								urls = append(urls, strings.TrimSpace(u))
							}
						}
						_ = database.SetExtensionStores(c.Context(), urls)
					}
				}

				// Persist setting to database
				var valStr string
				switch val := v.(type) {
				case string:
					valStr = val
				case bool:
					valStr = fmt.Sprintf("%t", val)
				case float64:
					if val == float64(int64(val)) {
						valStr = fmt.Sprintf("%d", int64(val))
					} else {
						valStr = fmt.Sprintf("%g", val)
					}
				case int:
					valStr = fmt.Sprintf("%d", val)
				default:
					b, _ := json.Marshal(val)
					valStr = string(b)
				}
				_ = database.SetServerSetting(c.Context(), k, valStr)

				// Synchronize in-memory config
				if cfg != nil {
					switch k {
					case "flareSolverrUrl":
						if s, ok := v.(string); ok {
							cfg.FlareSolverr.URL = util.NormalizeFlareSolverrURL(s)
						}
					case "flareSolverrEnabled":
						if b, ok := v.(bool); ok {
							cfg.FlareSolverr.Enabled = b
						}
					case "flareSolverrTimeout":
						if f, ok := v.(float64); ok {
							cfg.FlareSolverr.Timeout = int(f)
						} else if i, ok := v.(int); ok {
							cfg.FlareSolverr.Timeout = i
						}
					case "port":
						if f, ok := v.(float64); ok {
							cfg.Server.Port = int(f)
						} else if i, ok := v.(int); ok {
							cfg.Server.Port = i
						}
					case "globalUpdateInterval":
						if f, ok := v.(float64); ok {
							service.GlobalUpdateManager.SetInterval(f)
						} else if i, ok := v.(int); ok {
							service.GlobalUpdateManager.SetInterval(float64(i))
						}
					case "ip":
						if s, ok := v.(string); ok && s != "" {
							cfg.Server.Host = s
						}
					case "authMode":
						if s, ok := v.(string); ok {
							cfg.Auth.Mode = strings.ToLower(s)
						}
					case "authUsername":
						if s, ok := v.(string); ok {
							cfg.Auth.Username = s
						}
					case "authPassword":
						if s, ok := v.(string); ok {
							cfg.Auth.Password = s
						}
					}
				}
			}

			if cfg != nil {
				sources.SetConfig(cfg)
				_ = cfg.Save()
			}
		}

		data = map[string]interface{}{
			"setSettings": map[string]interface{}{
				"settings":   buildServerSettingsMap(c.Context(), cfg),
				"__typename": "SetSettingsPayload",
			},
		}

	case "RESET_SERVER_SETTINGS", "resetSettings":
		if database.DB != nil {
			_, _ = database.DB.Exec(c.Context(), "DELETE FROM suwayomi.serversetting")
		}
		data = map[string]interface{}{
			"resetSettings": map[string]interface{}{
				"settings":   buildServerSettingsMap(c.Context(), cfg),
				"__typename": "ResetSettingsPayload",
			},
		}

	case "GET_ABOUT":
		nowSec := fmt.Sprintf("%d", time.Now().Unix())
		nowMilli := fmt.Sprintf("%d", time.Now().UnixMilli())
		data = map[string]interface{}{
			"aboutServer": map[string]interface{}{
				"buildTime": nowSec,
				"buildType": "Preview",
				"discord":   "https://discord.gg/suwayomi",
				"github":    "https://github.com/Suwayomi/Suwayomi-Server",
				"name":      "Tsuruyomi",
				"version":   "1.0.0",
				"platformInfo": map[string]interface{}{
					"os": map[string]interface{}{
						"name":       "Alpine Linux",
						"build":      "v3.24",
						"version":    "3.24.1",
						"__typename": "PlatformOs",
					},
					"arch":     "x86_64",
					"headless": true,
					"jvm": map[string]interface{}{
						"javaVersion": "N/A (Go Native)",
						"vmName":      "Tsuruyomi Go Runtime",
						"vmVendor":    "Go",
						"vmVersion":   "go1.27.0",
						"__typename":  "PlatformJvm",
					},
					"__typename": "PlatformInfo",
				},
				"__typename": "AboutServerPayload",
			},
			"aboutWebUI": map[string]interface{}{
				"channel":         "PREVIEW",
				"tag":             webuiVersion,
				"updateTimestamp": nowMilli,
				"__typename":      "AboutWebUI",
			},
		}

	case "GET_WEBUI_UPDATE_STATUS":
		data = map[string]interface{}{
			"getWebUIUpdateStatus": map[string]interface{}{
				"info": map[string]interface{}{
					"channel":    "PREVIEW",
					"tag":        webuiVersion,
					"__typename": "WebUIUpdateInfo",
				},
				"progress":   0,
				"state":      "IDLE",
				"__typename": "WebUIUpdateStatus",
			},
		}

	case "GET_DOWNLOAD_STATUS":
		data = map[string]interface{}{
			"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
		}

	case "START_DOWNLOADER", "startDownloader":
		service.GlobalDownloadManager.Start()
		data = map[string]interface{}{
			"startDownloader": map[string]interface{}{
				"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
				"__typename":     "StartDownloaderPayload",
			},
		}

	case "STOP_DOWNLOADER", "stopDownloader":
		service.GlobalDownloadManager.Stop()
		data = map[string]interface{}{
			"stopDownloader": map[string]interface{}{
				"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
				"__typename":     "StopDownloaderPayload",
			},
		}

	case "CLEAR_DOWNLOADER", "clearDownloader":
		service.GlobalDownloadManager.Clear()
		data = map[string]interface{}{
			"clearDownloader": map[string]interface{}{
				"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
				"__typename":     "ClearDownloaderPayload",
			},
		}

	case "ENQUEUE_CHAPTER_DOWNLOAD", "enqueueChapterDownload":
		var chID int
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if idF, ok := input["id"].(float64); ok {
				chID = int(idF)
			} else if idF, ok := input["chapterId"].(float64); ok {
				chID = int(idF)
			}
		}
		if chID > 0 {
			_ = service.GlobalDownloadManager.Enqueue(c.Context(), []int{chID})
		}
		data = map[string]interface{}{
			"enqueueChapterDownload": map[string]interface{}{
				"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
				"__typename":     "EnqueueChapterDownloadPayload",
			},
		}

	case "ENQUEUE_CHAPTER_DOWNLOADS", "enqueueChapterDownloads":
		var chIDs []int
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if ids, ok := input["ids"].([]interface{}); ok {
				for _, idVal := range ids {
					if idF, ok := idVal.(float64); ok {
						chIDs = append(chIDs, int(idF))
					}
				}
			}
		}
		if len(chIDs) > 0 {
			_ = service.GlobalDownloadManager.Enqueue(c.Context(), chIDs)
		}
		data = map[string]interface{}{
			"enqueueChapterDownloads": map[string]interface{}{
				"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
				"__typename":     "EnqueueChapterDownloadsPayload",
			},
		}

	case "DEQUEUE_CHAPTER_DOWNLOAD", "dequeueChapterDownload":
		var chID int
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if idF, ok := input["id"].(float64); ok {
				chID = int(idF)
			} else if idF, ok := input["chapterId"].(float64); ok {
				chID = int(idF)
			}
		}
		if chID > 0 {
			service.GlobalDownloadManager.Dequeue([]int{chID})
		}
		data = map[string]interface{}{
			"dequeueChapterDownload": map[string]interface{}{
				"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
				"__typename":     "DequeueChapterDownloadPayload",
			},
		}

	case "DEQUEUE_CHAPTER_DOWNLOADS", "dequeueChapterDownloads":
		var chIDs []int
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if ids, ok := input["ids"].([]interface{}); ok {
				for _, idVal := range ids {
					if idF, ok := idVal.(float64); ok {
						chIDs = append(chIDs, int(idF))
					}
				}
			}
		}
		if len(chIDs) > 0 {
			service.GlobalDownloadManager.Dequeue(chIDs)
		}
		data = map[string]interface{}{
			"dequeueChapterDownloads": map[string]interface{}{
				"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
				"__typename":     "DequeueChapterDownloadsPayload",
			},
		}

	case "REORDER_CHAPTER_DOWNLOAD", "reorderChapterDownload":
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			chID := 0
			if idF, ok := input["chapterId"].(float64); ok {
				chID = int(idF)
			}
			to := 0
			if toF, ok := input["to"].(float64); ok {
				to = int(toF)
			}
			if chID > 0 {
				service.GlobalDownloadManager.ReorderByID(chID, to)
			}
		}
		data = map[string]interface{}{
			"reorderChapterDownload": map[string]interface{}{
				"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
				"__typename":     "ReorderChapterDownloadPayload",
			},
		}

	case "REORDER_CHAPTER_DOWNLOADS", "reorderChapterDownloads":
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if reorders, ok := input["reorders"].([]interface{}); ok {
				for _, rItem := range reorders {
					if rMap, ok := rItem.(map[string]interface{}); ok {
						chID := 0
						if idF, ok := rMap["chapterId"].(float64); ok {
							chID = int(idF)
						}
						to := 0
						if toF, ok := rMap["to"].(float64); ok {
							to = int(toF)
						}
						if chID > 0 {
							service.GlobalDownloadManager.ReorderByID(chID, to)
						}
					}
				}
			}
		}
		data = map[string]interface{}{
			"reorderChapterDownloads": map[string]interface{}{
				"downloadStatus": buildGraphQLDownloadStatus(c.Context()),
				"__typename":     "ReorderChapterDownloadsPayload",
			},
		}

	case "CLEAR_SERVER_CACHE", "clearCachedImages":
		var clearPages, clearThumbs, clearDownloadedThumbs bool = true, true, true
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if b, ok := input["clearCachedPages"].(bool); ok {
				clearPages = b
			}
			if b, ok := input["clearCachedThumbnails"].(bool); ok {
				clearThumbs = b
			}
			if b, ok := input["clearDownloadedThumbnails"].(bool); ok {
				clearDownloadedThumbs = b
			}
		}

		var countPages, countThumbs, countDownloadedThumbs int
		if cfg != nil {
			if clearPages {
				pagesDir := filepath.Join(cfg.Data.RootDir, "cache", "pages")
				if entries, err := os.ReadDir(pagesDir); err == nil {
					countPages = len(entries)
					_ = os.RemoveAll(pagesDir)
					_ = os.MkdirAll(pagesDir, 0755)
				}
			}
			if clearThumbs {
				cacheThumbDir := filepath.Join(cfg.Data.RootDir, "cache", "thumbnails")
				if entries, err := os.ReadDir(cacheThumbDir); err == nil {
					countThumbs = len(entries)
					_ = os.RemoveAll(cacheThumbDir)
					_ = os.MkdirAll(cacheThumbDir, 0755)
				}
			}
			if clearDownloadedThumbs {
				downThumbDir := filepath.Join(cfg.Data.RootDir, "downloads", "thumbnails")
				if entries, err := os.ReadDir(downThumbDir); err == nil {
					countDownloadedThumbs = len(entries)
					_ = os.RemoveAll(downThumbDir)
					_ = os.MkdirAll(downThumbDir, 0755)
				}
			}
		}

		data = map[string]interface{}{
			"clearCachedImages": map[string]interface{}{
				"cachedPages":          countPages,
				"cachedThumbnails":     countThumbs,
				"downloadedThumbnails": countDownloadedThumbs,
				"__typename":           "ClearCachedImagesPayload",
			},
		}

	case "RESET_WEBUI_UPDATE_STATUS", "resetWebUIUpdateStatus":
		data = map[string]interface{}{
			"resetWebUIUpdateStatus": map[string]interface{}{
				"getWebUIUpdateStatus": map[string]interface{}{
					"info": map[string]interface{}{
						"channel":    "PREVIEW",
						"tag":        webuiVersion,
						"__typename": "WebUIUpdateInfo",
					},
					"progress":   0,
					"state":      "IDLE",
					"__typename": "WebUIUpdateStatus",
				},
				"__typename": "ResetWebUIUpdateStatusPayload",
			},
		}

	case "WEBVIEW_CLEAR_CACHE_COOKIES", "clearCacheCookies":
		data = map[string]interface{}{
			"clearCacheCookies": map[string]interface{}{
				"success":    true,
				"__typename": "ClearCacheCookiesPayload",
			},
		}

	case "GET_EXTENSIONS_FETCH":
		// Sync configured extension repositories in background
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_ = service.SyncExtensionRepositories(ctx, cfg)
		}()

		extList := getExtensionNodes(c.Context(), "")

		dbStores, _ := database.GetExtensionStores(c.Context())
		storeNodes := []map[string]interface{}{}
		for _, s := range dbStores {
			storeNodes = append(storeNodes, map[string]interface{}{
				"id":               s.ID,
				"name":             s.Name,
				"indexUrl":         s.IndexURL,
				"isLegacy":         s.IsLegacy,
				"badgeLabel":       s.BadgeLabel,
				"signingKey":       s.SigningKey,
				"contactWebsite":   s.ContactWebsite,
				"contactDiscord":   s.ContactDiscord,
				"extensionListUrl": nil,
				"extensions": map[string]interface{}{
					"totalCount": s.TotalCount,
				},
				"__typename": "ExtensionStoreType",
			})
		}

		data = map[string]interface{}{
			"fetchExtensions": map[string]interface{}{
				"extensions":      extList,
				"extensionStores": storeNodes,
				"__typename":      "FetchExtensionsResult",
			},
		}

	case "CHECK_FOR_SERVER_UPDATES":
		data = map[string]interface{}{
			"checkForServerUpdates": []map[string]interface{}{
				{
					"channel":    "Preview",
					"tag":        "1.0.0",
					"url":        "https://github.com/Suwayomi/Suwayomi-Server",
					"__typename": "CheckForServerUpdatesPayload",
				},
			},
		}

	case "CHECK_FOR_WEBUI_UPDATE":
		data = map[string]interface{}{
			"checkForWebUIUpdate": map[string]interface{}{
				"channel":         "PREVIEW",
				"tag":             webuiVersion,
				"updateAvailable": false,
				"__typename":      "WebUIUpdateCheck",
			},
		}

	case "UPDATE_WEBUI", "updateWebUI":
		data = map[string]interface{}{
			"updateWebUI": map[string]interface{}{
				"getWebUIUpdateStatus": map[string]interface{}{
					"channel":    "PREVIEW",
					"tag":        webuiVersion,
					"progress":   100,
					"state":      "IDLE",
					"__typename": "WebUIUpdateStatus",
				},
				"__typename": "UpdateWebUIPayload",
			},
		}

	case "GET_GLOBAL_METADATA":
		key, _ := req.Variables["key"].(string)
		var val string
		if database.DB != nil && key != "" {
			_ = database.DB.QueryRow(c.Context(), "SELECT value FROM suwayomi.globalmeta WHERE user_id = 1 AND meta_key = $1", key).Scan(&val)
		}
		data = map[string]interface{}{
			"meta": map[string]interface{}{
				"key":        key,
				"value":      val,
				"__typename": "GlobalMetaType",
			},
		}

	case "GET_GLOBAL_METADATAS":
		nodes := []map[string]interface{}{}
		if database.DB != nil {
			rows, err := database.DB.Query(c.Context(), "SELECT meta_key, value FROM suwayomi.globalmeta WHERE user_id = 1")
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var k, v string
					if err := rows.Scan(&k, &v); err == nil {
						nodes = append(nodes, map[string]interface{}{
							"key":        k,
							"value":      v,
							"__typename": "GlobalMetaType",
						})
					}
				}
			}
		}

		data = map[string]interface{}{
			"metas": map[string]interface{}{
				"nodes":      nodes,
				"totalCount": len(nodes),
				"pageInfo": map[string]interface{}{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "GlobalMetaNodeList",
			},
		}

	case "GET_CATEGORIES_LIBRARY", "GET_CATEGORIES", "GET_CATEGORIES_BASE", "GET_CATEGORIES_SETTINGS":
		// Fetch categories dynamically from database
		nodes := []map[string]interface{}{}
		cats, err := database.GetCategoryList(c.Context())
		if err == nil {
			catMetas := make(map[int64][]map[string]interface{})
			if database.DB != nil {
				metaRows, err := database.DB.Query(c.Context(), "SELECT category_ref, meta_key, value FROM suwayomi.categorymeta WHERE user_id = 1")
				if err == nil {
					defer metaRows.Close()
					for metaRows.Next() {
						var cRef int64
						var k, v string
						if err := metaRows.Scan(&cRef, &k, &v); err == nil {
							catMetas[cRef] = append(catMetas[cRef], map[string]interface{}{
								"categoryId": cRef,
								"key":        k,
								"value":      v,
								"__typename": "CategoryMetaType",
							})
						}
					}
				}
			}

			for _, cat := range cats {
				// Count mangas in this category
				var count int
				if database.DB != nil {
					if cat.ID == 0 {
						_ = database.DB.QueryRow(c.Context(), `
							SELECT COUNT(*) 
							FROM suwayomi.manga m 
							INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga 
							WHERE COALESCE(mu.in_library, false) = true 
							  AND NOT EXISTS (SELECT 1 FROM suwayomi.categorymanga cm WHERE cm.manga = m.id)
						`).Scan(&count)
					} else {
						_ = database.DB.QueryRow(c.Context(), `
							SELECT COUNT(*) 
							FROM suwayomi.categorymanga cm 
							INNER JOIN suwayomi.mangauser mu ON cm.manga = mu.manga 
							WHERE cm.category = $1 AND COALESCE(mu.in_library, false) = true
						`, cat.ID).Scan(&count)
					}
				}
				mList := catMetas[int64(cat.ID)]
				if mList == nil {
					mList = []map[string]interface{}{}
				}
				nodes = append(nodes, map[string]interface{}{
					"id":                cat.ID,
					"name":              cat.Name,
					"order":             cat.Order,
					"default":           cat.Default,
					"isDefaultCategory": cat.ID == 0,
					"includeInUpdate":   cat.IncludeInUpdate.GraphQLEnum(),
					"includeInDownload": cat.IncludeInDownload.GraphQLEnum(),
					"mangas": map[string]interface{}{
						"totalCount": count,
					},
					"meta":       mList,
					"__typename": "CategoryType",
				})
			}
		}

		data = map[string]interface{}{
			"categories": map[string]interface{}{
				"nodes":      nodes,
				"totalCount": len(nodes),
				"pageInfo": map[string]interface{}{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "CategoryNodeList",
			},
		}

	case "GET_LIBRARY_MANGA_COUNT", "GET_MANGAS_COUNT":
		// Count total library mangas in database
		var total int
		if database.DB != nil {
			inLib := true
			var catId int64 = -1
			var isNullCat bool = false

			if cond, ok := req.Variables["condition"].(map[string]interface{}); ok {
				if v, ok := cond["inLibrary"].(bool); ok {
					inLib = v
				}
				if catIds, ok := cond["categoryIds"].([]interface{}); ok && len(catIds) > 0 {
					if idF, ok := catIds[0].(float64); ok {
						catId = int64(idF)
					}
				} else if catIdF, ok := cond["categoryIds"].(float64); ok {
					catId = int64(catIdF)
				} else if catIdF, ok := cond["categoryId"].(float64); ok {
					catId = int64(catIdF)
				}
			}

			if filterMap, ok := req.Variables["filter"].(map[string]interface{}); ok {
				if inLibFilter, ok := filterMap["inLibrary"].(map[string]interface{}); ok {
					if eq, ok := inLibFilter["equalTo"].(bool); ok {
						inLib = eq
					}
				}
				if catFilter, ok := filterMap["categoryId"].(map[string]interface{}); ok {
					if isNull, ok := catFilter["isNull"].(bool); ok && isNull {
						isNullCat = true
						catId = 0
					}
					if inArr, ok := catFilter["in"].([]interface{}); ok && len(inArr) > 0 {
						if idF, ok := inArr[0].(float64); ok {
							catId = int64(idF)
						}
					}
					if eq, ok := catFilter["equalTo"].(float64); ok {
						catId = int64(eq)
					}
				}
			}

			if catId == 0 || isNullCat {
				_ = database.DB.QueryRow(c.Context(), `
					SELECT COUNT(*) 
					FROM suwayomi.manga m 
					INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga 
					WHERE COALESCE(mu.in_library, false) = $1 
					  AND NOT EXISTS (SELECT 1 FROM suwayomi.categorymanga cm WHERE cm.manga = m.id)
				`, inLib).Scan(&total)
			} else if catId > 0 {
				_ = database.DB.QueryRow(c.Context(), `
					SELECT COUNT(*) 
					FROM suwayomi.manga m 
					INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga 
					INNER JOIN suwayomi.categorymanga cm ON m.id = cm.manga 
					WHERE COALESCE(mu.in_library, false) = $1 AND cm.category = $2
				`, inLib, catId).Scan(&total)
			} else {
				_ = database.DB.QueryRow(c.Context(), `
					SELECT COUNT(*) 
					FROM suwayomi.manga m 
					INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga 
					WHERE COALESCE(mu.in_library, false) = $1
				`, inLib).Scan(&total)
			}
		}
		data = map[string]interface{}{
			"mangas": map[string]interface{}{
				"totalCount": total,
				"__typename": "MangaNodeList",
			},
		}

	case "GET_SOURCES_LIST":
		nodes := []map[string]interface{}{}
		srcs, err := database.GetSourceList(c.Context())
		if err == nil && len(srcs) == 0 {
			_ = database.EnsureNativeSources(c.Context())
			srcs, _ = database.GetSourceList(c.Context())
		}
		if err == nil {
			for _, src := range srcs {
				if sid, errSid := strconv.ParseInt(src.ID, 10, 64); errSid == nil && sid > 0 {
					sources.RestoreNative(sid)
				}
				cw := src.ContentWarning
				if cw == "" {
					cw = "SAFE"
				}
				pkgName := src.PkgName
				iconURL := "/api/v1/source/" + src.ID + "/icon"
				nodes = append(nodes, map[string]interface{}{
					"id":             src.ID,
					"name":           src.Name,
					"displayName":    src.DisplayName,
					"lang":           src.Lang,
					"iconUrl":        iconURL,
					"contentWarning": cw,
					"isNsfw":         src.IsNsfw,
					"isConfigurable": false,
					"supportsLatest": src.SupportsLatest,
					"meta":           []interface{}{},
					"extension": map[string]interface{}{
						"pkgName":       pkgName,
						"storeIndexUrl": DefaultExtensionStoreURL,
						"extensionStore": map[string]interface{}{
							"indexUrl":   DefaultExtensionStoreURL,
							"name":       DefaultExtensionStoreName,
							"__typename": "ExtensionStoreType",
						},
						"__typename": "ExtensionType",
					},
					"__typename": "SourceType",
				})
			}
		}

		data = map[string]interface{}{
			"sources": map[string]interface{}{
				"nodes":      nodes,
				"__typename": "SourceNodeList",
			},
		}

	case "GET_EXTENSIONS":
		var filterInstalled *bool
		var filterStoreUrl string
		if cond, ok := req.Variables["condition"].(map[string]interface{}); ok {
			if v, ok := cond["isInstalled"].(bool); ok {
				filterInstalled = &v
			}
			if s, ok := cond["storeIndexUrl"].(string); ok {
				filterStoreUrl = s
			}
		}

		allNodes := getExtensionNodes(c.Context(), filterStoreUrl)
		nodes := []interface{}{}
		for _, node := range allNodes {
			if filterInstalled != nil {
				isInst, _ := node["isInstalled"].(bool)
				if isInst != *filterInstalled {
					continue
				}
			}
			nodes = append(nodes, node)
		}

		data = map[string]interface{}{
			"extensions": map[string]interface{}{
				"nodes":      nodes,
				"totalCount": len(nodes),
				"pageInfo": map[string]interface{}{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "ExtensionNodeList",
			},
		}

	case "GET_EXTENSION":
		var pkgName string
		if p, ok := req.Variables["pkgName"].(string); ok {
			pkgName = p
		}
		data = map[string]interface{}{
			"extension": getExtensionMap(c.Context(), pkgName),
		}

	case "UPDATE_EXTENSION", "updateExtension":
		var pkgName string
		var install, uninstall, update bool
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if p, ok := input["pkgName"].(string); ok && p != "" {
				pkgName = p
			} else if idStr, ok := input["id"].(string); ok && idStr != "" {
				pkgName = idStr
			} else if idNum, ok := input["id"].(float64); ok {
				if database.DB != nil {
					_ = database.DB.QueryRow(c.Context(), "SELECT pkg_name FROM suwayomi.extension WHERE id = $1", int(idNum)).Scan(&pkgName)
				}
				if pkgName == "" {
					pkgName = fmt.Sprintf("%.0f", idNum)
				}
			}

			if patch, ok := input["patch"].(map[string]interface{}); ok {
				if p, ok := patch["pkgName"].(string); ok && p != "" {
					pkgName = p
				}
				if u, ok := patch["uninstall"].(bool); ok && u {
					uninstall = true
				}
				if i, ok := patch["install"].(bool); ok && i {
					install = true
				}
				if up, ok := patch["update"].(bool); ok && up {
					update = true
				}
			}

			if !install && !uninstall && !update {
				install, _ = input["install"].(bool)
				uninstall, _ = input["uninstall"].(bool)
				update, _ = input["update"].(bool)
			}
		}

		if pkgName != "" {
			if idVal, err := strconv.Atoi(pkgName); err == nil && database.DB != nil {
				var p string
				if err := database.DB.QueryRow(c.Context(), "SELECT pkg_name FROM suwayomi.extension WHERE id = $1", idVal).Scan(&p); err == nil && p != "" {
					pkgName = p
				}
			}
		}

		log.Printf("[EXTENSIONS] UPDATE_EXTENSION: pkg=%s install=%v uninstall=%v update=%v", pkgName, install, uninstall, update)

		if pkgName != "" {
			if uninstall {
				if err := service.UninstallExtension(c.Context(), cfg, pkgName); err != nil {
					log.Printf("[EXTENSIONS] Failed to uninstall %s: %v", pkgName, err)
				}
			} else if install {
				if err := service.InstallExtension(c.Context(), cfg, pkgName); err != nil {
					log.Printf("[EXTENSIONS] Failed to install %s: %v", pkgName, err)
				}
			} else if update {
				if err := service.UpdateExtension(c.Context(), cfg, pkgName); err != nil {
					log.Printf("[EXTENSIONS] Failed to update %s: %v", pkgName, err)
				}
			}
		}

		data = map[string]interface{}{
			"updateExtension": map[string]interface{}{
				"extension":  getExtensionMap(c.Context(), pkgName),
				"__typename": "UpdateExtensionPayload",
			},
		}

	case "UPDATE_EXTENSIONS", "updateExtensions":
		var pkgs []string
		var isUninstall bool
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if list, ok := input["pkgNames"].([]interface{}); ok {
				for _, p := range list {
					if s, ok := p.(string); ok {
						pkgs = append(pkgs, s)
					}
				}
			}
			if list, ok := input["ids"].([]interface{}); ok {
				for _, p := range list {
					if s, ok := p.(string); ok {
						if idVal, err := strconv.Atoi(s); err == nil && database.DB != nil {
							var pName string
							_ = database.DB.QueryRow(c.Context(), "SELECT pkg_name FROM suwayomi.extension WHERE id = $1", idVal).Scan(&pName)
							if pName != "" {
								pkgs = append(pkgs, pName)
								continue
							}
						}
						pkgs = append(pkgs, s)
					} else if f, ok := p.(float64); ok && database.DB != nil {
						var pName string
						_ = database.DB.QueryRow(c.Context(), "SELECT pkg_name FROM suwayomi.extension WHERE id = $1", int(f)).Scan(&pName)
						if pName != "" {
							pkgs = append(pkgs, pName)
						}
					}
				}
			}
			if patch, ok := input["patch"].(map[string]interface{}); ok {
				if u, ok := patch["uninstall"].(bool); ok && u {
					isUninstall = true
				}
			}
			if u, ok := input["uninstall"].(bool); ok && u {
				isUninstall = true
			}
		}
		extList := []interface{}{}
		for _, p := range pkgs {
			if isUninstall {
				_ = service.UninstallExtension(c.Context(), cfg, p)
			} else {
				_ = service.UpdateExtension(c.Context(), cfg, p)
			}
			extList = append(extList, getExtensionMap(c.Context(), p))
		}
		data = map[string]interface{}{
			"updateExtensions": map[string]interface{}{
				"extensions": extList,
				"__typename": "UpdateExtensionsPayload",
			},
		}

	case "INSTALL_EXTERNAL_EXTENSION", "installExternalExtension":
		data = map[string]interface{}{
			"installExternalExtension": map[string]interface{}{
				"extension":  getExtensionMap(c.Context(), "eu.kanade.tachiyomi.extension.all.mangadex"),
				"__typename": "InstallExternalExtensionPayload",
			},
		}

	case "GET_CHAPTERS_UPDATES":
		limit := 150
		if f, ok := req.Variables["first"].(float64); ok && f > 0 {
			limit = int(f)
		}
		offset := 0
		if o, ok := req.Variables["offset"].(float64); ok && o >= 0 {
			offset = int(o)
		}

		chapterNodes, totalCount, maxFetchedAt, err := fetchUpdatesOrHistoryChapters(c.Context(), false, limit, offset)
		if err != nil {
			log.Printf("[ERROR] GET_CHAPTERS_UPDATES error: %v", err)
		}

		maxFetchedAtMillis := maxFetchedAt
		if maxFetchedAtMillis > 0 && maxFetchedAtMillis < 10000000000 {
			maxFetchedAtMillis *= 1000
		}

		data = map[string]interface{}{
			"lastUpdateTimestamp": map[string]interface{}{
				"timestamp":  fmt.Sprintf("%d", maxFetchedAtMillis),
				"__typename": "LastUpdateTimestamp",
			},
			"chapters": map[string]interface{}{
				"nodes":      chapterNodes,
				"totalCount": totalCount,
				"pageInfo": map[string]interface{}{
					"hasNextPage":     offset+len(chapterNodes) < totalCount,
					"hasPreviousPage": offset > 0,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "ChapterNodeList",
			},
		}

	case "GET_LAST_UPDATE_TIMESTAMP", "GET_LAST_GLOBAL_UPDATE_TIMESTAMP":
		var maxFetchedAt int64
		if database.DB != nil {
			_ = database.DB.QueryRow(c.Context(), `
				SELECT COALESCE(MAX(c.fetched_at), 0)
				FROM suwayomi.chapter c
				INNER JOIN suwayomi.mangauser mu ON c.manga = mu.manga
				WHERE COALESCE(mu.in_library, false) = true
			`).Scan(&maxFetchedAt)
		}
		if maxFetchedAt > 0 && maxFetchedAt < 10000000000 {
			maxFetchedAt *= 1000
		}
		data = map[string]interface{}{
			"lastUpdateTimestamp": map[string]interface{}{
				"timestamp":  fmt.Sprintf("%d", maxFetchedAt),
				"__typename": "LastUpdateTimestamp",
			},
		}

	case "GET_SYNC_STATUS":
		data = map[string]interface{}{
			"lastSyncStatus": map[string]interface{}{
				"backupRestoreId": nil,
				"endDate":         nil,
				"errorMessage":    nil,
				"startDate":       nil,
				"state":           "IDLE",
				"__typename":      "SyncStatus",
			},
		}

	case "GET_UPDATE_STATUS", "GET_GLOBAL_UPDATE_SUMMARY":
		data = map[string]interface{}{
			"libraryUpdateStatus": service.GlobalUpdateManager.GetStatusGraphQL(),
		}

	case "UPDATE_LIBRARY", "updateLibrary":
		service.GlobalUpdateManager.StartUpdate()
		data = map[string]interface{}{
			"updateLibrary": map[string]interface{}{
				"updateStatus": service.GlobalUpdateManager.GetStatusGraphQL(),
				"__typename":   "UpdateLibraryPayload",
			},
		}

	case "STOP_UPDATER", "updateStop":
		service.GlobalUpdateManager.CancelUpdate()
		data = map[string]interface{}{
			"updateStop": map[string]interface{}{
				"clientMutationId": nil,
				"__typename":       "UpdateStopPayload",
			},
		}

	case "GET_KO_SYNC_STATUS":
		data = map[string]interface{}{
			"koSyncStatus": map[string]interface{}{
				"serverAddress": "",
				"username":      "",
				"isLoggedIn":    false,
				"__typename":    "KoSyncStatusPayload",
			},
		}

	case "GET_CHAPTERS_HISTORY":
		limit := 150
		if f, ok := req.Variables["first"].(float64); ok && f > 0 {
			limit = int(f)
		}
		offset := 0
		if o, ok := req.Variables["offset"].(float64); ok && o >= 0 {
			offset = int(o)
		}

		chapterNodes, totalCount, _, err := fetchUpdatesOrHistoryChapters(c.Context(), true, limit, offset)
		if err != nil {
			log.Printf("[ERROR] GET_CHAPTERS_HISTORY error: %v", err)
		}

		data = map[string]interface{}{
			"chapters": map[string]interface{}{
				"nodes":      chapterNodes,
				"totalCount": totalCount,
				"pageInfo": map[string]interface{}{
					"hasNextPage":     offset+len(chapterNodes) < totalCount,
					"hasPreviousPage": offset > 0,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "ChapterNodeList",
			},
		}

	case "UPDATE_GLOBAL_METADATA":
		preDeletedMetas := []map[string]interface{}{}
		postDeletedMetas := []map[string]interface{}{}
		updatedMetas := []map[string]interface{}{}
		migrationMetas := []map[string]interface{}{}

		if database.DB != nil {
			processGlobalDeletions := func(delInput map[string]interface{}) []map[string]interface{} {
				result := []map[string]interface{}{}
				if delInput == nil {
					return result
				}
				if keys, ok := delInput["keys"].([]interface{}); ok && len(keys) > 0 {
					for _, k := range keys {
					if keyStr, ok := k.(string); ok && keyStr != "" {
						_, _ = database.DB.Exec(c.Context(), "DELETE FROM suwayomi.globalmeta WHERE user_id = 1 AND meta_key = $1", keyStr)
						result = append(result, map[string]interface{}{
							"key":        keyStr,
							"value":      "",
							"__typename": "GlobalMetaType",
						})
					}
					}
				}
				if prefixes, ok := delInput["prefixes"].([]interface{}); ok && len(prefixes) > 0 {
					for _, p := range prefixes {
					if pStr, ok := p.(string); ok && pStr != "" {
						_, _ = database.DB.Exec(c.Context(), "DELETE FROM suwayomi.globalmeta WHERE user_id = 1 AND meta_key LIKE $1 || '%'", pStr)
					}
					}
				}
				return result
			}

			if preDel, ok := req.Variables["preUpdateDeleteInput"].(map[string]interface{}); ok {
				preDeletedMetas = processGlobalDeletions(preDel)
			}
			if postDel, ok := req.Variables["postUpdateDeleteInput"].(map[string]interface{}); ok {
				postDeletedMetas = processGlobalDeletions(postDel)
			}

			processGlobalUpdates := func(upInput map[string]interface{}) []map[string]interface{} {
				result := []map[string]interface{}{}
				if upInput == nil {
					return result
				}
				if metas, ok := upInput["metas"].([]interface{}); ok {
					for _, m := range metas {
					if metaMap, ok := m.(map[string]interface{}); ok {
						k, _ := metaMap["key"].(string)
						var valStr string
						if v, ok := metaMap["value"].(string); ok {
							valStr = v
						} else {
							vBytes, _ := json.Marshal(metaMap["value"])
							valStr = string(vBytes)
						}

						if k != "" {
							_, _ = database.DB.Exec(c.Context(), `
								INSERT INTO suwayomi.globalmeta (user_id, meta_key, value)
								VALUES (1, $1, $2)
								ON CONFLICT (user_id, meta_key) DO UPDATE SET value = EXCLUDED.value
							`, k, valStr)

							result = append(result, map[string]interface{}{
								"key":        k,
								"value":      valStr,
								"__typename": "GlobalMetaType",
							})
						}
					}
					}
				}
				return result
			}

			if updateInput, ok := req.Variables["updateInput"].(map[string]interface{}); ok {
				updatedMetas = processGlobalUpdates(updateInput)
			}
			if migrateInput, ok := req.Variables["migrateInput"].(map[string]interface{}); ok {
				migrationMetas = processGlobalUpdates(migrateInput)
			}
		}

		resMap := make(map[string]interface{})
		hasPre, _ := req.Variables["hasPreUpdateDeletions"].(bool)
		hasUp, _ := req.Variables["hasUpdates"].(bool)
		hasPost, _ := req.Variables["hasPostUpdateDeletions"].(bool)
		isMig, _ := req.Variables["isMigration"].(bool)

		if _, ok := req.Variables["hasUpdates"]; !ok {
			hasUp = true
		}

		if hasPre {
			resMap["preUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      preDeletedMetas,
				"__typename": "DeleteGlobalMetasPayload",
			}
		}
		if hasUp {
			resMap["updatedMeta"] = map[string]interface{}{
				"metas":      updatedMetas,
				"__typename": "SetGlobalMetasPayload",
			}
		}
		if hasPost {
			resMap["postUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      postDeletedMetas,
				"__typename": "DeleteGlobalMetasPayload",
			}
		}
		if isMig {
			resMap["migrationMeta"] = map[string]interface{}{
				"metas":      migrationMetas,
				"__typename": "SetGlobalMetasPayload",
			}
		}
		data = resMap

	case "UPDATE_MANGA_METADATA":
		updatedMetas := []map[string]interface{}{}
		deletedMetas := []map[string]interface{}{}
		if database.DB != nil {
			processDeletions := func(delInput map[string]interface{}) {
				if delInput == nil {
					return
				}
				// 1. Array of items
				if items, ok := delInput["items"].([]interface{}); ok {
					for _, item := range items {
						if itemMap, ok := item.(map[string]interface{}); ok {
							var mIDs []int64
							if ids, ok := itemMap["mangaIds"].([]interface{}); ok {
								for _, idVal := range ids {
									if idF, ok := idVal.(float64); ok {
										mIDs = append(mIDs, int64(idF))
									}
								}
							} else if idF, ok := itemMap["mangaId"].(float64); ok {
								mIDs = append(mIDs, int64(idF))
							}

							// Keys
							if keys, ok := itemMap["keys"].([]interface{}); ok {
								for _, kVal := range keys {
									if kStr, ok := kVal.(string); ok {
										for _, mID := range mIDs {
											_, _ = database.DB.Exec(c.Context(), "DELETE FROM suwayomi.mangameta WHERE user_id = 1 AND manga_ref = $1 AND meta_key = $2", mID, kStr)
											deletedMetas = append(deletedMetas, map[string]interface{}{
												"mangaId":    mID,
												"key":        kStr,
												"value":      "",
												"__typename": "MangaMetaType",
											})
										}
									}
								}
							}
							// Prefixes
							if prefixes, ok := itemMap["prefixes"].([]interface{}); ok {
								for _, pVal := range prefixes {
									if pStr, ok := pVal.(string); ok {
										for _, mID := range mIDs {
											_, _ = database.DB.Exec(c.Context(), "DELETE FROM suwayomi.mangameta WHERE user_id = 1 AND manga_ref = $1 AND meta_key LIKE $2 || '%'", mID, pStr)
										}
									}
								}
							}
						}
					}
				}
				// 2. Direct keys (legacy)
				var mID int64
				if idF, ok := delInput["mangaId"].(float64); ok {
					mID = int64(idF)
				}
				if keys, ok := delInput["keys"].([]interface{}); ok && mID > 0 {
					for _, kVal := range keys {
						if kStr, ok := kVal.(string); ok {
							_, _ = database.DB.Exec(c.Context(), "DELETE FROM suwayomi.mangameta WHERE user_id = 1 AND manga_ref = $1 AND meta_key = $2", mID, kStr)
							deletedMetas = append(deletedMetas, map[string]interface{}{
								"mangaId":    mID,
								"key":        kStr,
								"value":      "",
								"__typename": "MangaMetaType",
							})
						}
					}
				}
			}

			if preDel, ok := req.Variables["preUpdateDeleteInput"].(map[string]interface{}); ok {
				processDeletions(preDel)
			}
			if postDel, ok := req.Variables["postUpdateDeleteInput"].(map[string]interface{}); ok {
				processDeletions(postDel)
			}

			processUpdates := func(upInput map[string]interface{}) {
				if upInput == nil {
					return
				}
				// 1. Array of items
				if items, ok := upInput["items"].([]interface{}); ok {
					for _, item := range items {
						if itemMap, ok := item.(map[string]interface{}); ok {
							var mIDs []int64
							if ids, ok := itemMap["mangaIds"].([]interface{}); ok {
								for _, idVal := range ids {
									if idF, ok := idVal.(float64); ok {
										mIDs = append(mIDs, int64(idF))
									}
								}
							} else if idF, ok := itemMap["mangaId"].(float64); ok {
								mIDs = append(mIDs, int64(idF))
							}

							if metas, ok := itemMap["metas"].([]interface{}); ok {
								for _, m := range metas {
									if metaMap, ok := m.(map[string]interface{}); ok {
										k, _ := metaMap["key"].(string)
										var valStr string
										if s, ok := metaMap["value"].(string); ok {
											valStr = s
										} else if f, ok := metaMap["value"].(float64); ok {
											if f == float64(int64(f)) {
												valStr = fmt.Sprintf("%d", int64(f))
											} else {
												valStr = fmt.Sprintf("%g", f)
											}
										} else if b, ok := metaMap["value"].(bool); ok {
											valStr = fmt.Sprintf("%t", b)
										} else {
											vBytes, _ := json.Marshal(metaMap["value"])
											valStr = string(vBytes)
										}

										if k != "" {
											for _, mID := range mIDs {
												_, _ = database.DB.Exec(c.Context(), `
													INSERT INTO suwayomi.mangameta (user_id, manga_ref, meta_key, value)
													VALUES (1, $1, $2, $3)
													ON CONFLICT (user_id, manga_ref, meta_key) DO UPDATE SET value = EXCLUDED.value
												`, mID, k, valStr)

												updatedMetas = append(updatedMetas, map[string]interface{}{
													"mangaId":    mID,
													"key":        k,
													"value":      valStr,
													"__typename": "MangaMetaType",
												})
											}
										}
									}
								}
							}
						}
					}
				}
				// 2. Direct updates (legacy)
				var mID int64
				if idF, ok := upInput["mangaId"].(float64); ok {
					mID = int64(idF)
				}
				if metas, ok := upInput["metas"].([]interface{}); ok && mID > 0 {
					for _, m := range metas {
						if metaMap, ok := m.(map[string]interface{}); ok {
							k, _ := metaMap["key"].(string)
							var valStr string
							if s, ok := metaMap["value"].(string); ok {
								valStr = s
							} else {
								vBytes, _ := json.Marshal(metaMap["value"])
								valStr = string(vBytes)
							}
							if k != "" {
								_, _ = database.DB.Exec(c.Context(), `
									INSERT INTO suwayomi.mangameta (user_id, manga_ref, meta_key, value)
									VALUES (1, $1, $2, $3)
									ON CONFLICT (user_id, manga_ref, meta_key) DO UPDATE SET value = EXCLUDED.value
								`, mID, k, valStr)

								updatedMetas = append(updatedMetas, map[string]interface{}{
									"mangaId":    mID,
									"key":        k,
									"value":      valStr,
									"__typename": "MangaMetaType",
								})
							}
						}
					}
				}
			}

			if upInput, ok := req.Variables["updateInput"].(map[string]interface{}); ok {
				processUpdates(upInput)
			}
			if migInput, ok := req.Variables["migrateInput"].(map[string]interface{}); ok {
				processUpdates(migInput)
			}
		}

		resMap := make(map[string]interface{})
		hasPre, _ := req.Variables["hasPreUpdateDeletions"].(bool)
		hasUp, _ := req.Variables["hasUpdates"].(bool)
		hasPost, _ := req.Variables["hasPostUpdateDeletions"].(bool)
		isMig, _ := req.Variables["isMigration"].(bool)

		if _, ok := req.Variables["hasUpdates"]; !ok {
			hasUp = true
		}

		if hasPre {
			resMap["preUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      deletedMetas,
				"__typename": "DeleteMangaMetasPayload",
			}
		}
		if hasUp {
			resMap["updatedMeta"] = map[string]interface{}{
				"metas":      updatedMetas,
				"__typename": "SetMangaMetasPayload",
			}
		}
		if hasPost {
			resMap["postUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      deletedMetas,
				"__typename": "DeleteMangaMetasPayload",
			}
		}
		if isMig {
			resMap["migrationMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "SetMangaMetasPayload",
			}
		}
		data = resMap

	case "UPDATE_CATEGORY_METADATA":
		updatedMetas := []map[string]interface{}{}
		deletedMetas := []map[string]interface{}{}
		if database.DB != nil {
			processDeletions := func(delInput map[string]interface{}) {
				if delInput == nil {
					return
				}
				// 1. Array of items
				if items, ok := delInput["items"].([]interface{}); ok {
					for _, item := range items {
						if itemMap, ok := item.(map[string]interface{}); ok {
							var cIDs []int64
							if ids, ok := itemMap["categoryIds"].([]interface{}); ok {
								for _, idVal := range ids {
									if idF, ok := idVal.(float64); ok {
										cIDs = append(cIDs, int64(idF))
									}
								}
							} else if idF, ok := itemMap["categoryId"].(float64); ok {
								cIDs = append(cIDs, int64(idF))
							}

							// Keys
							if keys, ok := itemMap["keys"].([]interface{}); ok {
								for _, kVal := range keys {
									if kStr, ok := kVal.(string); ok {
										for _, cID := range cIDs {
											_, _ = database.DB.Exec(c.Context(), "DELETE FROM suwayomi.categorymeta WHERE user_id = 1 AND category_ref = $1 AND meta_key = $2", cID, kStr)
											deletedMetas = append(deletedMetas, map[string]interface{}{
												"categoryId": cID,
												"key":        kStr,
												"value":      "",
												"__typename": "CategoryMetaType",
											})
										}
									}
								}
							}
							// Prefixes
							if prefixes, ok := itemMap["prefixes"].([]interface{}); ok {
								for _, pVal := range prefixes {
									if pStr, ok := pVal.(string); ok {
										for _, cID := range cIDs {
											_, _ = database.DB.Exec(c.Context(), "DELETE FROM suwayomi.categorymeta WHERE user_id = 1 AND category_ref = $1 AND meta_key LIKE $2 || '%'", cID, pStr)
										}
									}
								}
							}
						}
					}
				}
				// 2. Direct keys (legacy)
				var cID int64
				if idF, ok := delInput["categoryId"].(float64); ok {
					cID = int64(idF)
				}
				if keys, ok := delInput["keys"].([]interface{}); ok {
					for _, kVal := range keys {
						if kStr, ok := kVal.(string); ok {
							_, _ = database.DB.Exec(c.Context(), "DELETE FROM suwayomi.categorymeta WHERE user_id = 1 AND category_ref = $1 AND meta_key = $2", cID, kStr)
							deletedMetas = append(deletedMetas, map[string]interface{}{
								"categoryId": cID,
								"key":        kStr,
								"value":      "",
								"__typename": "CategoryMetaType",
							})
						}
					}
				}
			}

			if preDel, ok := req.Variables["preUpdateDeleteInput"].(map[string]interface{}); ok {
				processDeletions(preDel)
			}
			if postDel, ok := req.Variables["postUpdateDeleteInput"].(map[string]interface{}); ok {
				processDeletions(postDel)
			}

			processUpdates := func(upInput map[string]interface{}) {
				if upInput == nil {
					return
				}
				// 1. Array of items
				if items, ok := upInput["items"].([]interface{}); ok {
					for _, item := range items {
						if itemMap, ok := item.(map[string]interface{}); ok {
							var cIDs []int64
							if ids, ok := itemMap["categoryIds"].([]interface{}); ok {
								for _, idVal := range ids {
									if idF, ok := idVal.(float64); ok {
										cIDs = append(cIDs, int64(idF))
									}
								}
							} else if idF, ok := itemMap["categoryId"].(float64); ok {
								cIDs = append(cIDs, int64(idF))
							}

							if metas, ok := itemMap["metas"].([]interface{}); ok {
								for _, m := range metas {
									if metaMap, ok := m.(map[string]interface{}); ok {
										k, _ := metaMap["key"].(string)
										var valStr string
										if s, ok := metaMap["value"].(string); ok {
											valStr = s
										} else if f, ok := metaMap["value"].(float64); ok {
											if f == float64(int64(f)) {
												valStr = fmt.Sprintf("%d", int64(f))
											} else {
												valStr = fmt.Sprintf("%g", f)
											}
										} else if b, ok := metaMap["value"].(bool); ok {
											valStr = fmt.Sprintf("%t", b)
										} else {
											vBytes, _ := json.Marshal(metaMap["value"])
											valStr = string(vBytes)
										}

										if k != "" {
											for _, cID := range cIDs {
												_, _ = database.DB.Exec(c.Context(), `
													INSERT INTO suwayomi.categorymeta (user_id, category_ref, meta_key, value)
													VALUES (1, $1, $2, $3)
													ON CONFLICT (user_id, category_ref, meta_key) DO UPDATE SET value = EXCLUDED.value
												`, cID, k, valStr)

												updatedMetas = append(updatedMetas, map[string]interface{}{
													"categoryId": cID,
													"key":        k,
													"value":      valStr,
													"__typename": "CategoryMetaType",
												})
											}
										}
									}
								}
							}
						}
					}
				}
				// 2. Direct updates (legacy)
				var cID int64
				if idF, ok := upInput["categoryId"].(float64); ok {
					cID = int64(idF)
				}
				if metas, ok := upInput["metas"].([]interface{}); ok {
					for _, m := range metas {
						if metaMap, ok := m.(map[string]interface{}); ok {
							k, _ := metaMap["key"].(string)
							var valStr string
							if s, ok := metaMap["value"].(string); ok {
								valStr = s
							} else {
								vBytes, _ := json.Marshal(metaMap["value"])
								valStr = string(vBytes)
							}
							if k != "" {
								_, _ = database.DB.Exec(c.Context(), `
									INSERT INTO suwayomi.categorymeta (user_id, category_ref, meta_key, value)
									VALUES (1, $1, $2, $3)
									ON CONFLICT (user_id, category_ref, meta_key) DO UPDATE SET value = EXCLUDED.value
								`, cID, k, valStr)

								updatedMetas = append(updatedMetas, map[string]interface{}{
									"categoryId": cID,
									"key":        k,
									"value":      valStr,
									"__typename": "CategoryMetaType",
								})
							}
						}
					}
				}
			}

			if upInput, ok := req.Variables["updateInput"].(map[string]interface{}); ok {
				processUpdates(upInput)
			}
			if migInput, ok := req.Variables["migrateInput"].(map[string]interface{}); ok {
				processUpdates(migInput)
			}
		}

		resMap := make(map[string]interface{})
		hasPre, _ := req.Variables["hasPreUpdateDeletions"].(bool)
		hasUp, _ := req.Variables["hasUpdates"].(bool)
		hasPost, _ := req.Variables["hasPostUpdateDeletions"].(bool)
		isMig, _ := req.Variables["isMigration"].(bool)

		if _, ok := req.Variables["hasUpdates"]; !ok {
			hasUp = true
		}

		if hasPre {
			resMap["preUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      deletedMetas,
				"__typename": "DeleteCategoryMetasPayload",
			}
		}
		if hasUp {
			resMap["updatedMeta"] = map[string]interface{}{
				"metas":      updatedMetas,
				"__typename": "SetCategoryMetasPayload",
			}
		}
		if hasPost {
			resMap["postUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      deletedMetas,
				"__typename": "DeleteCategoryMetasPayload",
			}
		}
		if isMig {
			resMap["migrationMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "SetCategoryMetasPayload",
			}
		}
		data = resMap

	case "UPDATE_CHAPTER_METADATA":
		resMap := make(map[string]interface{})
		hasPre, _ := req.Variables["hasPreUpdateDeletions"].(bool)
		hasUp, _ := req.Variables["hasUpdates"].(bool)
		hasPost, _ := req.Variables["hasPostUpdateDeletions"].(bool)
		isMig, _ := req.Variables["isMigration"].(bool)

		if _, ok := req.Variables["hasUpdates"]; !ok {
			hasUp = true
		}

		if hasPre {
			resMap["preUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "DeleteChapterMetasPayload",
			}
		}
		if hasUp {
			resMap["updatedMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "SetChapterMetasPayload",
			}
		}
		if hasPost {
			resMap["postUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "DeleteChapterMetasPayload",
			}
		}
		if isMig {
			resMap["migrationMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "SetChapterMetasPayload",
			}
		}
		data = resMap

	case "UPDATE_SOURCE_METADATA":
		resMap := make(map[string]interface{})
		hasPre, _ := req.Variables["hasPreUpdateDeletions"].(bool)
		hasUp, _ := req.Variables["hasUpdates"].(bool)
		hasPost, _ := req.Variables["hasPostUpdateDeletions"].(bool)
		isMig, _ := req.Variables["isMigration"].(bool)

		if _, ok := req.Variables["hasUpdates"]; !ok {
			hasUp = true
		}

		if hasPre {
			resMap["preUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "DeleteSourceMetasPayload",
			}
		}
		if hasUp {
			resMap["updatedMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "SetSourceMetasPayload",
			}
		}
		if hasPost {
			resMap["postUpdateDeletedMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "DeleteSourceMetasPayload",
			}
		}
		if isMig {
			resMap["migrationMeta"] = map[string]interface{}{
				"metas":      []interface{}{},
				"__typename": "SetSourceMetasPayload",
			}
		}
		data = resMap

	case "GET_EXTENSION_STORE":
		indexUrl, _ := req.Variables["indexUrl"].(string)
		store := map[string]interface{}{
			"id":               1,
			"name":             "Extension Store",
			"indexUrl":         indexUrl,
			"isLegacy":         false,
			"badgeLabel":       "",
			"signingKey":       "",
			"contactWebsite":   "",
			"contactDiscord":   "",
			"extensionListUrl": nil,
			"extensions": map[string]interface{}{
				"totalCount": 0,
			},
			"__typename": "ExtensionStoreType",
		}
		if dbStores, err := database.GetExtensionStores(c.Context()); err == nil {
			for _, s := range dbStores {
				if s.IndexURL == indexUrl {
					store["id"] = s.ID
					store["name"] = s.Name
					store["badgeLabel"] = s.BadgeLabel
					store["signingKey"] = s.SigningKey
					store["contactWebsite"] = s.ContactWebsite
					store["contactDiscord"] = s.ContactDiscord
					store["extensions"] = map[string]interface{}{
						"totalCount": s.TotalCount,
					}
					break
				}
			}
		}
		data = map[string]interface{}{
			"extensionStore": store,
		}

	case "GET_EXTENSION_STORES":
		dbStores, err := database.GetExtensionStores(c.Context())
		if err != nil || len(dbStores) == 0 {
			_ = database.EnsureDefaultExtensionStores(c.Context())
			dbStores, _ = database.GetExtensionStores(c.Context())
		}

		stores := []map[string]interface{}{}
		for _, s := range dbStores {
			stores = append(stores, map[string]interface{}{
				"id":               s.ID,
				"name":             s.Name,
				"indexUrl":         s.IndexURL,
				"isLegacy":         s.IsLegacy,
				"badgeLabel":       s.BadgeLabel,
				"signingKey":       s.SigningKey,
				"contactWebsite":   s.ContactWebsite,
				"contactDiscord":   s.ContactDiscord,
				"extensionListUrl": nil,
				"extensions": map[string]interface{}{
					"totalCount": s.TotalCount,
				},
				"__typename": "ExtensionStoreType",
			})
		}

		data = map[string]interface{}{
			"extensionStores": map[string]interface{}{
				"nodes":      stores,
				"totalCount": len(stores),
				"pageInfo": map[string]interface{}{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "ExtensionStoreNodeList",
			},
		}

	case "GET_MIGRATABLE_SOURCES":
		nodes := []map[string]interface{}{}
		if database.DB != nil {
			rows, err := database.DB.Query(c.Context(), `
				SELECT m.source, COALESCE(s.name, ''), COALESCE(s.lang, 'all')
				FROM suwayomi.manga m
				LEFT JOIN suwayomi.mangauser mu ON m.id = mu.manga
				LEFT JOIN suwayomi.source s ON m.source = s.id
				WHERE COALESCE(mu.in_library, false) = true
			`)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var srcID int64
					var name, lang string
					if err := rows.Scan(&srcID, &name, &lang); err == nil {
						sIdStr := fmt.Sprintf("%d", srcID)
						displayName := name
						if known, ok := database.KnownSourcesByID[srcID]; ok {
							if name == "" || name == "Source" || strings.HasPrefix(name, "Source ") {
								name = known.Name
							}
							if lang == "" || (lang == "all" && known.Lang != "all") {
								lang = known.Lang
							}
							if known.DisplayName != "" {
								displayName = known.DisplayName
							}
						} else if name == "" || name == "Source" || strings.HasPrefix(name, "Source ") {
							if sName := sources.GetSourceName(srcID); sName != "" {
								name = sName
							} else {
								name = sIdStr
							}
						}
						if displayName == "" {
							displayName = name
						}
						if lang == "" {
							lang = "all"
						}
						nodes = append(nodes, map[string]interface{}{
							"sourceId": sIdStr,
							"source": map[string]interface{}{
								"id":          sIdStr,
								"name":        name,
								"displayName": displayName,
								"lang":        lang,
								"iconUrl":     "/api/v1/source/" + sIdStr + "/icon",
								"__typename":  "SourceType",
							},
							"__typename": "MangaType",
						})
					}
				}
			} else {
				log.Printf("[ERROR] GET_MIGRATABLE_SOURCES query error: %v", err)
			}
		}

		data = map[string]interface{}{
			"mangas": map[string]interface{}{
				"nodes":      nodes,
				"totalCount": len(nodes),
				"pageInfo": map[string]interface{}{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "MangaNodeList",
			},
		}

	case "GET_SOURCE_MIGRATABLE":
		var sourceID string
		if idVal, ok := req.Variables["id"].(string); ok {
			sourceID = idVal
		} else if idVal, ok := req.Variables["id"].(float64); ok {
			sourceID = fmt.Sprintf("%.0f", idVal)
		}

		var name, lang string
		if database.DB != nil && sourceID != "" {
			if idNum, err := strconv.ParseInt(sourceID, 10, 64); err == nil {
				_ = database.DB.QueryRow(c.Context(), "SELECT COALESCE(name, ''), COALESCE(lang, 'all') FROM suwayomi.source WHERE id = $1", idNum).Scan(&name, &lang)
			}
		}

		displayName := name
		if idNum, err := strconv.ParseInt(sourceID, 10, 64); err == nil {
			if known, ok := database.KnownSourcesByID[idNum]; ok {
				if name == "" || name == "Source" || strings.HasPrefix(name, "Source ") {
					name = known.Name
				}
				if lang == "" || (lang == "all" && known.Lang != "all") {
					lang = known.Lang
				}
				if known.DisplayName != "" {
					displayName = known.DisplayName
				}
			}
		}

		if name == "" {
			if idNum, err := strconv.ParseInt(sourceID, 10, 64); err == nil {
				name = sources.GetSourceName(idNum)
			}
			if name == "" {
				name = sourceID
			}
		}
		if displayName == "" {
			displayName = name
		}
		if lang == "" {
			lang = "all"
		}

		data = map[string]interface{}{
			"source": map[string]interface{}{
				"id":          sourceID,
				"name":        name,
				"displayName": displayName,
				"lang":        lang,
				"iconUrl":     "/api/v1/source/" + sourceID + "/icon",
				"__typename":  "SourceType",
			},
		}

	case "GET_MIGRATABLE_SOURCE_MANGAS":
		var sourceID int64
		if cond, ok := req.Variables["condition"].(map[string]interface{}); ok {
			if sId, ok := cond["sourceId"].(string); ok {
				sourceID, _ = strconv.ParseInt(sId, 10, 64)
			} else if sId, ok := cond["sourceId"].(float64); ok {
				sourceID = int64(sId)
			}
		}

		trueVal := true
		var srcPtr *int64
		if sourceID > 0 {
			srcPtr = &sourceID
		}

		nodes, err := fetchMangaGraphQLNodes(c.Context(), MangaGraphQLFilter{
			SourceID:  srcPtr,
			InLibrary: &trueVal,
			OrderBy:   "ORDER BY m.title ASC",
		})
		if err != nil {
			log.Printf("[ERROR] GET_MIGRATABLE_SOURCE_MANGAS error: %v", err)
			nodes = []map[string]interface{}{}
		}

		data = map[string]interface{}{
			"mangas": map[string]interface{}{
				"nodes":      nodes,
				"totalCount": len(nodes),
				"pageInfo": map[string]interface{}{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "MangaNodeList",
			},
		}

	case "GET_MANGA_TO_MIGRATE":
		var id int64
		if val, ok := req.Variables["id"].(float64); ok {
			id = int64(val)
		} else if s, ok := req.Variables["id"].(string); ok {
			id, _ = strconv.ParseInt(s, 10, 64)
		}

		if id <= 0 {
			data = map[string]interface{}{"manga": nil}
			break
		}

		mNodes, _ := fetchMangaGraphQLNodes(c.Context(), MangaGraphQLFilter{
			MangaIDs: []int64{id},
		})

		var mangaNode map[string]interface{}
		if len(mNodes) > 0 {
			mangaNode = mNodes[0]
		} else {
			mangaNode = map[string]interface{}{
				"id":         id,
				"title":      "",
				"inLibrary":  false,
				"__typename": "MangaType",
			}
		}

		cNodes, _ := fetchChapterGraphQLNodes(c.Context(), id)
		mangaNode["chapters"] = map[string]interface{}{
			"nodes":      cNodes,
			"totalCount": len(cNodes),
			"pageInfo": map[string]interface{}{
				"hasNextPage":     false,
				"hasPreviousPage": false,
				"startCursor":     nil,
				"endCursor":       nil,
				"__typename":      "PageInfo",
			},
			"__typename": "ChapterNodeList",
		}

		data = map[string]interface{}{
			"manga": mangaNode,
		}

	case "GET_TRACKERS_SETTINGS", "GET_TRACKERS_BIND":
		trackers := []map[string]interface{}{
			{
				"id":                      1,
				"name":                    "MyAnimeList",
				"icon":                    "/api/v1/track/1/thumbnail",
				"isLoggedIn":              false,
				"authUrl":                 "https://myanimelist.net",
				"supportsTrackDeletion":   true,
				"supportsReadingDates":    true,
				"supportsPrivateTracking": true,
				"__typename":              "TrackerType",
			},
			{
				"id":                      2,
				"name":                    "AniList",
				"icon":                    "/api/v1/track/2/thumbnail",
				"isLoggedIn":              false,
				"authUrl":                 "https://anilist.co",
				"supportsTrackDeletion":   true,
				"supportsReadingDates":    true,
				"supportsPrivateTracking": true,
				"__typename":              "TrackerType",
			},
			{
				"id":                      3,
				"name":                    "Kitsu",
				"icon":                    "/api/v1/track/3/thumbnail",
				"isLoggedIn":              false,
				"authUrl":                 "https://kitsu.io",
				"supportsTrackDeletion":   true,
				"supportsReadingDates":    true,
				"supportsPrivateTracking": true,
				"__typename":              "TrackerType",
			},
			{
				"id":                      4,
				"name":                    "Shikimori",
				"icon":                    "/api/v1/track/4/thumbnail",
				"isLoggedIn":              false,
				"authUrl":                 "https://shikimori.one",
				"supportsTrackDeletion":   true,
				"supportsReadingDates":    true,
				"supportsPrivateTracking": true,
				"__typename":              "TrackerType",
			},
			{
				"id":                      5,
				"name":                    "Bangumi",
				"icon":                    "/api/v1/track/5/thumbnail",
				"isLoggedIn":              false,
				"authUrl":                 "https://bgm.tv",
				"supportsTrackDeletion":   true,
				"supportsReadingDates":    true,
				"supportsPrivateTracking": true,
				"__typename":              "TrackerType",
			},
		}

		data = map[string]interface{}{
			"trackers": map[string]interface{}{
				"nodes":      trackers,
				"totalCount": len(trackers),
				"pageInfo": map[string]interface{}{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "TrackerNodeList",
			},
		}

	case "TRACKER_BIND_TRACK_RECORD":
		var mangaID int64
		var trackRecordID int64
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if mVal, ok := input["mangaId"].(float64); ok {
				mangaID = int64(mVal)
			}
			if tVal, ok := input["trackRecordId"].(float64); ok {
				trackRecordID = int64(tVal)
			}
		}
		data = map[string]interface{}{
			"bindTrackRecord": map[string]interface{}{
				"trackRecord": map[string]interface{}{
					"id":        trackRecordID,
					"trackerId": 1,
					"manga": map[string]interface{}{
						"id": mangaID,
						"trackRecords": map[string]interface{}{
							"totalCount": 0,
							"nodes":      []interface{}{},
							"__typename": "TrackRecordNodeList",
						},
						"__typename": "MangaType",
					},
					"__typename": "TrackRecordType",
				},
				"__typename": "BindTrackRecordPayload",
			},
		}

	case "TRACKER_UNBIND":
		var recordID int64
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if rVal, ok := input["recordId"].(float64); ok {
				recordID = int64(rVal)
			}
		}
		data = map[string]interface{}{
			"unbindTrack": map[string]interface{}{
				"trackRecord": map[string]interface{}{
					"id": recordID,
					"manga": map[string]interface{}{
						"id": 0,
						"trackRecords": map[string]interface{}{
							"totalCount": 0,
							"nodes":      []interface{}{},
							"__typename": "TrackRecordNodeList",
						},
						"__typename": "MangaType",
					},
					"__typename": "TrackRecordType",
				},
				"__typename": "UnbindTrackPayload",
			},
		}

	case "TRACKER_BIND", "TRACKER_FETCH_BIND", "TRACKER_UPDATE_BIND":
		data = map[string]interface{}{
			"bindTrack": map[string]interface{}{
				"trackRecord": map[string]interface{}{
					"id":        1,
					"trackerId": 1,
					"manga": map[string]interface{}{
						"id":         0,
						"__typename": "MangaType",
					},
					"__typename": "TrackRecordType",
				},
				"__typename": "BindTrackPayload",
			},
		}

	case "TRACKER_LOGIN_CREDENTIALS", "TRACKER_LOGIN_OAUTH", "TRACKER_LOGOUT":
		data = map[string]interface{}{
			"login": map[string]interface{}{
				"tracker": map[string]interface{}{
					"id":         1,
					"name":       "Tracker",
					"isLoggedIn": false,
					"__typename": "TrackerType",
				},
				"__typename": "LoginPayload",
			},
		}

	case "TRACKER_SEARCH":
		data = map[string]interface{}{
			"search": map[string]interface{}{
				"nodes":      []interface{}{},
				"totalCount": 0,
				"__typename": "TrackSearchNodeList",
			},
		}

	case "START_SYNC", "startSync", "KO_SYNC_LOGIN", "koSyncLogin", "KO_SYNC_LOGOUT", "koSyncLogout", "REFRESH_ACCESS_TOKEN", "refreshAccessToken", "USER_LOGIN", "userLogin", "USER_REFRESH", "userRefresh":
		data = map[string]interface{}{
			"success":    true,
			"__typename": "SyncPayload",
		}

	case "GET_MANGAS_LIBRARY", "GET_MANGAS", "GET_MANGAS_BASE", "GET_MANGAS_DUPLICATES":
		var sourceIDFilter *int64
		if cond, ok := req.Variables["condition"].(map[string]interface{}); ok {
			if sIdF, ok := cond["sourceId"].(float64); ok {
				sid := int64(sIdF)
				sourceIDFilter = &sid
			} else if sIdStr, ok := cond["sourceId"].(string); ok {
				var sid int64
				if _, err := fmt.Sscan(sIdStr, &sid); err == nil && sid > 0 {
					sourceIDFilter = &sid
				}
			}
		}
		if sourceIDFilter == nil && strings.Contains(req.Query, "sourceId:") {
			idx := strings.Index(req.Query, "sourceId:")
			sub := strings.TrimLeft(req.Query[idx+len("sourceId:"):], " \t\r\n")
			var sid int64
			if _, err := fmt.Sscan(sub, &sid); err == nil && sid > 0 {
				sourceIDFilter = &sid
			}
		}

		var inLibraryFilter *bool
		if cond, ok := req.Variables["condition"].(map[string]interface{}); ok {
			if val, ok := cond["inLibrary"].(bool); ok {
				inLibraryFilter = &val
			}
		}
		if inLibraryFilter == nil {
			if strings.Contains(req.Query, "inLibrary: false") {
				f := false
				inLibraryFilter = &f
			} else if strings.Contains(req.Query, "inLibrary: true") {
				t := true
				inLibraryFilter = &t
			} else if sourceIDFilter == nil {
				t := true
				inLibraryFilter = &t
			}
		}

		var categoryIds []int64
		var titleSearch string

		// 1. Try parsing from variables condition
		if cond, ok := req.Variables["condition"].(map[string]interface{}); ok {
			if catIds, ok := cond["categoryIds"].([]interface{}); ok {
				for _, idVal := range catIds {
					if idF, ok := idVal.(float64); ok {
						categoryIds = append(categoryIds, int64(idF))
					}
				}
			} else if catIdF, ok := cond["categoryIds"].(float64); ok {
				categoryIds = append(categoryIds, int64(catIdF))
			} else if catIdF, ok := cond["categoryId"].(float64); ok {
				categoryIds = append(categoryIds, int64(catIdF))
			}
		}

		// 2. Try parsing inLibrary from inline query string
		if strings.Contains(req.Query, "inLibrary: false") {
			f := false
			inLibraryFilter = &f
		} else if strings.Contains(req.Query, "inLibrary: true") {
			t := true
			inLibraryFilter = &t
		}

		// 3. Try parsing from variables filter
		if filterMap, ok := req.Variables["filter"].(map[string]interface{}); ok {
			if inLibMap, ok := filterMap["inLibrary"].(map[string]interface{}); ok {
				if eq, ok := inLibMap["equalTo"].(bool); ok {
					inLibraryFilter = &eq
				}
			}
			if catFilter, ok := filterMap["categoryId"].(map[string]interface{}); ok {
				if isNull, ok := catFilter["isNull"].(bool); ok && isNull {
					categoryIds = []int64{0}
				}
				if inArr, ok := catFilter["in"].([]interface{}); ok {
					for _, item := range inArr {
						if idF, ok := item.(float64); ok {
							categoryIds = append(categoryIds, int64(idF))
						}
					}
				}
				if eq, ok := catFilter["equalTo"].(float64); ok {
					categoryIds = append(categoryIds, int64(eq))
				}
			}
			if titleFilter, ok := filterMap["title"].(map[string]interface{}); ok {
				for _, v := range titleFilter {
					if s, ok := v.(string); ok && s != "" {
						term := strings.Trim(s, "%")
						if term != "" {
							titleSearch = term
						}
					}
				}
			}
		}

		// 4. Try parsing categoryIds and filters from inline query string
		if len(categoryIds) == 0 {
			if strings.Contains(req.Query, "isNull: true") && strings.Contains(req.Query, "categoryId") {
				categoryIds = []int64{0}
			} else if idx := strings.Index(req.Query, "category(id:"); idx != -1 {
				var id int64
				if _, err := fmt.Sscan(strings.TrimLeft(req.Query[idx+len("category(id:"):], " \t\r\n"), &id); err == nil {
					categoryIds = []int64{id}
				}
			} else if idx := strings.Index(req.Query, "categoryIds:"); idx != -1 {
				sub := req.Query[idx:]
				if start := strings.Index(sub, "["); start != -1 {
					if end := strings.Index(sub[start:], "]"); end != -1 {
						arrStr := sub[start+1 : start+end]
						parts := strings.Split(arrStr, ",")
						for _, part := range parts {
							part = strings.TrimSpace(part)
							var id int64
							if _, err := fmt.Sscan(part, &id); err == nil {
								categoryIds = append(categoryIds, id)
							}
						}
					}
				}
			}
		}

		// 5. Try parsing order from variables
		var orderByClause string
		if orderArr, ok := req.Variables["order"].([]interface{}); ok && len(orderArr) > 0 {
			var orderClauses []string
			for _, item := range orderArr {
				if orderMap, ok := item.(map[string]interface{}); ok {
					by, _ := orderMap["by"].(string)
					byType, _ := orderMap["byType"].(string)
					dir := "ASC"
					if strings.EqualFold(byType, "DESC") {
						dir = "DESC"
					}
					switch strings.ToUpper(by) {
					case "TITLE":
						orderClauses = append(orderClauses, fmt.Sprintf("m.title %s", dir))
					case "IN_LIBRARY_AT":
						orderClauses = append(orderClauses, fmt.Sprintf("mu.in_library_at %s", dir))
					case "LAST_FETCHED_AT":
						orderClauses = append(orderClauses, fmt.Sprintf("m.last_fetched_at %s", dir))
					case "ID":
						orderClauses = append(orderClauses, fmt.Sprintf("m.id %s", dir))
					case "UNREAD", "UNREAD_COUNT", "UNREAD_CHAPTERS":
						orderClauses = append(orderClauses, fmt.Sprintf("stat.unread_count %s", dir))
					case "TOTAL_CHAPTERS", "CHAPTERS_COUNT":
						orderClauses = append(orderClauses, fmt.Sprintf("stat.chapters_count %s", dir))
					case "LAST_READ", "LAST_READ_AT":
						orderClauses = append(orderClauses, fmt.Sprintf("stat.max_last_read_at %s", dir))
					case "LATEST_UPLOAD_DATE", "DATE_UPLOAD", "LATEST_UPLOADED_CHAPTER":
						orderClauses = append(orderClauses, fmt.Sprintf("stat.max_date_upload %s", dir))
					}
				}
			}
			if len(orderClauses) > 0 {
				orderByClause = "ORDER BY " + strings.Join(orderClauses, ", ")
			}
		}

		nodes, _ := fetchMangaGraphQLNodes(c.Context(), MangaGraphQLFilter{
			InLibrary:   inLibraryFilter,
			SourceID:    sourceIDFilter,
			CategoryIDs: categoryIds,
			TitleSearch: titleSearch,
			OrderBy:     orderByClause,
		})

		data = map[string]interface{}{
			"mangas": map[string]interface{}{
				"nodes":      nodes,
				"totalCount": len(nodes),
				"pageInfo": map[string]interface{}{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "MangaNodeList",
			},
		}

	case "GET_CATEGORY_MANGAS":
		var categoryId int64
		if val, ok := req.Variables["id"].(float64); ok {
			categoryId = int64(val)
		}
		inLib := true
		nodes, _ := fetchMangaGraphQLNodes(c.Context(), MangaGraphQLFilter{
			InLibrary:   &inLib,
			CategoryIDs: []int64{categoryId},
		})

		data = map[string]interface{}{
			"category": map[string]interface{}{
				"id":                categoryId,
				"isDefaultCategory": categoryId == 0,
				"mangas": map[string]interface{}{
					"nodes":      nodes,
					"totalCount": len(nodes),
					"pageInfo": map[string]interface{}{
						"hasNextPage":     false,
						"hasPreviousPage": false,
						"startCursor":     nil,
						"endCursor":       nil,
						"__typename":      "PageInfo",
					},
					"__typename": "MangaNodeList",
				},
				"__typename": "CategoryType",
			},
		}

	case "ADD_EXTENSION_STORE", "addExtensionStore":
		var url string
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if u, ok := input["indexUrl"].(string); ok && u != "" {
				url = u
			} else if u, ok := input["url"].(string); ok {
				url = u
			}
		}

		name := "Custom Extension Store"
		badge := "REPO"
		if strings.Contains(url, "tsuruyomi") || strings.Contains(url, "devil1210") {
			name = "Tsuruyomi Native Go Extensions"
			badge = "TSURU"
		} else if strings.Contains(url, "keiyoushi") {
			name = "Keiyoushi"
			badge = "KEI"
		}

		_ = database.AddExtensionStore(c.Context(), models.ExtensionStore{
			IndexURL:   url,
			Name:       name,
			BadgeLabel: badge,
		})

		data = map[string]interface{}{
			"addExtensionStore": map[string]interface{}{
				"extensionStore": map[string]interface{}{
					"name":             name,
					"indexUrl":         url,
					"isLegacy":         false,
					"badgeLabel":       badge,
					"signingKey":       "",
					"contactWebsite":   "",
					"contactDiscord":   "",
					"extensionListUrl": nil,
					"extensions": map[string]interface{}{
						"totalCount": 6,
					},
					"__typename": "ExtensionStoreType",
				},
				"__typename": "AddExtensionStorePayload",
			},
		}

	case "REMOVE_EXTENSION_STORE", "removeExtensionStore":
		var url string
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if u, ok := input["indexUrl"].(string); ok && u != "" {
				url = u
			} else if u, ok := input["url"].(string); ok {
				url = u
			}
		}

		if url != "" {
			_ = database.RemoveExtensionStore(c.Context(), url)
		}

		data = map[string]interface{}{
			"removeExtensionStore": map[string]interface{}{
				"extensionStore": map[string]interface{}{
					"name":             "Extension Store",
					"indexUrl":         url,
					"isLegacy":         false,
					"badgeLabel":       "",
					"signingKey":       "",
					"contactWebsite":   "",
					"contactDiscord":   "",
					"extensionListUrl": nil,
					"extensions": map[string]interface{}{
						"totalCount": 0,
					},
					"__typename": "ExtensionStoreType",
				},
				"__typename": "RemoveExtensionStorePayload",
			},
		}

	case "GET_MANGA_SCREEN", "GET_MANGA_META", "GET_MANGA_READER", "GET_MANGA_TRACK_RECORDS", "GET_MANGA_CATEGORIES":
		var id int64
		if val, ok := req.Variables["id"].(float64); ok {
			id = int64(val)
		} else if s, ok := req.Variables["id"].(string); ok {
			id, _ = strconv.ParseInt(s, 10, 64)
		}
		if id == 0 {
			if idx := strings.Index(req.Query, "id:"); idx != -1 {
				sub := req.Query[idx+len("id:"):]
				_, _ = fmt.Sscan(strings.TrimSpace(sub), &id)
			}
		}

		var mangaData map[string]interface{}
		nodes, err := fetchMangaGraphQLNodes(c.Context(), MangaGraphQLFilter{
			MangaIDs: []int64{id},
		})
		if err == nil && len(nodes) > 0 {
			mangaData = nodes[0]
		} else {
			mangaData = map[string]interface{}{
				"id":           id,
				"title":        "",
				"thumbnailUrl": fmt.Sprintf("/api/v1/manga/%d/thumbnail", id),
				"genre":        []string{},
				"source": map[string]interface{}{
					"id":          "0",
					"name":        "Source",
					"displayName": "Source",
					"lang":        "all",
					"iconUrl":     "",
					"__typename":  "SourceType",
				},
				"chapters": map[string]interface{}{
					"totalCount": 0,
					"__typename": "ChapterNodeList",
				},
				"trackRecords": map[string]interface{}{
					"totalCount": 0,
					"__typename": "TrackRecordNodeList",
				},
				"categories": map[string]interface{}{
					"totalCount": 0,
					"nodes":      []interface{}{},
					"__typename": "CategoryNodeList",
				},
				"__typename": "MangaType",
			}
		}

		// Query custom metadata for this manga
		mangaMetaNodes := []map[string]interface{}{}
		if database.DB != nil && id > 0 {
			rowsMeta, err := database.DB.Query(c.Context(), "SELECT meta_key, value FROM suwayomi.mangameta WHERE manga_ref = $1 AND user_id = 1", id)
			if err == nil {
				defer rowsMeta.Close()
				for rowsMeta.Next() {
					var k, v string
					if err := rowsMeta.Scan(&k, &v); err == nil {
						mangaMetaNodes = append(mangaMetaNodes, map[string]interface{}{
							"mangaId":    id,
							"key":        k,
							"value":      v,
							"__typename": "MangaMetaType",
						})
					}
				}
			}
		}
		mangaData["meta"] = mangaMetaNodes

		data = map[string]interface{}{
			"manga": mangaData,
		}

	case "GET_CHAPTERS_MANGA", "GET_CHAPTERS_READER", "GET_MANGAS_CHAPTER_IDS_WITH_STATE":
		var mangaIDs []int64
		if cond, ok := req.Variables["condition"].(map[string]interface{}); ok {
			if val, ok := cond["mangaId"].(float64); ok {
				mangaIDs = append(mangaIDs, int64(val))
			}
		}
		if filterMap, ok := req.Variables["filter"].(map[string]interface{}); ok {
			if mFilter, ok := filterMap["mangaId"].(map[string]interface{}); ok {
				if eq, ok := mFilter["equalTo"].(float64); ok {
					mangaIDs = append(mangaIDs, int64(eq))
				}
				if inArr, ok := mFilter["in"].([]interface{}); ok {
					for _, item := range inArr {
						if idF, ok := item.(float64); ok {
							mangaIDs = append(mangaIDs, int64(idF))
						}
					}
				}
			}
		}
		if len(mangaIDs) == 0 {
			if idx := strings.Index(req.Query, "mangaId:"); idx != -1 {
				sub := req.Query[idx:]
				var id int64
				if _, err := fmt.Sscan(strings.TrimLeft(sub[len("mangaId:"):], " \t\r\n"), &id); err == nil {
					mangaIDs = append(mangaIDs, id)
				}
			}
		}

		nodes, _ := fetchChaptersForMangaIDs(c.Context(), mangaIDs)
		// If no chapters found in DB, auto-fetch if source has native handler
		if len(nodes) == 0 && len(mangaIDs) == 1 && mangaIDs[0] > 0 && database.DB != nil {
			mangaId := mangaIDs[0]
			var sourceID int64
			var mangaURL string
			if err := database.DB.QueryRow(c.Context(), "SELECT source, url FROM suwayomi.manga WHERE id = $1", mangaId).Scan(&sourceID, &mangaURL); err == nil {
				if sources.Has(sourceID) {
					log.Printf("[SOURCES] Auto-fetching chapters for manga %d (source %d, url %s)...", mangaId, sourceID, mangaURL)
					if _, err := sources.FetchAndSaveMangaChapters(c.Context(), sourceID, mangaId, mangaURL); err == nil {
						nodes, _ = fetchChaptersForMangaIDs(c.Context(), mangaIDs)
					}
				}
			}
		}

		data = map[string]interface{}{
			"chapters": map[string]interface{}{
				"nodes":      nodes,
				"totalCount": len(nodes),
				"pageInfo": map[string]interface{}{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     nil,
					"endCursor":       nil,
					"__typename":      "PageInfo",
				},
				"__typename": "ChapterNodeList",
			},
		}

	case "REFRESH_MANGA", "GET_MANGA_TO_MIGRATE_TO_FETCH":
		var mangaId int64
		fetchManga := true
		fetchChapters := true

		if val, ok := req.Variables["id"].(float64); ok {
			mangaId = int64(val)
		}
		if val, ok := req.Variables["fetchManga"].(bool); ok {
			fetchManga = val
		}
		if val, ok := req.Variables["fetchChapters"].(bool); ok {
			fetchChapters = val
		}
		if val, ok := req.Variables["migrateChapters"].(bool); ok {
			fetchChapters = val
		}
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if val, ok := input["id"].(float64); ok {
				mangaId = int64(val)
			}
			if val, ok := input["fetchManga"].(bool); ok {
				fetchManga = val
			}
			if val, ok := input["fetchChapters"].(bool); ok {
				fetchChapters = val
			}
		}

		if mangaId > 0 && database.DB != nil {
			var sourceID int64
			var mangaURL string
			if err := database.DB.QueryRow(c.Context(), "SELECT source, url FROM suwayomi.manga WHERE id = $1", mangaId).Scan(&sourceID, &mangaURL); err == nil {
				if sources.Has(sourceID) {
					if fetchChapters {
						log.Printf("[SOURCES] Refreshing chapters for manga %d (source %d, url %s)...", mangaId, sourceID, mangaURL)
						_, err := sources.FetchAndSaveMangaChapters(c.Context(), sourceID, mangaId, mangaURL)
						if err != nil {
							log.Printf("[SOURCES] Error refreshing chapters: %v", err)
						}
					}
				} else if cfg != nil && cfg.Sidecar.Enabled && cfg.Sidecar.BaseURL != "" {
					return proxyToJava(c, req, cfg)
				}
			}
		}

		payload := map[string]interface{}{
			"__typename": "FetchMangaAndChaptersPayload",
		}

		if fetchManga && mangaId > 0 {
			mNodes, _ := fetchMangaGraphQLNodes(c.Context(), MangaGraphQLFilter{
				MangaIDs: []int64{mangaId},
			})
			if len(mNodes) > 0 {
				payload["manga"] = mNodes[0]
			}
		}

		if fetchChapters && mangaId > 0 {
			cNodes, _ := fetchChapterGraphQLNodes(c.Context(), mangaId)
			payload["chapters"] = cNodes
		} else {
			payload["chapters"] = []interface{}{}
		}

		data = map[string]interface{}{
			"fetchMangaAndChapters": payload,
		}

	case "GET_CHAPTER_PAGES_FETCH":
		var chapterID int64
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if cid, ok := input["chapterId"].(float64); ok {
				chapterID = int64(cid)
			}
		}
		if chapterID == 0 {
			if idx := strings.Index(req.Query, "chapterId:"); idx != -1 {
				sub := req.Query[idx:]
				var id int64
				if _, err := fmt.Sscan(strings.TrimLeft(sub[len("chapterId:"):], " \t\r\n"), &id); err == nil {
					chapterID = id
				}
			}
		}

		if chapterID > 0 && database.DB != nil {
			var mangaID int
			var sourceOrder int
			var chapterURL string
			var mangaURL string
			var sourceID int64

			query := `
				SELECT c.manga, c.source_order, c.url, m.source, m.url
				FROM suwayomi.chapter c
				INNER JOIN suwayomi.manga m ON c.manga = m.id
				WHERE c.id = $1
			`
			err := database.DB.QueryRow(c.Context(), query, chapterID).Scan(&mangaID, &sourceOrder, &chapterURL, &sourceID, &mangaURL)
			if err == nil {
				// If native Go handler exists for this source, execute in pure Go!
				if sources.Has(sourceID) {
					pages, err := sources.FetchAndSaveChapterPages(c.Context(), sourceID, mangaID, chapterID, sourceOrder, chapterURL, mangaURL)
					if err == nil && len(pages) > 0 {
						data = map[string]interface{}{
							"fetchChapterPages": map[string]interface{}{
								"chapter": map[string]interface{}{
									"id":           chapterID,
									"pageCount":    len(pages),
									"isDownloaded": false,
									"manga": map[string]interface{}{
										"id":            mangaID,
										"downloadCount": 0,
										"__typename":    "MangaType",
									},
									"__typename": "ChapterType",
								},
								"pages":      pages,
								"__typename": "FetchChapterPagesPayload",
							},
						}
						return c.JSON(fiber.Map{"data": data})
					}
					log.Printf("[SOURCES] Native fetch failed for chapter %d: %v. Falling back to Java.", chapterID, err)
				}
			}
		}

	case "UPDATE_MANGA":
		var mangaID int64
		var inLib bool
		var inLibSet bool
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if idF, ok := input["id"].(float64); ok {
				mangaID = int64(idF)
			} else if idS, ok := input["id"].(string); ok {
				mangaID, _ = strconv.ParseInt(idS, 10, 64)
			}
			if patch, ok := input["patch"].(map[string]interface{}); ok {
				if v, ok := patch["inLibrary"].(bool); ok {
					inLib = v
					inLibSet = true
				}
			}
		}

		now := time.Now().UnixMilli()
		if database.DB != nil && mangaID > 0 && inLibSet {
			_, _ = database.DB.Exec(c.Context(), `
				INSERT INTO suwayomi.mangauser (manga, user_id, in_library, in_library_at)
				VALUES ($1, 1, $2, $3)
				ON CONFLICT (manga, user_id) DO UPDATE 
				SET in_library = $2, in_library_at = CASE WHEN $2 = true THEN $3 ELSE suwayomi.mangauser.in_library_at END
			`, mangaID, inLib, now)
		}

		var catNodes []map[string]interface{}
		if updateCat, ok := req.Variables["updateCategories"].(bool); ok && updateCat {
			if catInput, ok := req.Variables["updateCategoryInput"].(map[string]interface{}); ok {
				catNodes, _ = updateMangaCategoriesInDB(c.Context(), mangaID, catInput)
			}
		}

		res := map[string]interface{}{
			"updateManga": map[string]interface{}{
				"manga": map[string]interface{}{
					"id":          mangaID,
					"inLibrary":   inLib,
					"inLibraryAt": now,
					"__typename":  "MangaType",
				},
				"__typename": "UpdateMangaPayload",
			},
		}
		if catNodes != nil {
			res["updateMangaCategories"] = map[string]interface{}{
				"manga": map[string]interface{}{
					"id": mangaID,
					"categories": map[string]interface{}{
						"nodes":      catNodes,
						"totalCount": len(catNodes),
						"__typename": "CategoryNodeList",
					},
					"__typename": "MangaType",
				},
				"__typename": "UpdateMangaCategoriesPayload",
			}
		}
		data = res

	case "UPDATE_MANGAS":
		var mangaIDs []int64
		var inLib bool
		var inLibSet bool
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if ids, ok := input["ids"].([]interface{}); ok {
				for _, idV := range ids {
					if idF, ok := idV.(float64); ok {
						mangaIDs = append(mangaIDs, int64(idF))
					}
				}
			}
			if patch, ok := input["patch"].(map[string]interface{}); ok {
				if v, ok := patch["inLibrary"].(bool); ok {
					inLib = v
					inLibSet = true
				}
			}
		}

		now := time.Now().UnixMilli()
		mangasNodes := []map[string]interface{}{}
		if database.DB != nil && len(mangaIDs) > 0 && inLibSet {
			for _, mID := range mangaIDs {
				_, _ = database.DB.Exec(c.Context(), `
					INSERT INTO suwayomi.mangauser (manga, user_id, in_library, in_library_at)
					VALUES ($1, 1, $2, $3)
					ON CONFLICT (manga, user_id) DO UPDATE 
					SET in_library = $2, in_library_at = CASE WHEN $2 = true THEN $3 ELSE suwayomi.mangauser.in_library_at END
				`, mID, inLib, now)

				mangasNodes = append(mangasNodes, map[string]interface{}{
					"id":          mID,
					"inLibrary":   inLib,
					"inLibraryAt": now,
					"__typename":  "MangaType",
				})
			}
		}

		var mangasCatPayload []map[string]interface{}
		if updateCat, ok := req.Variables["updateCategories"].(bool); ok && updateCat {
			if catInput, ok := req.Variables["updateCategoryInput"].(map[string]interface{}); ok {
				for _, mID := range mangaIDs {
					cNodes, _ := updateMangaCategoriesInDB(c.Context(), mID, catInput)
					mangasCatPayload = append(mangasCatPayload, map[string]interface{}{
						"id": mID,
						"categories": map[string]interface{}{
							"nodes":      cNodes,
							"totalCount": len(cNodes),
							"__typename": "CategoryNodeList",
						},
						"__typename": "MangaType",
					})
				}
			}
		}

		resM := map[string]interface{}{
			"updateMangas": map[string]interface{}{
				"mangas":     mangasNodes,
				"__typename": "UpdateMangasPayload",
			},
		}
		if mangasCatPayload != nil {
			resM["updateMangasCategories"] = map[string]interface{}{
				"mangas":     mangasCatPayload,
				"__typename": "UpdateMangasCategoriesPayload",
			}
		}
		data = resM

	case "UPDATE_MANGA_CATEGORIES":
		var mangaID int64
		inputMap, _ := req.Variables["input"].(map[string]interface{})
		if inputMap != nil {
			if idF, ok := inputMap["id"].(float64); ok {
				mangaID = int64(idF)
			} else if idF, ok := inputMap["mangaId"].(float64); ok {
				mangaID = int64(idF)
			}
		}

		catNodes, _ := updateMangaCategoriesInDB(c.Context(), mangaID, inputMap)

		data = map[string]interface{}{
			"updateMangaCategories": map[string]interface{}{
				"manga": map[string]interface{}{
					"id": mangaID,
					"categories": map[string]interface{}{
						"nodes":      catNodes,
						"totalCount": len(catNodes),
						"__typename": "CategoryNodeList",
					},
					"__typename": "MangaType",
				},
				"__typename": "UpdateMangaCategoriesPayload",
			},
		}

	case "UPDATE_MANGAS_CATEGORIES":
		var mangaIDs []int64
		inputMap, _ := req.Variables["input"].(map[string]interface{})
		if inputMap != nil {
			if ids, ok := inputMap["ids"].([]interface{}); ok {
				for _, idV := range ids {
					if idF, ok := idV.(float64); ok {
						mangaIDs = append(mangaIDs, int64(idF))
					}
				}
			} else if ids, ok := inputMap["mangaIds"].([]interface{}); ok {
				for _, idV := range ids {
					if idF, ok := idV.(float64); ok {
						mangaIDs = append(mangaIDs, int64(idF))
					}
				}
			} else if idF, ok := inputMap["id"].(float64); ok {
				mangaIDs = append(mangaIDs, int64(idF))
			}
		}

		var mangasPayload []map[string]interface{}
		for _, mID := range mangaIDs {
			catNodes, _ := updateMangaCategoriesInDB(c.Context(), mID, inputMap)
			mangasPayload = append(mangasPayload, map[string]interface{}{
				"id": mID,
				"categories": map[string]interface{}{
					"nodes":      catNodes,
					"totalCount": len(catNodes),
					"__typename": "CategoryNodeList",
				},
				"__typename": "MangaType",
			})
		}
		if mangasPayload == nil {
			mangasPayload = []map[string]interface{}{}
		}

		data = map[string]interface{}{
			"updateMangasCategories": map[string]interface{}{
				"mangas":     mangasPayload,
				"__typename": "UpdateMangasCategoriesPayload",
			},
		}

	case "UPDATE_CHAPTER":
		var chapterID int64
		var patch map[string]interface{}
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if idF, ok := input["id"].(float64); ok {
				chapterID = int64(idF)
			}
			if p, ok := input["patch"].(map[string]interface{}); ok {
				patch = p
			}
		}

		var updatedChapter map[string]interface{}
		if chapterID > 0 && database.DB != nil {
			var isRead *bool
			var isBookmarked *bool
			var lastPageRead *int
			var lastReadAt *int64

			if patch != nil {
				if r, ok := patch["isRead"].(bool); ok {
					isRead = &r
				}
				if b, ok := patch["isBookmarked"].(bool); ok {
					isBookmarked = &b
				}
				if p, ok := patch["lastPageRead"].(float64); ok {
					ip := int(p)
					lastPageRead = &ip
				}
				if lr, ok := patch["lastReadAt"].(float64); ok {
					ilr := int64(lr)
					lastReadAt = &ilr
				} else if isRead != nil && *isRead {
					now := time.Now().Unix()
					lastReadAt = &now
				} else if lastPageRead != nil && *lastPageRead > 0 {
					now := time.Now().Unix()
					lastReadAt = &now
				}
			}

			var currentRead, currentBookmark bool
			var currentLPR int
			var currentLRA int64
			_ = database.DB.QueryRow(c.Context(), `
				SELECT read, bookmark, last_page_read, last_read_at
				FROM suwayomi.chapteruser
				WHERE chapter = $1 AND user_id = 1
			`, chapterID).Scan(&currentRead, &currentBookmark, &currentLPR, &currentLRA)

			newRead := currentRead
			if isRead != nil {
				newRead = *isRead
			}
			newBookmark := currentBookmark
			if isBookmarked != nil {
				newBookmark = *isBookmarked
			}
			newLPR := currentLPR
			if lastPageRead != nil {
				newLPR = *lastPageRead
			}
			newLRA := currentLRA
			if lastReadAt != nil {
				newLRA = *lastReadAt
			} else if isRead != nil && !*isRead && newLPR == 0 {
				newLRA = 0
			}

			_, _ = database.DB.Exec(c.Context(), `
				INSERT INTO suwayomi.chapteruser (chapter, user_id, read, bookmark, last_page_read, last_read_at, version, last_modified_at)
				VALUES ($1, 1, $2, $3, $4, $5, 1, $6)
				ON CONFLICT (chapter, user_id) DO UPDATE SET
					read = EXCLUDED.read,
					bookmark = EXCLUDED.bookmark,
					last_page_read = EXCLUDED.last_page_read,
					last_read_at = EXCLUDED.last_read_at,
					last_modified_at = EXCLUDED.last_modified_at
			`, chapterID, newRead, newBookmark, newLPR, newLRA, time.Now().Unix())

			var mangaID int64
			_ = database.DB.QueryRow(c.Context(), "SELECT manga FROM suwayomi.chapter WHERE id = $1", chapterID).Scan(&mangaID)

			var mObj map[string]interface{}
			if mangaID > 0 {
				mNodes, _ := fetchMangaGraphQLNodes(c.Context(), MangaGraphQLFilter{MangaIDs: []int64{mangaID}})
				if len(mNodes) > 0 {
					mObj = mNodes[0]
				}
			}

			updatedChapter = map[string]interface{}{
				"id":           chapterID,
				"isRead":       newRead,
				"isBookmarked": newBookmark,
				"lastPageRead": newLPR,
				"lastReadAt":   newLRA,
				"manga":        mObj,
				"__typename":   "ChapterType",
			}
		}

		data = map[string]interface{}{
			"updateChapter": map[string]interface{}{
				"chapter":    updatedChapter,
				"__typename": "UpdateChapterPayload",
			},
		}

	case "UPDATE_CHAPTERS":
		var chapterIDs []int64
		var patch map[string]interface{}
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if ids, ok := input["ids"].([]interface{}); ok {
				for _, idVal := range ids {
					if idF, ok := idVal.(float64); ok {
						chapterIDs = append(chapterIDs, int64(idF))
					}
				}
			} else if idF, ok := input["ids"].(float64); ok {
				chapterIDs = append(chapterIDs, int64(idF))
			}
			if p, ok := input["patch"].(map[string]interface{}); ok {
				patch = p
			}
		}

		var updatedChapters []map[string]interface{}
		if len(chapterIDs) > 0 && database.DB != nil {
			var isRead *bool
			var isBookmarked *bool
			var lastPageRead *int
			var lastReadAt *int64
			if patch != nil {
				if r, ok := patch["isRead"].(bool); ok {
					isRead = &r
				}
				if b, ok := patch["isBookmarked"].(bool); ok {
					isBookmarked = &b
				}
				if p, ok := patch["lastPageRead"].(float64); ok {
					ip := int(p)
					lastPageRead = &ip
				}
				if lr, ok := patch["lastReadAt"].(float64); ok {
					ilr := int64(lr)
					lastReadAt = &ilr
				}
			}

			now := time.Now().Unix()
			for _, cid := range chapterIDs {
				var currentRead, currentBookmark bool
				var currentLPR int
				var currentLRA int64
				_ = database.DB.QueryRow(c.Context(), `
					SELECT read, bookmark, last_page_read, last_read_at
					FROM suwayomi.chapteruser
					WHERE chapter = $1 AND user_id = 1
				`, cid).Scan(&currentRead, &currentBookmark, &currentLPR, &currentLRA)

				newRead := currentRead
				if isRead != nil {
					newRead = *isRead
				}
				newBookmark := currentBookmark
				if isBookmarked != nil {
					newBookmark = *isBookmarked
				}
				newLPR := currentLPR
				if lastPageRead != nil {
					newLPR = *lastPageRead
				}
				newLRA := currentLRA
				if lastReadAt != nil {
					newLRA = *lastReadAt
				} else if isRead != nil && *isRead {
					newLRA = now
				} else if lastPageRead != nil && *lastPageRead > 0 {
					newLRA = now
				} else if isRead != nil && !*isRead && newLPR == 0 {
					newLRA = 0
				}

				_, _ = database.DB.Exec(c.Context(), `
					INSERT INTO suwayomi.chapteruser (chapter, user_id, read, bookmark, last_page_read, last_read_at, version, last_modified_at)
					VALUES ($1, 1, $2, $3, $4, $5, 1, $6)
					ON CONFLICT (chapter, user_id) DO UPDATE SET
						read = EXCLUDED.read,
						bookmark = EXCLUDED.bookmark,
						last_page_read = EXCLUDED.last_page_read,
						last_read_at = EXCLUDED.last_read_at,
						last_modified_at = EXCLUDED.last_modified_at
				`, cid, newRead, newBookmark, newLPR, newLRA, now)

				var chMangaID int64
				_ = database.DB.QueryRow(c.Context(), "SELECT manga FROM suwayomi.chapter WHERE id = $1", cid).Scan(&chMangaID)

				updatedChapters = append(updatedChapters, map[string]interface{}{
					"id":           cid,
					"isRead":       newRead,
					"isBookmarked": newBookmark,
					"lastReadAt":   newLRA,
					"lastPageRead": newLPR,
					"manga": map[string]interface{}{
						"id":                 chMangaID,
						"unreadCount":        0,
						"lastReadChapter":    map[string]interface{}{"id": cid, "lastPageRead": newLPR, "lastReadAt": newLRA, "__typename": "ChapterType"},
						"latestReadChapter":  map[string]interface{}{"id": cid, "lastPageRead": newLPR, "lastReadAt": newLRA, "__typename": "ChapterType"},
						"firstUnreadChapter": nil,
						"__typename":         "MangaType",
					},
					"__typename": "ChapterType",
				})
			}
		}

		data = map[string]interface{}{
			"updateChapters": map[string]interface{}{
				"chapters":   updatedChapters,
				"__typename": "UpdateChaptersPayload",
			},
		}

	case "UPDATE_CATEGORY":
		var catID int
		var patch map[string]interface{}
		if inputMap, ok := req.Variables["input"].(map[string]interface{}); ok {
			if idF, ok := inputMap["id"].(float64); ok {
				catID = int(idF)
			}
			if p, ok := inputMap["patch"].(map[string]interface{}); ok {
				patch = p
			} else {
				patch = inputMap
			}
		}

		if catID >= 0 && patch != nil && database.DB != nil {
			var namePtr *string
			var defaultPtr *bool
			var includeInUpdatePtr *int
			var includeInDownloadPtr *int

			if n, ok := patch["name"].(string); ok && n != "" {
				namePtr = &n
			}
			if d, ok := patch["default"].(bool); ok {
				defaultPtr = &d
			}
			if u, ok := patch["includeInUpdate"]; ok {
				status := int(models.ParseIncludeOrExclude(u))
				includeInUpdatePtr = &status
			}
			if d, ok := patch["includeInDownload"]; ok {
				status := int(models.ParseIncludeOrExclude(d))
				includeInDownloadPtr = &status
			}

			_ = database.UpdateCategory(c.Context(), catID, namePtr, defaultPtr, includeInUpdatePtr, includeInDownloadPtr)
		}

		catObj := map[string]interface{}{
			"id":                catID,
			"name":              "",
			"order":             0,
			"default":           false,
			"isDefaultCategory": catID == 0,
			"includeInUpdate":   "UNSET",
			"includeInDownload": "UNSET",
			"__typename":        "CategoryType",
		}
		if cat, err := database.GetCategoryByID(c.Context(), catID); err == nil && cat != nil {
			catObj["name"] = cat.Name
			catObj["order"] = cat.Order
			catObj["default"] = cat.Default
			catObj["includeInUpdate"] = cat.IncludeInUpdate.GraphQLEnum()
			catObj["includeInDownload"] = cat.IncludeInDownload.GraphQLEnum()
		}

		data = map[string]interface{}{
			"updateCategory": map[string]interface{}{
				"category":   catObj,
				"__typename": "UpdateCategoryPayload",
			},
		}

	case "UPDATE_CATEGORIES":
		var catIDs []int
		var patch map[string]interface{}
		if inputMap, ok := req.Variables["input"].(map[string]interface{}); ok {
			if ids, ok := inputMap["ids"].([]interface{}); ok {
				for _, idV := range ids {
					if idF, ok := idV.(float64); ok {
						catIDs = append(catIDs, int(idF))
					}
				}
			}
			if p, ok := inputMap["patch"].(map[string]interface{}); ok {
				patch = p
			} else {
				patch = inputMap
			}
		}

		var catPayload []map[string]interface{}
		if len(catIDs) > 0 && patch != nil && database.DB != nil {
			var namePtr *string
			var defaultPtr *bool
			var includeInUpdatePtr *int
			var includeInDownloadPtr *int

			if n, ok := patch["name"].(string); ok && n != "" {
				namePtr = &n
			}
			if d, ok := patch["default"].(bool); ok {
				defaultPtr = &d
			}
			if u, ok := patch["includeInUpdate"]; ok {
				status := int(models.ParseIncludeOrExclude(u))
				includeInUpdatePtr = &status
			}
			if d, ok := patch["includeInDownload"]; ok {
				status := int(models.ParseIncludeOrExclude(d))
				includeInDownloadPtr = &status
			}

			for _, cid := range catIDs {
				_ = database.UpdateCategory(c.Context(), cid, namePtr, defaultPtr, includeInUpdatePtr, includeInDownloadPtr)
				if cat, err := database.GetCategoryByID(c.Context(), cid); err == nil && cat != nil {
					catPayload = append(catPayload, map[string]interface{}{
						"id":                cat.ID,
						"name":              cat.Name,
						"order":             cat.Order,
						"default":           cat.Default,
						"isDefaultCategory": cat.ID == 0,
						"includeInUpdate":   cat.IncludeInUpdate.GraphQLEnum(),
						"includeInDownload": cat.IncludeInDownload.GraphQLEnum(),
						"__typename":        "CategoryType",
					})
				}
			}
		}
		if catPayload == nil {
			catPayload = []map[string]interface{}{}
		}

		data = map[string]interface{}{
			"updateCategories": map[string]interface{}{
				"categories": catPayload,
				"__typename": "UpdateCategoriesPayload",
			},
		}

	case "CREATE_CATEGORY":
		var catName string
		if inputMap, ok := req.Variables["input"].(map[string]interface{}); ok {
			if n, ok := inputMap["name"].(string); ok {
				catName = n
			} else if p, ok := inputMap["patch"].(map[string]interface{}); ok {
				catName, _ = p["name"].(string)
			}
		}

		var newID int
		if catName != "" && database.DB != nil {
			newID, _ = database.CreateCategory(c.Context(), catName)
		}

		catObj := map[string]interface{}{
			"id":                newID,
			"name":              catName,
			"order":             0,
			"default":           false,
			"isDefaultCategory": newID == 0,
			"includeInUpdate":   "UNSET",
			"includeInDownload": "UNSET",
			"__typename":        "CategoryType",
		}
		if newID > 0 {
			if cat, err := database.GetCategoryByID(c.Context(), newID); err == nil && cat != nil {
				catObj["name"] = cat.Name
				catObj["order"] = cat.Order
				catObj["default"] = cat.Default
				catObj["includeInUpdate"] = cat.IncludeInUpdate.GraphQLEnum()
				catObj["includeInDownload"] = cat.IncludeInDownload.GraphQLEnum()
			}
		}

		data = map[string]interface{}{
			"createCategory": map[string]interface{}{
				"category":   catObj,
				"__typename": "CreateCategoryPayload",
			},
		}

	case "DELETE_CATEGORY":
		var catID int
		if inputMap, ok := req.Variables["input"].(map[string]interface{}); ok {
			if cIdF, ok := inputMap["categoryId"].(float64); ok {
				catID = int(cIdF)
			} else if idF, ok := inputMap["id"].(float64); ok {
				catID = int(idF)
			}
		}

		if catID > 0 && database.DB != nil {
			_ = database.DeleteCategory(c.Context(), catID)
		}

		data = map[string]interface{}{
			"deleteCategory": map[string]interface{}{
				"category": map[string]interface{}{
					"id":                catID,
					"isDefaultCategory": catID == 0,
					"__typename":        "CategoryType",
				},
				"__typename": "DeleteCategoryPayload",
			},
		}

	case "UPDATE_CATEGORY_ORDER":
		if inputMap, ok := req.Variables["input"].(map[string]interface{}); ok && database.DB != nil {
			var catID, pos int
			if idF, ok := inputMap["id"].(float64); ok {
				catID = int(idF)
			}
			if pF, ok := inputMap["position"].(float64); ok {
				pos = int(pF)
			} else if oF, ok := inputMap["order"].(float64); ok {
				pos = int(oF)
			}

			// If from & to provided directly
			if fromF, ok := inputMap["from"].(float64); ok {
				if toF, ok := inputMap["to"].(float64); ok {
					_ = database.ReorderCategory(c.Context(), int(fromF), int(toF))
				}
			} else if catID > 0 {
				cats, _ := database.GetCategoryList(c.Context())
				currentFrom := -1
				for _, cItem := range cats {
					if cItem.ID == catID {
						currentFrom = cItem.Order
						break
					}
				}
				if currentFrom > 0 && pos > 0 && currentFrom != pos {
					_ = database.ReorderCategory(c.Context(), currentFrom, pos)
				}
			}
		}

		var catOrderPayload []map[string]interface{}
		cats, _ := database.GetCategoryList(c.Context())
		for _, cat := range cats {
			catOrderPayload = append(catOrderPayload, map[string]interface{}{
				"id":                cat.ID,
				"order":             cat.Order,
				"isDefaultCategory": cat.ID == 0,
				"__typename":        "CategoryType",
			})
		}
		if catOrderPayload == nil {
			catOrderPayload = []map[string]interface{}{}
		}

		data = map[string]interface{}{
			"updateCategoryOrder": map[string]interface{}{
				"categories": catOrderPayload,
				"__typename": "UpdateCategoryOrderPayload",
			},
		}

	case "DELETE_DOWNLOADED_CHAPTER", "DELETE_DOWNLOADED_CHAPTERS", "deleteDownloadedChapter", "deleteDownloadedChapters":
		var chapterIDs []int64
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if ids, ok := input["ids"].([]interface{}); ok {
				for _, idVal := range ids {
					if idF, ok := idVal.(float64); ok {
						chapterIDs = append(chapterIDs, int64(idF))
					}
				}
			} else if idF, ok := input["ids"].(float64); ok {
				chapterIDs = append(chapterIDs, int64(idF))
			} else if idF, ok := input["id"].(float64); ok {
				chapterIDs = append(chapterIDs, int64(idF))
			} else if idF, ok := input["chapterId"].(float64); ok {
				chapterIDs = append(chapterIDs, int64(idF))
			}
		}

		deletedChapters := []map[string]interface{}{}
		if len(chapterIDs) > 0 && database.DB != nil {
			for _, cid := range chapterIDs {
				var chMangaID int64
				var chName, mTitle, sourceName sql.NullString
				var scanlator sql.NullString
				query := `
					SELECT c.manga, c.name, c.scanlator, m.title, s.name
					FROM suwayomi.chapter c
					JOIN suwayomi.manga m ON c.manga = m.id
					LEFT JOIN suwayomi.source s ON m.source = s.id
					WHERE c.id = $1`
				_ = database.DB.QueryRow(c.Context(), query, cid).Scan(&chMangaID, &chName, &scanlator, &mTitle, &sourceName)

				if cfg != nil && chName.Valid && mTitle.Valid && sourceName.Valid {
					cbzPath := util.GetChapterPath(cfg, sourceName.String, mTitle.String, chName.String, scanlator.String) + ".cbz"
					_ = os.Remove(cbzPath)
					// Also clean cached pages folder for this chapter
					cacheDir := filepath.Join(cfg.Data.RootDir, "cache", "pages", fmt.Sprintf("%d", chMangaID), fmt.Sprintf("ch_%d", cid))
					_ = os.RemoveAll(cacheDir)
				}

				deletedChapters = append(deletedChapters, map[string]interface{}{
					"id":           cid,
					"isDownloaded": false,
					"manga": map[string]interface{}{
						"id":            chMangaID,
						"downloadCount": 0,
						"__typename":    "MangaType",
					},
					"__typename": "ChapterType",
				})
			}
			_, _ = database.DB.Exec(c.Context(), "UPDATE suwayomi.chapteruser SET is_downloaded = false WHERE chapter = ANY($1)", chapterIDs)
			_, _ = database.DB.Exec(c.Context(), "UPDATE suwayomi.chapter SET is_downloaded = false WHERE id = ANY($1)", chapterIDs)
		}

		data = map[string]interface{}{
			"deleteDownloadedChapters": map[string]interface{}{
				"chapters":   deletedChapters,
				"__typename": "DeleteDownloadedChaptersPayload",
			},
			"deleteDownloadedChapter": map[string]interface{}{
				"chapters":   deletedChapters,
				"__typename": "DeleteDownloadedChapterPayload",
			},
		}

	case "GET_SOURCE_BROWSE", "GET_SOURCE_SETTINGS":
		var sourceID int64
		if sID, ok := req.Variables["id"].(string); ok {
			_, _ = fmt.Sscan(sID, &sourceID)
		} else if idF, ok := req.Variables["id"].(float64); ok {
			sourceID = int64(idF)
		}

		var srcData map[string]interface{}
		if sourceID > 0 {
			src, err := database.GetSourceByID(c.Context(), sourceID)
			if err == nil && src != nil {
				iconURL := "/api/v1/source/" + src.ID + "/icon"
				srcData = map[string]interface{}{
					"id":             src.ID,
					"name":           src.Name,
					"displayName":    src.DisplayName,
					"lang":           src.Lang,
					"iconUrl":        iconURL,
					"homeUrl":        "",
					"isConfigurable": false,
					"supportsLatest": src.SupportsLatest,
					"meta":           []interface{}{},
					"filters":        []interface{}{},
					"preferences":    []interface{}{},
					"__typename":     "SourceType",
				}
			}
		}

		data = map[string]interface{}{
			"source": srcData,
		}

	case "UPDATE_SOURCE_PREFERENCES", "updateSourcePreference":
		var sourceID int64
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if sID, ok := input["sourceId"].(string); ok {
				_, _ = fmt.Sscan(sID, &sourceID)
			} else if idF, ok := input["sourceId"].(float64); ok {
				sourceID = int64(idF)
			}
		}
		var srcData map[string]interface{}
		if sourceID > 0 {
			src, err := database.GetSourceByID(c.Context(), sourceID)
			if err == nil && src != nil {
				iconURL := "/api/v1/source/" + src.ID + "/icon"
				srcData = map[string]interface{}{
					"id":             src.ID,
					"name":           src.Name,
					"displayName":    src.DisplayName,
					"lang":           src.Lang,
					"iconUrl":        iconURL,
					"homeUrl":        "",
					"isConfigurable": false,
					"supportsLatest": src.SupportsLatest,
					"meta":           []interface{}{},
					"filters":        []interface{}{},
					"preferences":    []interface{}{},
					"__typename":     "SourceType",
				}
			}
		}
		data = map[string]interface{}{
			"updateSourcePreference": map[string]interface{}{
				"source":     srcData,
				"__typename": "UpdateSourcePreferencePayload",
			},
		}

	case "GET_SOURCE_MANGAS_FETCH", "GET_MIGRATION_SOURCE_MANGAS_FETCH":
		var sourceStr string
		var queryStr string
		var typeStr string = "POPULAR"
		var page int = 1
		if input, ok := req.Variables["input"].(map[string]interface{}); ok {
			if s, ok := input["source"].(string); ok {
				sourceStr = s
			} else if s, ok := input["sourceId"].(string); ok {
				sourceStr = s
			} else if f, ok := input["source"].(float64); ok {
				sourceStr = fmt.Sprintf("%.0f", f)
			} else if f, ok := input["sourceId"].(float64); ok {
				sourceStr = fmt.Sprintf("%.0f", f)
			}
			if q, ok := input["query"].(string); ok {
				queryStr = q
			}
			if t, ok := input["type"].(string); ok && t != "" {
				typeStr = t
			}
			if p, ok := input["page"].(float64); ok && p > 0 {
				page = int(p)
			}
		}

		var sourceId int64
		_, _ = fmt.Sscan(sourceStr, &sourceId)

		// Check if source has a native online scraper (MangaSearcher / PopularLister / LatestLister)
		if database.DB != nil && sourceId > 0 {
			var onlineItems []sources.MangaItem
			var hasNext bool
			var errOnline error

			if h, ok := sources.Get(sourceId); ok {
				if queryStr != "" {
					if searcher, ok := h.(sources.MangaSearcher); ok {
						onlineItems, hasNext, errOnline = searcher.SearchManga(c.Context(), queryStr, page)
					}
				} else if typeStr == "LATEST" {
					if latest, ok := h.(sources.LatestLister); ok {
						onlineItems, hasNext, errOnline = latest.GetLatestManga(c.Context(), page)
					} else if pop, ok := h.(sources.PopularLister); ok {
						onlineItems, hasNext, errOnline = pop.GetPopularManga(c.Context(), page)
					}
				} else {
					if pop, ok := h.(sources.PopularLister); ok {
						onlineItems, hasNext, errOnline = pop.GetPopularManga(c.Context(), page)
					} else if latest, ok := h.(sources.LatestLister); ok {
						onlineItems, hasNext, errOnline = latest.GetLatestManga(c.Context(), page)
					}
				}
			}

			if errOnline == nil && len(onlineItems) > 0 {
				var ids []int64
				for _, it := range onlineItems {
					if it.SourceID == 0 {
						it.SourceID = sourceId
					}
					genreStr := strings.Join(it.Genres, ", ")
					var mID int64
					errFind := database.DB.QueryRow(c.Context(), "SELECT id FROM suwayomi.manga WHERE source = $1 AND url = $2", it.SourceID, it.URL).Scan(&mID)
					if errFind == nil && mID > 0 {
						_, _ = database.DB.Exec(c.Context(), `
							UPDATE suwayomi.manga SET
								title = $1,
								thumbnail_url = COALESCE(NULLIF($2, ''), thumbnail_url),
								description = $3,
								genre = $4,
								status = $5,
								artist = $6,
								author = $7
							WHERE id = $8
						`, it.Title, it.ThumbnailURL, it.Description, genreStr, it.Status, it.Artist, it.Author, mID)
					} else {
						errInsert := database.DB.QueryRow(c.Context(), `
							INSERT INTO suwayomi.manga (source, url, title, artist, author, description, genre, status, thumbnail_url, initialized, memo)
							VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, '{}')
							RETURNING id
						`, it.SourceID, it.URL, it.Title, it.Artist, it.Author, it.Description, genreStr, it.Status, it.ThumbnailURL).Scan(&mID)
						if errInsert == nil && mID > 0 {
							_, _ = database.DB.Exec(c.Context(), `
								INSERT INTO suwayomi.mangauser (manga, user_id, in_library, in_library_at)
								VALUES ($1, 1, false, 0)
								ON CONFLICT (manga, user_id) DO NOTHING
							`, mID)
						}
					}
					if mID > 0 {
						ids = append(ids, mID)
					}
				}
				if len(ids) > 0 {
					mNodes, errNodes := fetchMangaGraphQLNodes(c.Context(), MangaGraphQLFilter{
						MangaIDs: ids,
					})
					if errNodes == nil && len(mNodes) > 0 {
						data = map[string]interface{}{
							"fetchSourceManga": map[string]interface{}{
								"hasNextPage": hasNext,
								"mangas":      mNodes,
								"__typename":  "FetchSourceMangaPayload",
							},
						}
						break
					}
				}
			}
		}

		var sourceIdPtr *int64
		if sourceId > 0 {
			sourceIdPtr = &sourceId
		}

		offset := (page - 1) * 20
		mangaNodes, err := fetchMangaGraphQLNodes(c.Context(), MangaGraphQLFilter{
			SourceID:    sourceIdPtr,
			TitleSearch: queryStr,
			Limit:       20,
			Offset:      offset,
			OrderBy:     "ORDER BY m.id DESC",
		})
		if err != nil || mangaNodes == nil {
			mangaNodes = []map[string]interface{}{}
		}

		data = map[string]interface{}{
			"fetchSourceManga": map[string]interface{}{
				"hasNextPage": len(mangaNodes) == 20,
				"mangas":      mangaNodes,
				"__typename":  "FetchSourceMangaPayload",
			},
		}

	case "CREATE_BACKUP", "createBackup":
		data = map[string]interface{}{
			"createBackup": map[string]interface{}{
				"url":        "/api/v1/backup/export/file",
				"__typename": "CreateBackupPayload",
			},
		}

	case "VALIDATE_BACKUP", "validateBackup":
		var dataBytes []byte
		if form, err := c.MultipartForm(); err == nil && form != nil {
			for _, files := range form.File {
				for _, fh := range files {
					if f, err := fh.Open(); err == nil {
						dataBytes, _ = io.ReadAll(f)
						f.Close()
						if len(dataBytes) > 0 {
							break
						}
					}
				}
				if len(dataBytes) > 0 {
					break
				}
			}
		}
		if len(dataBytes) == 0 {
			dataBytes = c.Body()
		}

		res, err := service.ValidateBackup(c.Context(), dataBytes)
		if err != nil {
			log.Printf("[BACKUP] Warning validating backup: %v", err)
			res = &service.ValidateBackupResult{
				MissingSources:  []service.MissingSourceInfo{},
				MissingTrackers: []interface{}{},
			}
		}

		missingSrcNodes := []map[string]interface{}{}
		for _, s := range res.MissingSources {
			missingSrcNodes = append(missingSrcNodes, map[string]interface{}{
				"id":         s.ID,
				"name":       s.Name,
				"__typename": "SourceType",
			})
		}

		data = map[string]interface{}{
			"validateBackup": map[string]interface{}{
				"missingSources":  missingSrcNodes,
				"missingTrackers": []interface{}{},
				"__typename":      "ValidateBackupPayload",
			},
		}

	case "RESTORE_BACKUP", "restoreBackup":
		// Read uploaded file if available in multipart form
		var restoreErr error
		if form, err := c.MultipartForm(); err == nil && form != nil {
			for _, files := range form.File {
				for _, fh := range files {
					if f, err := fh.Open(); err == nil {
						dataBytes, err := io.ReadAll(f)
						f.Close()
						if err == nil && len(dataBytes) > 0 {
							log.Printf("[BACKUP] Received backup file %q (%d bytes) via GraphQL. Restoring...", fh.Filename, len(dataBytes))
							restoreErr = service.ImportBackup(c.Context(), dataBytes)
							if restoreErr != nil {
								log.Printf("[BACKUP] Error restoring backup: %v\n", restoreErr)
							} else {
								log.Printf("[BACKUP] Successfully restored backup from %q\n", fh.Filename)
							}
							break
						}
					}
				}
			}
		}

		if restoreErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": restoreErr.Error()}},
			})
		}

		data = map[string]interface{}{
			"restoreBackup": map[string]interface{}{
				"id": "1",
				"status": map[string]interface{}{
					"state":         "COMPLETED",
					"mangaProgress": 100,
					"totalManga":    100,
					"__typename":    "BackupRestoreStatusType",
				},
				"__typename": "RestoreBackupPayload",
			},
		}

	case "GET_RESTORE_STATUS", "GET_BACKUP_RESTORE_STATUS", "restoreStatus":
		data = map[string]interface{}{
			"restoreStatus": map[string]interface{}{
				"id":            "1",
				"state":         "COMPLETED",
				"mangaProgress": 100,
				"totalManga":    100,
				"status": map[string]interface{}{
					"state":         "COMPLETED",
					"mangaProgress": 100,
					"totalManga":    100,
					"__typename":    "BackupRestoreStatusType",
				},
				"__typename": "BackupRestoreStatusType",
			},
		}

	default:
		log.Printf("[DEBUG] Hit default case for operation: %q", req.OperationName)
		if cfg != nil && cfg.Sidecar.Enabled && cfg.Sidecar.BaseURL != "" {
			return proxyToJava(c, req, cfg)
		}
		if req.OperationName == "" && req.Query == "" {
			return c.JSON(fiber.Map{"data": nil})
		}
		log.Printf("[WARN] Unhandled GraphQL operation in Go: %q", req.OperationName)
		return c.JSON(fiber.Map{"data": nil})
	}

	return c.JSON(fiber.Map{"data": data})
}

func proxyToJava(c *fiber.Ctx, req GraphQLRequest, cfg *config.Config) error {
	baseURL := "http://localhost:4567"
	if cfg != nil && cfg.Sidecar.BaseURL != "" {
		baseURL = cfg.Sidecar.BaseURL
	}
	url := strings.TrimSuffix(baseURL, "/") + "/api/graphql"
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	reqJava, err := http.NewRequestWithContext(c.Context(), "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	reqJava.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	respJava, err := client.Do(reqJava)
	if err != nil {
		log.Printf("[ERROR] Java proxy failed: %v", err)
		return fmt.Errorf("java proxy unreachable: %w", err)
	}
	defer respJava.Body.Close()

	if respJava.StatusCode >= 400 {
		return fmt.Errorf("java proxy returned status %d", respJava.StatusCode)
	}

	var respMap map[string]interface{}
	if err := json.NewDecoder(respJava.Body).Decode(&respMap); err != nil {
		return fmt.Errorf("decoding java proxy response: %w", err)
	}

	return c.Status(respJava.StatusCode).JSON(respMap)
}

type MangaGraphQLFilter struct {
	InLibrary   *bool
	CategoryIDs []int64
	MangaIDs    []int64
	TitleSearch string
	OrderBy     string
	SourceID    *int64
	Limit       int
	Offset      int
}

func fetchMangaGraphQLNodes(ctx context.Context, filter MangaGraphQLFilter) ([]map[string]interface{}, error) {
	if database.DB == nil {
		return []map[string]interface{}{}, nil
	}

	baseQuery := `
		SELECT 
			m.id, 
			COALESCE(m.title, ''), 
			COALESCE(m.thumbnail_url, ''), 
			COALESCE(m.thumbnail_url_last_fetched, 0),
			COALESCE(mu.in_library, false),
			COALESCE(mu.in_library_at, 0),
			m.initialized,
			COALESCE(m.artist, ''), 
			COALESCE(m.author, ''), 
			COALESCE(m.description, ''), 
			COALESCE(m.genre, ''), 
			m.status,
			m.source,
			COALESCE(s.name, 'Source'),
			COALESCE(m.real_url, ''),
			COALESCE(m.url, ''),
			COALESCE(m.last_fetched_at, 0),
			COALESCE(stat.unread_count, 0),
			COALESCE(stat.download_count, 0),
			COALESCE(stat.bookmark_count, 0),
			COALESCE(stat.chapters_count, 0),
			COALESCE(stat.max_last_read_at, 0),
			COALESCE(stat.max_date_upload, 0),
			COALESCE(stat.max_fetched_at, 0),
			COALESCE(stat.max_chapter_number, 0.0),
			COALESCE(fu.id, 0),
			COALESCE(fu.source_order, 0),
			COALESCE(fu.chapter_number, 0.0),
			COALESCE(fu.name, ''),
			COALESCE(fu.scanlator, ''),
			COALESCE(fu.last_page_read, 0),
			COALESCE(lr.id, 0),
			COALESCE(lr.source_order, 0),
			COALESCE(lr.last_page_read, 0),
			COALESCE(lu.id, 0)
		FROM suwayomi.manga m
		LEFT JOIN suwayomi.mangauser mu ON m.id = mu.manga
		LEFT JOIN suwayomi.source s ON m.source = s.id
		LEFT JOIN LATERAL (
			SELECT 
				COUNT(*) as chapters_count,
				COUNT(*) FILTER (WHERE COALESCE(cu.read, false) = false) as unread_count,
				COUNT(*) FILTER (WHERE COALESCE(cu.is_downloaded, c.is_downloaded, false) = true) as download_count,
				COUNT(*) FILTER (WHERE COALESCE(cu.bookmark, false) = true) as bookmark_count,
				MAX(cu.last_read_at) as max_last_read_at,
				MAX(c.date_upload) as max_date_upload,
				MAX(c.fetched_at) as max_fetched_at,
				MAX(c.chapter_number) as max_chapter_number
			FROM suwayomi.chapter c
			LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
			WHERE c.manga = m.id
		) stat ON true
		LEFT JOIN LATERAL (
			SELECT c.id, c.source_order, c.chapter_number, c.name, COALESCE(c.scanlator, '') as scanlator,
			       COALESCE(cu.last_page_read, 0) as last_page_read
			FROM suwayomi.chapter c
			LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
			WHERE c.manga = m.id AND COALESCE(cu.read, false) = false
			ORDER BY c.source_order ASC
			LIMIT 1
		) fu ON stat.unread_count > 0
		LEFT JOIN LATERAL (
			SELECT c.id, c.source_order, COALESCE(cu.last_page_read, 0) as last_page_read
			FROM suwayomi.chapter c
			INNER JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
			WHERE c.manga = m.id AND cu.last_read_at = stat.max_last_read_at
			ORDER BY c.source_order DESC
			LIMIT 1
		) lr ON stat.max_last_read_at > 0
		LEFT JOIN LATERAL (
			SELECT c.id
			FROM suwayomi.chapter c
			WHERE c.manga = m.id AND c.date_upload = stat.max_date_upload
			LIMIT 1
		) lu ON stat.max_date_upload > 0
	`

	var whereClauses []string
	var args []interface{}

	if filter.InLibrary != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("COALESCE(mu.in_library, false) = $%d", len(args)+1))
		args = append(args, *filter.InLibrary)
	}

	if filter.SourceID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("m.source = $%d", len(args)+1))
		args = append(args, *filter.SourceID)
	}

	if len(filter.CategoryIDs) > 0 {
		if len(filter.CategoryIDs) == 1 && filter.CategoryIDs[0] == 0 {
			whereClauses = append(whereClauses, "NOT EXISTS (SELECT 1 FROM suwayomi.categorymanga cm WHERE cm.manga = m.id)")
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("EXISTS (SELECT 1 FROM suwayomi.categorymanga cm WHERE cm.manga = m.id AND cm.category = ANY($%d))", len(args)+1))
			args = append(args, filter.CategoryIDs)
		}
	}

	if len(filter.MangaIDs) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("m.id = ANY($%d)", len(args)+1))
		args = append(args, filter.MangaIDs)
	}

	if filter.TitleSearch != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("m.title ILIKE $%d", len(args)+1))
		args = append(args, "%"+filter.TitleSearch+"%")
	}

	query := baseQuery
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	if filter.OrderBy != "" {
		query += " " + filter.OrderBy
	} else {
		query += " ORDER BY m.id ASC"
	}

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		log.Printf("[ERROR] fetchMangaGraphQLNodes query error: %v", err)
		return nil, err
	}
	defer rows.Close()

	var mangaIDs []int64
	var nodes []map[string]interface{}

	for rows.Next() {
		var id int64
		var title, thumbnailUrl string
		var thumbnailUrlLastFetched int64
		var inLibraryVal bool
		var inLibraryAtVal int64
		var initialized bool
		var artist, author, description, genreStr string
		var statusInt int
		var sourceId int64
		var sourceName, realUrl, mangaUrl string
		var lastFetchedAt int64
		var unreadCount, downloadCount, bookmarkCount, chaptersCount int
		var maxLastReadAt, maxDateUpload, maxFetchedAt int64
		var maxChapterNumber float32
		var fuID, fuSourceOrder int64
		var fuChapterNumber float32
		var fuName, fuScanlator string
		var fuLastPageRead int
		var lrID, lrSourceOrder int64
		var lrLastPageRead int
		var luID int64

		err := rows.Scan(
			&id, &title, &thumbnailUrl, &thumbnailUrlLastFetched,
			&inLibraryVal, &inLibraryAtVal, &initialized,
			&artist, &author, &description, &genreStr,
			&statusInt, &sourceId, &sourceName, &realUrl, &mangaUrl, &lastFetchedAt,
			&unreadCount, &downloadCount, &bookmarkCount, &chaptersCount,
			&maxLastReadAt, &maxDateUpload, &maxFetchedAt, &maxChapterNumber,
			&fuID, &fuSourceOrder, &fuChapterNumber, &fuName, &fuScanlator, &fuLastPageRead,
			&lrID, &lrSourceOrder, &lrLastPageRead, &luID,
		)
		if err != nil {
			log.Printf("[ERROR] fetchMangaGraphQLNodes scan error: %v", err)
			continue
		}

		if realUrl == "" && mangaUrl != "" {
			realUrl = sources.GetMangaURL(sourceId, mangaUrl)
		}

		mangaIDs = append(mangaIDs, id)

		var genres []string
		if genreStr != "" {
			parts := strings.Split(genreStr, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					genres = append(genres, p)
				}
			}
		}
		if genres == nil {
			genres = []string{}
		}

		var firstUnreadChapter interface{} = nil
		if fuID > 0 {
			firstUnreadChapter = map[string]interface{}{
				"id":            fuID,
				"sourceOrder":   fuSourceOrder,
				"isRead":        false,
				"mangaId":       id,
				"chapterNumber": fuChapterNumber,
				"name":          fuName,
				"scanlator":     fuScanlator,
				"lastPageRead":  fuLastPageRead,
				"__typename":    "ChapterType",
			}
		}

		var lastReadChapter interface{} = nil
		if maxLastReadAt > 0 {
			lastReadChapter = map[string]interface{}{
				"id":           lrID,
				"sourceOrder":  lrSourceOrder,
				"lastReadAt":   maxLastReadAt,
				"lastPageRead": lrLastPageRead,
				"__typename":   "ChapterType",
			}
		}

		var latestReadChapter interface{} = nil
		if maxLastReadAt > 0 {
			latestReadChapter = map[string]interface{}{
				"id":           lrID,
				"sourceOrder":  lrSourceOrder,
				"lastReadAt":   maxLastReadAt,
				"lastPageRead": lrLastPageRead,
				"__typename":   "ChapterType",
			}
		}

		var latestUploadedChapter interface{} = nil
		if maxDateUpload > 0 {
			latestUploadedChapter = map[string]interface{}{
				"id":         luID,
				"uploadDate": maxDateUpload,
				"__typename": "ChapterType",
			}
		}

		var latestFetchedChapter interface{} = nil
		if maxFetchedAt > 0 {
			latestFetchedChapter = map[string]interface{}{
				"id":         0,
				"fetchedAt":  maxFetchedAt,
				"__typename": "ChapterType",
			}
		}

		var highestNumberedChapter interface{} = nil
		if maxChapterNumber > 0 {
			highestNumberedChapter = map[string]interface{}{
				"id":            0,
				"chapterNumber": maxChapterNumber,
				"__typename":    "ChapterType",
			}
		}

		if sourceName == "" || sourceName == "Source" {
			if sName := sources.GetSourceName(sourceId); sName != "" {
				sourceName = sName
			} else {
				sourceName = fmt.Sprintf("%d", sourceId)
			}
		}

		node := map[string]interface{}{
			"id":                      id,
			"title":                   title,
			"thumbnailUrl":            "/api/v1/manga/" + fmt.Sprintf("%d", id) + "/thumbnail",
			"thumbnailUrlLastFetched": thumbnailUrlLastFetched,
			"inLibrary":               inLibraryVal,
			"inLibraryAt":             inLibraryAtVal,
			"initialized":             initialized,
			"artist":                  artist,
			"author":                  author,
			"description":             description,
			"genre":                   genres,
			"status":                  string(models.MangaStatusFromInt(statusInt)),
			"sourceId":                fmt.Sprintf("%d", sourceId),
			"realUrl":                 realUrl,
			"lastFetchedAt":           lastFetchedAt,
			"source": map[string]interface{}{
				"id":          fmt.Sprintf("%d", sourceId),
				"name":        sourceName,
				"displayName": sourceName,
				"__typename":  "SourceType",
			},
			"unreadCount":             unreadCount,
			"downloadCount":           downloadCount,
			"bookmarkCount":           bookmarkCount,
			"hasDuplicateChapters":    false,
			"chapters": map[string]interface{}{
				"totalCount": chaptersCount,
				"__typename": "ChapterNodeList",
			},
			"categories": map[string]interface{}{
				"totalCount": 0,
				"nodes":      []interface{}{},
				"__typename": "CategoryNodeList",
			},
			"firstUnreadChapter":     firstUnreadChapter,
			"lastReadChapter":        lastReadChapter,
			"latestReadChapter":       latestReadChapter,
			"latestFetchedChapter":    latestFetchedChapter,
			"latestUploadedChapter":   latestUploadedChapter,
			"highestNumberedChapter":  highestNumberedChapter,
			"meta":                   []interface{}{},
			"trackRecords": map[string]interface{}{
				"totalCount": 0,
				"nodes":      []interface{}{},
				"__typename": "TrackRecordNodeList",
			},
			"__typename": "MangaType",
		}

		nodes = append(nodes, node)
	}

	// Batch query categories for all returned mangas
	if len(mangaIDs) > 0 {
		mangaCats := make(map[int64][]int64)
		catRows, err := database.DB.Query(ctx, "SELECT manga, category FROM suwayomi.categorymanga WHERE manga = ANY($1)", mangaIDs)
		if err == nil {
			defer catRows.Close()
			for catRows.Next() {
				var mId, cId int64
				if err := catRows.Scan(&mId, &cId); err == nil {
					mangaCats[mId] = append(mangaCats[mId], cId)
				}
			}
		}

		for _, node := range nodes {
			mId := node["id"].(int64)
			catList := mangaCats[mId]
			catNodes := make([]map[string]interface{}, 0, len(catList))
			for _, cId := range catList {
				catNodes = append(catNodes, map[string]interface{}{
					"id":                cId,
					"isDefaultCategory": cId == 0,
					"__typename":        "CategoryType",
				})
			}
			node["categories"] = map[string]interface{}{
				"totalCount": len(catNodes),
				"nodes":      catNodes,
				"__typename": "CategoryNodeList",
			}
		}

		// Batch query custom metadata for all returned mangas
		mangaMetas := make(map[int64][]map[string]interface{})
		metaRows, err := database.DB.Query(ctx, "SELECT manga_ref, meta_key, value FROM suwayomi.mangameta WHERE manga_ref = ANY($1) AND user_id = 1", mangaIDs)
		if err == nil {
			defer metaRows.Close()
			for metaRows.Next() {
				var mId int64
				var k, v string
				if err := metaRows.Scan(&mId, &k, &v); err == nil {
					mangaMetas[mId] = append(mangaMetas[mId], map[string]interface{}{
						"mangaId":    mId,
						"key":        k,
						"value":      v,
						"__typename": "MangaMetaType",
					})
				}
			}
		}

		for _, node := range nodes {
			mId := node["id"].(int64)
			if metas, ok := mangaMetas[mId]; ok {
				node["meta"] = metas
			}
		}
	}

	return nodes, nil
}

func fetchChapterGraphQLNodes(ctx context.Context, mangaId int64) ([]map[string]interface{}, error) {
	if mangaId <= 0 {
		return []map[string]interface{}{}, nil
	}
	return fetchChaptersForMangaIDs(ctx, []int64{mangaId})
}

func fetchChaptersForMangaIDs(ctx context.Context, mangaIDs []int64) ([]map[string]interface{}, error) {
	if database.DB == nil || len(mangaIDs) == 0 {
		return []map[string]interface{}{}, nil
	}

	query := `
		SELECT c.id, COALESCE(c.name, ''), COALESCE(c.scanlator, ''), COALESCE(c.real_url, ''), c.source_order, c.chapter_number, 
		       COALESCE(cu.read, false), COALESCE(cu.is_downloaded, false), COALESCE(cu.bookmark, false), c.fetched_at, c.date_upload, COALESCE(cu.last_read_at, 0),
		       COALESCE(cu.last_page_read, 0), COALESCE(c.page_count, -1), c.manga
		FROM suwayomi.chapter c
		LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
		WHERE c.manga = ANY($1)
		ORDER BY c.source_order DESC
	`
	rows, err := database.DB.Query(ctx, query, mangaIDs)
	if err != nil {
		log.Printf("[ERROR] fetchChaptersForMangaIDs query error: %v", err)
		return nil, err
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var id int64
		var name, scanlator, realUrl string
		var sourceOrder int
		var chapterNumber float32
		var read, isDownloaded, bookmark bool
		var fetchedAt, dateUpload, lastReadAt int64
		var lastPageRead, pageCount int
		var mId int64
		err := rows.Scan(
			&id, &name, &scanlator, &realUrl, &sourceOrder, &chapterNumber,
			&read, &isDownloaded, &bookmark, &fetchedAt, &dateUpload, &lastReadAt,
			&lastPageRead, &pageCount, &mId,
		)
		if err == nil {
			nodes = append(nodes, map[string]interface{}{
				"id":            id,
				"name":          name,
				"mangaId":       mId,
				"manga": map[string]interface{}{
					"id":         mId,
					"__typename": "MangaType",
				},
				"scanlator":     scanlator,
				"realUrl":       realUrl,
				"sourceOrder":   sourceOrder,
				"chapterNumber": chapterNumber,
				"isRead":        read,
				"isDownloaded":  isDownloaded,
				"isBookmarked":  bookmark,
				"fetchedAt":     fetchedAt,
				"uploadDate":    dateUpload,
				"lastReadAt":    lastReadAt,
				"lastPageRead":  lastPageRead,
				"pageCount":     pageCount,
				"meta":          []interface{}{},
				"user": map[string]interface{}{
					"isRead":        read,
					"isDownloaded":  isDownloaded,
					"isBookmarked":  bookmark,
					"lastReadAt":    lastReadAt,
					"lastPageRead":  lastPageRead,
					"__typename":    "ChapterUserType",
				},
				"__typename": "ChapterType",
			})
		}
	}
	if nodes == nil {
		nodes = []map[string]interface{}{}
	}
	return nodes, nil
}

func updateMangaCategoriesInDB(ctx context.Context, mangaID int64, inputMap map[string]interface{}) ([]map[string]interface{}, error) {
	if database.DB == nil || mangaID <= 0 {
		return []map[string]interface{}{}, nil
	}

	var patch map[string]interface{}
	if p, ok := inputMap["patch"].(map[string]interface{}); ok {
		patch = p
	} else {
		patch = inputMap
	}

	clearCategories, _ := patch["clearCategories"].(bool)
	if clearCategories {
		_, _ = database.DB.Exec(ctx, "DELETE FROM suwayomi.categorymanga WHERE manga = $1 AND user_id = 1", mangaID)
	}

	// 1. removeFromCategories
	if remList, ok := patch["removeFromCategories"].([]interface{}); ok {
		for _, rVal := range remList {
			if rIdF, ok := rVal.(float64); ok {
				rId := int64(rIdF)
				_, _ = database.DB.Exec(ctx, "DELETE FROM suwayomi.categorymanga WHERE manga = $1 AND category = $2 AND user_id = 1", mangaID, rId)
			}
		}
	}

	// 2. addToCategories
	if addList, ok := patch["addToCategories"].([]interface{}); ok {
		for _, aVal := range addList {
			if aIdF, ok := aVal.(float64); ok {
				aId := int64(aIdF)
				if aId > 0 {
					_, _ = database.DB.Exec(ctx, `
						INSERT INTO suwayomi.categorymanga (category, manga, user_id) 
						VALUES ($1, $2, 1) 
						ON CONFLICT (user_id, category, manga) DO NOTHING
					`, aId, mangaID)
				}
			}
		}
	}

	// 3. direct categoryIds (replaces all categories)
	var catIds []interface{}
	if ids, ok := patch["categoryIds"].([]interface{}); ok {
		catIds = ids
	} else if ids, ok := inputMap["categoryIds"].([]interface{}); ok {
		catIds = ids
	}

	if catIds != nil {
		_, _ = database.DB.Exec(ctx, "DELETE FROM suwayomi.categorymanga WHERE manga = $1 AND user_id = 1", mangaID)
		for _, cVal := range catIds {
			if cIdF, ok := cVal.(float64); ok {
				cId := int64(cIdF)
				if cId > 0 {
					_, _ = database.DB.Exec(ctx, `
						INSERT INTO suwayomi.categorymanga (category, manga, user_id) 
						VALUES ($1, $2, 1) 
						ON CONFLICT (user_id, category, manga) DO NOTHING
					`, cId, mangaID)
				}
			}
		}
	}

	// Query updated categories for this manga
	rows, err := database.DB.Query(ctx, `
		SELECT cm.category, COALESCE(stat.cnt, 0)
		FROM suwayomi.categorymanga cm
		LEFT JOIN (
			SELECT category, COUNT(*) as cnt 
			FROM suwayomi.categorymanga 
			WHERE user_id = 1 
			GROUP BY category
		) stat ON cm.category = stat.category
		WHERE cm.manga = $1 AND cm.user_id = 1
		ORDER BY cm.category ASC
	`, mangaID)
	if err != nil {
		return []map[string]interface{}{}, nil
	}
	defer rows.Close()

	var nodes []map[string]interface{}
	for rows.Next() {
		var cId int64
		var count int
		if err := rows.Scan(&cId, &count); err == nil {
			nodes = append(nodes, map[string]interface{}{
				"id":                cId,
				"isDefaultCategory": cId == 0,
				"mangas": map[string]interface{}{
					"totalCount": count,
					"__typename": "MangaNodeList",
				},
				"__typename": "CategoryType",
			})
		}
	}
	if nodes == nil {
		nodes = []map[string]interface{}{}
	}
	return nodes, nil
}

func fetchUpdatesOrHistoryChapters(ctx context.Context, isHistory bool, limit, offset int) ([]map[string]interface{}, int, int64, error) {
	if database.DB == nil {
		return []map[string]interface{}{}, 0, 0, nil
	}

	if limit <= 0 {
		limit = 150
	}
	if offset < 0 {
		offset = 0
	}

	var totalCount int
	var maxFetchedAt int64

	type chapterRow struct {
		id            int64
		name          string
		scanlator     string
		realUrl       string
		sourceOrder   int
		chapterNumber float32
		read          bool
		isDownloaded  bool
		bookmark      bool
		fetchedAt     int64
		dateUpload    int64
		lastReadAt    int64
		lastPageRead  int
		pageCount     int
		mangaId       int64
	}

	var rowsData []chapterRow
	var mangaIDs []int64
	seenManga := make(map[int64]bool)

	if isHistory {
		_ = database.DB.QueryRow(ctx, `
			SELECT COUNT(*) 
			FROM suwayomi.chapteruser cu 
			INNER JOIN suwayomi.chapter c ON cu.chapter = c.id
			INNER JOIN suwayomi.manga m ON c.manga = m.id
			WHERE cu.last_read_at > 0
		`).Scan(&totalCount)

		q := `
			SELECT 
				c.id, COALESCE(c.name, ''), COALESCE(c.scanlator, ''), COALESCE(c.real_url, ''), c.source_order, c.chapter_number,
				COALESCE(cu.read, false), COALESCE(cu.is_downloaded, false), COALESCE(cu.bookmark, false),
				c.fetched_at, c.date_upload, cu.last_read_at, COALESCE(cu.last_page_read, 0), COALESCE(c.page_count, -1),
				m.id
			FROM suwayomi.chapteruser cu
			INNER JOIN suwayomi.chapter c ON cu.chapter = c.id
			INNER JOIN suwayomi.manga m ON c.manga = m.id
			WHERE cu.last_read_at > 0
			ORDER BY cu.last_read_at DESC, c.source_order DESC
			LIMIT $1 OFFSET $2
		`
		r, err := database.DB.Query(ctx, q, limit, offset)
		if err != nil {
			return nil, 0, 0, err
		}
		defer r.Close()

		for r.Next() {
			var cr chapterRow
			if err := r.Scan(
				&cr.id, &cr.name, &cr.scanlator, &cr.realUrl, &cr.sourceOrder, &cr.chapterNumber,
				&cr.read, &cr.isDownloaded, &cr.bookmark,
				&cr.fetchedAt, &cr.dateUpload, &cr.lastReadAt, &cr.lastPageRead, &cr.pageCount,
				&cr.mangaId,
			); err == nil {
				rowsData = append(rowsData, cr)
				if !seenManga[cr.mangaId] {
					seenManga[cr.mangaId] = true
					mangaIDs = append(mangaIDs, cr.mangaId)
				}
			}
		}
	} else {
		// Updates: recent library chapters
		_ = database.DB.QueryRow(ctx, `
			SELECT COUNT(*), COALESCE(MAX(c.fetched_at), 0)
			FROM suwayomi.chapter c
			INNER JOIN suwayomi.manga m ON c.manga = m.id
			INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga
			WHERE COALESCE(mu.in_library, false) = true
		`).Scan(&totalCount, &maxFetchedAt)

		q := `
			SELECT 
				c.id, COALESCE(c.name, ''), COALESCE(c.scanlator, ''), COALESCE(c.real_url, ''), c.source_order, c.chapter_number,
				COALESCE(cu.read, false), COALESCE(cu.is_downloaded, false), COALESCE(cu.bookmark, false),
				c.fetched_at, c.date_upload, COALESCE(cu.last_read_at, 0), COALESCE(cu.last_page_read, 0), COALESCE(c.page_count, -1),
				m.id
			FROM suwayomi.chapter c
			INNER JOIN suwayomi.manga m ON c.manga = m.id
			INNER JOIN suwayomi.mangauser mu ON m.id = mu.manga
			LEFT JOIN suwayomi.chapteruser cu ON c.id = cu.chapter
			WHERE COALESCE(mu.in_library, false) = true
			ORDER BY c.fetched_at DESC, c.source_order DESC
			LIMIT $1 OFFSET $2
		`
		r, err := database.DB.Query(ctx, q, limit, offset)
		if err != nil {
			return nil, 0, 0, err
		}
		defer r.Close()

		for r.Next() {
			var cr chapterRow
			if err := r.Scan(
				&cr.id, &cr.name, &cr.scanlator, &cr.realUrl, &cr.sourceOrder, &cr.chapterNumber,
				&cr.read, &cr.isDownloaded, &cr.bookmark,
				&cr.fetchedAt, &cr.dateUpload, &cr.lastReadAt, &cr.lastPageRead, &cr.pageCount,
				&cr.mangaId,
			); err == nil {
				rowsData = append(rowsData, cr)
				if !seenManga[cr.mangaId] {
					seenManga[cr.mangaId] = true
					mangaIDs = append(mangaIDs, cr.mangaId)
				}
			}
		}
	}

	mangaMap := make(map[int64]map[string]interface{})
	if len(mangaIDs) > 0 {
		mNodes, _ := fetchMangaGraphQLNodes(ctx, MangaGraphQLFilter{
			MangaIDs: mangaIDs,
		})
		for _, m := range mNodes {
			if idVal, ok := m["id"].(int64); ok {
				mangaMap[idVal] = m
			}
		}
	}

	var nodes []map[string]interface{}
	for _, cr := range rowsData {
		mObj, ok := mangaMap[cr.mangaId]
		if !ok {
			mObj = map[string]interface{}{
				"id":                      cr.mangaId,
				"title":                   "Manga",
				"thumbnailUrl":            fmt.Sprintf("/api/v1/manga/%d/thumbnail", cr.mangaId),
				"thumbnailUrlLastFetched": 0,
				"sourceId":                "0",
				"inLibrary":               true,
				"initialized":             true,
				"unreadCount":             0,
				"downloadCount":           0,
				"bookmarkCount":           0,
				"hasDuplicateChapters":    false,
				"chapters": map[string]interface{}{
					"totalCount": 0,
					"__typename": "ChapterNodeList",
				},
				"__typename": "MangaType",
			}
		}

		nodes = append(nodes, map[string]interface{}{
			"id":            cr.id,
			"name":          cr.name,
			"mangaId":       cr.mangaId,
			"scanlator":     cr.scanlator,
			"realUrl":       cr.realUrl,
			"sourceOrder":   cr.sourceOrder,
			"chapterNumber": cr.chapterNumber,
			"isRead":        cr.read,
			"isDownloaded":  cr.isDownloaded,
			"isBookmarked":  cr.bookmark,
			"fetchedAt":     cr.fetchedAt,
			"uploadDate":    cr.dateUpload,
			"lastReadAt":    cr.lastReadAt,
			"lastPageRead":  cr.lastPageRead,
			"pageCount":     cr.pageCount,
			"meta":          []interface{}{},
			"user": map[string]interface{}{
				"isRead":        cr.read,
				"isDownloaded":  cr.isDownloaded,
				"isBookmarked":  cr.bookmark,
				"lastReadAt":    cr.lastReadAt,
				"lastPageRead":  cr.lastPageRead,
				"__typename":    "ChapterUserType",
			},
			"manga":      mObj,
			"__typename": "ChapterType",
		})
	}

	if nodes == nil {
		nodes = []map[string]interface{}{}
	}

	return nodes, totalCount, maxFetchedAt, nil
}


