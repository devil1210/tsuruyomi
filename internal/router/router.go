package router

import (
	"encoding/base64"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/extensions"
	"tsuruyomi/internal/router/handlers"
)

// Setup creates and configures the Fiber application router.
func Setup(cfg *config.Config) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "Tsuruyomi Server",
		BodyLimit:    64 * 1024 * 1024, // 64MB limit for backups/APKs
		ReadTimeout:  0,                // No timeout to support long WS connections
		WriteTimeout: 0,
	})

	// Global Middlewares
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, HEAD, PUT, DELETE, PATCH, OPTIONS",
		AllowCredentials: false,
	}))

	// Determine prefix from subpath (if any)
	prefix := cfg.Server.Subpath
	if prefix == "" {
		prefix = "/"
	}

	// Root prefix router
	root := app.Group(prefix)

	// Auth check before matched routes (if auth is enabled)
	// For basic/simple login, we can implement it as a middleware
	root.Use(func(c *fiber.Ctx) error {
		c.Locals("config", cfg)

		// Skip auth for options, static assets like manifest/login.html
		path := c.Path()
		if c.Method() == fiber.MethodOptions ||
			strings.HasSuffix(path, "site.webmanifest") ||
			strings.HasSuffix(path, "manifest.json") ||
			strings.HasSuffix(path, "login.html") {
			return c.Next()
		}

		if cfg.Auth.Mode == "basic" {
			authHeader := c.Get(fiber.HeaderAuthorization)
			if !strings.HasPrefix(authHeader, "Basic ") {
				c.Set(fiber.HeaderWWWAuthenticate, `Basic realm="Suwayomi"`)
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			
			// Extract and decode base64
			payload, err := base64.StdEncoding.DecodeString(authHeader[6:])
			if err != nil {
				c.Set(fiber.HeaderWWWAuthenticate, `Basic realm="Suwayomi"`)
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			
			pair := strings.SplitN(string(payload), ":", 2)
			if len(pair) != 2 || pair[0] != cfg.Auth.Username || pair[1] != cfg.Auth.Password {
				c.Set(fiber.HeaderWWWAuthenticate, `Basic realm="Suwayomi"`)
				return c.SendStatus(fiber.StatusUnauthorized)
			}
		} else if cfg.Auth.Mode == "simple_login" {
			// For simple_login we check session cookie or session store.
			// Temporary placeholder: read a cookie named "logged-in"
			cookie := c.Cookies("logged-in")
			if cookie != cfg.Auth.Username && !strings.Contains(path, "/api/") {
				// Redirect to login.html if not API
				return c.Redirect(prefix + "/login.html?redirect=" + path)
			} else if cookie != cfg.Auth.Username && strings.Contains(path, "/api/") {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
		}

		return c.Next()
	})

	// Login routes (matching JavalinSetup loginPath)
	root.Get("/login.html", handlers.GetLogin)
	root.Post("/login.html", handlers.PostLogin)
	root.Get("/update-server", handlers.UpdateServer)

	// API v1 routes
	api := root.Group("/api/v1")

	// Global/Settings Routes
	api.Get("/meta", handlers.GetGlobalMeta)
	api.Patch("/meta", handlers.ModifyGlobalMeta)
	api.Get("/settings/about", handlers.GetAbout)
	api.Get("/settings/check-update", handlers.CheckUpdate)
	api.Post("/settings/update-server", handlers.PostUpdateServer)

	// Webview Routes
	api.Use("/webview", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return handlers.GetWebview(c)
	})
	api.Get("/webview", websocket.New(handlers.WebviewWS))
	api.Get("/webview/ws", websocket.New(handlers.WebviewWS))

	// Middleware to check and forward to sidecar if enabled
	sidecarMiddleware := func(c *fiber.Ctx) error {
		// NEVER forward icon requests to sidecar!
		if strings.Contains(c.Path(), "/icon") {
			return c.Next()
		}
		cfg := c.Locals("config").(*config.Config)
		if cfg.Sidecar.Enabled {
			return extensions.ForwardToSidecar(c, cfg.Sidecar.BaseURL)
		}
		return c.Next()
	}

	// Direct icon routes (always handled natively by Tsuruyomi with in-memory caching)
	api.Get("/extension/icon/:pkgName", handlers.GetExtensionIcon)
	api.Get("/source/:sourceId/icon", handlers.GetSourceIcon)

	// Extensions Routes
	extGroup := api.Group("/extension", sidecarMiddleware)
	extGroup.Get("/list", handlers.ListExtensions)
	extGroup.Get("/install/:pkgName", handlers.InstallExtension)
	extGroup.Post("/install", handlers.InstallExtensionFile)
	extGroup.Post("/install-missing", handlers.InstallMissingExtensions)
	api.Post("/extension/install-missing", handlers.InstallMissingExtensions)
	extGroup.Get("/update/:pkgName", handlers.UpdateExtension)
	extGroup.Get("/uninstall/:pkgName", handlers.UninstallExtension)

	// Sources Routes
	srcGroup := api.Group("/source", sidecarMiddleware)
	srcGroup.Get("/list", handlers.ListSources)
	srcGroup.Get("/:sourceId", handlers.GetSource)
	srcGroup.Get("/:sourceId/popular/:pageNum", handlers.GetPopularManga)
	srcGroup.Get("/:sourceId/latest/:pageNum", handlers.GetLatestManga)
	srcGroup.Get("/:sourceId/preferences", handlers.GetSourcePreferences)
	srcGroup.Post("/:sourceId/preferences", handlers.SetSourcePreference)
	srcGroup.Get("/:sourceId/filters", handlers.GetSourceFilters)
	srcGroup.Post("/:sourceId/filters", handlers.SetSourceFilters)
	srcGroup.Get("/:sourceId/search", handlers.SearchManga)
	srcGroup.Post("/:sourceId/quick-search", handlers.QuickSearchManga)

	// Manga Routes
	api.Get("/manga/:mangaId", handlers.GetManga)
	api.Get("/manga/:mangaId/full", handlers.GetMangaFull)
	api.Get("/manga/:mangaId/thumbnail", handlers.GetMangaThumbnail)
	api.Get("/manga/:mangaId/category", handlers.GetMangaCategories)
	api.Get("/manga/:mangaId/category/:categoryId", handlers.AddMangaToCategory)
	api.Delete("/manga/:mangaId/category/:categoryId", handlers.RemoveMangaFromCategory)
	api.Get("/manga/:mangaId/library", handlers.AddMangaToLibrary)
	api.Delete("/manga/:mangaId/library", handlers.RemoveMangaFromLibrary)
	api.Get("/manga/:mangaId/meta", handlers.GetMangaMeta)
	api.Patch("/manga/:mangaId/meta", handlers.ModifyMangaMeta)

	// Chapters Routes
	api.Get("/manga/:mangaId/chapters", handlers.GetChaptersList)
	api.Post("/manga/:mangaId/chapter/batch", handlers.ChapterBatchManga)
	api.Get("/manga/:mangaId/chapter/:chapterIndex", handlers.GetChapter)
	api.Patch("/manga/:mangaId/chapter/:chapterIndex", handlers.ModifyChapter)
	api.Put("/manga/:mangaId/chapter/:chapterIndex", handlers.ModifyChapter) // alias to Patch
	api.Delete("/manga/:mangaId/chapter/:chapterIndex", handlers.DeleteChapter)
	api.Patch("/manga/:mangaId/chapter/:chapterIndex/meta", handlers.ModifyChapterMeta)
	api.Get("/manga/:mangaId/chapter/:chapterIndex/page/:index", handlers.GetPageImage)

	// Chapter (cross-manga) Routes
	api.Post("/chapter/batch", handlers.ChapterBatchGlobal)
	api.Head("/chapter/:chapterId/download", handlers.DownloadChapterMetadata)
	api.Get("/chapter/:chapterId/download", handlers.DownloadChapter)

	// Category Routes
	api.Get("/category", handlers.ListCategories)
	api.Post("/category", handlers.CreateCategory)
	api.Patch("/category/reorder", handlers.ReorderCategories)
	api.Get("/category/:categoryId", handlers.GetCategoryMangas)
	api.Patch("/category/:categoryId", handlers.ModifyCategory)
	api.Delete("/category/:categoryId", handlers.DeleteCategory)
	api.Patch("/category/:categoryId/meta", handlers.ModifyCategoryMeta)

	// Downloads Routes
	api.Get("/downloads/ws", websocket.New(handlers.DownloadsWS))
	api.Get("/downloads/start", handlers.StartDownloader)
	api.Get("/downloads/stop", handlers.StopDownloader)
	api.Get("/downloads/clear", handlers.ClearDownloaderQueue)
	api.Get("/download/:mangaId/chapter/:chapterIndex", handlers.QueueChapter)
	api.Delete("/download/:mangaId/chapter/:chapterIndex", handlers.UnqueueChapter)
	api.Patch("/download/:mangaId/chapter/:chapterIndex/reorder/:to", handlers.ReorderChapterInQueue)
	api.Post("/download/batch", handlers.QueueChaptersBatch)
	api.Delete("/download/batch", handlers.UnqueueChaptersBatch)

	// Updates Routes
	api.Get("/update/recentChapters/:pageNum", handlers.GetRecentChapters)
	api.Post("/update/fetch", handlers.StartUpdater)
	api.Post("/update/reset", handlers.ResetUpdater)
	api.Get("/update/summary", handlers.GetUpdaterSummary)
	api.Get("/update/ws", websocket.New(handlers.UpdatesWS))

	// Backup Routes
	api.Post("/backup/import", handlers.ImportBackup)
	api.Post("/backup/import/file", handlers.ImportBackupFile)
	api.Post("/backup/validate", handlers.ValidateBackup)
	api.Post("/backup/validate/file", handlers.ValidateBackupFile)
	api.Get("/backup/export", handlers.ExportBackup)
	api.Get("/backup/export/file", handlers.ExportBackupFile)
	api.Get("/backup/restore/status/:id?", handlers.GetRestoreStatus)

	// Track Routes
	api.Get("/track/list", handlers.ListTrackers)
	api.Post("/track/login", handlers.TrackerLogin)
	api.Post("/track/logout", handlers.TrackerLogout)
	api.Post("/track/search", handlers.TrackerSearch)
	api.Post("/track/bind", handlers.TrackerBindManga)
	api.Post("/track/update", handlers.TrackerUpdateProgress)
	api.Get("/track/:trackerId/thumbnail", handlers.GetTrackerThumbnail)

	// OPDS v1.2 Endpoints (and aliases /opds, /opds/v1.2)
	registerOpdsRoutes := func(r fiber.Router) {
		r.Get("/", handlers.GetOpdsRoot)
		r.Get("/search", handlers.GetOpdsSearch)
		r.Get("/history", handlers.GetOpdsHistory)
		r.Get("/library-updates", handlers.GetOpdsLibraryUpdates)

		// Library Navigation
		r.Get("/library/series", handlers.GetOpdsLibrarySeries)
		r.Get("/library/categories", handlers.GetOpdsCategories)
		r.Get("/library/sources", handlers.GetOpdsSources)
		r.Get("/library/genres", handlers.GetOpdsGenres)
		r.Get("/library/statuses", handlers.GetOpdsStatuses)

		// Filtered Series Acquisition Feeds
		r.Get("/category/:categoryId", handlers.GetOpdsCategorySeries)
		r.Get("/source/:sourceId", handlers.GetOpdsSourceSeries)
		r.Get("/genre/:genre", handlers.GetOpdsGenreSeries)
		r.Get("/status/:statusId", handlers.GetOpdsStatusSeries)

		// Item Specific Feeds
		r.Get("/series/:mangaId", handlers.GetOpdsSeriesChapters)
		r.Get("/series/:mangaId/chapters", handlers.GetOpdsSeriesChapters)
	}

	opdsApi := root.Group("/api/opds/v1.2")
	registerOpdsRoutes(opdsApi)

	opdsV1 := root.Group("/opds/v1.2")
	registerOpdsRoutes(opdsV1)

	opdsRoot := root.Group("/opds")
	registerOpdsRoutes(opdsRoot)

	// OPDS 2.0 Endpoints (/api/opds/v2.0, /api/opds/2.0, /opds/v2.0, /opds/2.0)
	registerOpds2Routes := func(r fiber.Router) {
		r.Get("/", handlers.GetOpds2Root)
		r.Get("/search", handlers.GetOpds2Search)
		r.Get("/history", handlers.GetOpds2History)
		r.Get("/library-updates", handlers.GetOpds2LibraryUpdates)

		// Library Navigation
		r.Get("/library/series", handlers.GetOpds2LibrarySeries)
		r.Get("/library/categories", handlers.GetOpds2Categories)
		r.Get("/library/sources", handlers.GetOpds2Sources)
		r.Get("/library/genres", handlers.GetOpds2Genres)
		r.Get("/library/statuses", handlers.GetOpds2Statuses)

		// Filtered Series Navigation Feeds
		r.Get("/category/:categoryId", handlers.GetOpds2CategorySeries)
		r.Get("/source/:sourceId", handlers.GetOpds2SourceSeries)
		r.Get("/genre/:genre", handlers.GetOpds2GenreSeries)
		r.Get("/status/:statusId", handlers.GetOpds2StatusSeries)

		// Item Specific Publication Feeds
		r.Get("/series/:mangaId", handlers.GetOpds2SeriesChapters)
		r.Get("/series/:mangaId/chapters", handlers.GetOpds2SeriesChapters)
	}

	opds2ApiV := root.Group("/api/opds/v2.0")
	registerOpds2Routes(opds2ApiV)

	opds2Api := root.Group("/api/opds/2.0")
	registerOpds2Routes(opds2Api)

	opds2V := root.Group("/opds/v2.0")
	registerOpds2Routes(opds2V)

	opds2 := root.Group("/opds/2.0")
	registerOpds2Routes(opds2)

	// Serve WebUI static files from root_dir + "/webUI"
	root.Static("/", filepath.Join(cfg.Data.RootDir, "webUI"), fiber.Static{
		Compress:  true,
		ByteRange: true,
		Index:     "index.html",
	})

	// GraphQL Endpoint (Tachidesk-WebUI Compatibility)
	app.All("/api/graphql", handlers.HandleGraphQL)

	// SPA Fallback: Serve index.html for all non-file, non-API requests (e.g. /library, /settings)
	root.Get("/*", func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/opds") || strings.HasSuffix(path, "/update-server") || strings.Contains(path, ".") {
			return c.Next()
		}
		return c.SendFile(filepath.Join(cfg.Data.RootDir, "webUI", "index.html"))
	})

	return app
}
