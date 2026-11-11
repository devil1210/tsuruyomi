package router

import (
	"context"
	"log/slog"
	"net/url"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/websocket/v2"

	"tsuruyomi/internal/auth"
	"tsuruyomi/internal/config"
	"tsuruyomi/internal/extensions"
	"tsuruyomi/internal/router/handlers"
	"tsuruyomi/internal/util"
)

// Setup creates and configures the Fiber application router.
func Setup(cfg *config.Config) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "Tsuruyomi Server",
		BodyLimit:    64 * 1024 * 1024, // 64MB limit for backups/APKs
		ReadTimeout:  0,                // No timeout to support long WS connections
		WriteTimeout: 0,
		ErrorHandler: CentralErrorHandler,
	})

	// Global Middlewares
	app.Use(requestid.New(requestid.Config{
		Header:     "X-Request-ID",
		ContextKey: "requestid",
	}))

	app.Use(func(c *fiber.Ctx) error {
		reqID := c.Locals("requestid")
		reqIDStr, _ := reqID.(string)
		if reqIDStr == "" {
			reqIDStr = c.Get("X-Request-ID")
		}

		reqLogger := slog.Default().With(
			slog.String("request_id", reqIDStr),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
		)
		ctx := context.WithValue(c.UserContext(), util.ContextLoggerKey, reqLogger)
		c.SetUserContext(ctx)

		start := time.Now()
		err := c.Next()
		latency := time.Since(start)
		status := c.Response().StatusCode()

		if strings.HasPrefix(c.Path(), "/assets") || strings.HasPrefix(c.Path(), "/favicon") {
			reqLogger.Debug("HTTP asset request completed",
				slog.Int("status", status),
				slog.Duration("latency", latency),
			)
		} else {
			reqLogger.Info("HTTP request completed",
				slog.Int("status", status),
				slog.Duration("latency", latency),
			)
		}
		return err
	})

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e any) {
			reqID := c.Get("X-Request-ID")
			slog.Error("Unhandled panic intercepted in HTTP request",
				slog.String("request_id", reqID),
				slog.String("method", c.Method()),
				slog.String("path", c.Path()),
				slog.Any("panic", e),
				slog.String("stack", string(debug.Stack())),
			)
		},
	}))

	// Security Headers Middleware
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "SAMEORIGIN")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		return c.Next()
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Requested-With",
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

	// Auth check middleware
	root.Use(func(c *fiber.Ctx) error {
		c.Locals("config", cfg)

		// If auth is disabled, allow entry
		if cfg.Auth.Mode == "none" || cfg.Auth.Mode == "" {
			return c.Next()
		}

		// CORS Pre-flight checks
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}

		path := c.Path()

		// Always allow public assets
		if auth.IsPublicAsset(path) {
			return c.Next()
		}

		// 1. Validate Bearer Token or HTTP Basic Auth (WebUI, Mihon, Tachiyomi, Kotatsu, API scripts)
		authHeader := c.Get(fiber.HeaderAuthorization)
		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimSpace(authHeader[7:])
				if auth.ValidateJWT(token, cfg.Auth.Username, cfg.Auth.Password) ||
					auth.ValidateSessionToken(token, cfg.Auth.Username, cfg.Auth.Password) {
					c.Locals("user", cfg.Auth.Username)
					return c.Next()
				}
			} else if auth.CheckBasicAuth(authHeader, cfg.Auth.Username, cfg.Auth.Password) {
				c.Locals("user", cfg.Auth.Username)
				return c.Next()
			}
		}

		// 2. Validate signed session cookie (WebUI browser sessions)
		sessionCookie := c.Cookies(auth.SessionCookieName)
		if sessionCookie != "" {
			if auth.ValidateJWT(sessionCookie, cfg.Auth.Username, cfg.Auth.Password) ||
				auth.ValidateSessionToken(sessionCookie, cfg.Auth.Username, cfg.Auth.Password) {
				c.Locals("user", cfg.Auth.Username)
				return c.Next()
			}
		}

		// 3. Not authenticated
		authMode := strings.ToLower(strings.TrimSpace(cfg.Auth.Mode))
		isModernAuth := authMode == "simple_login" || authMode == "ui_login"

		// In modern login mode (simple_login or ui_login):
		// - allow GraphQL requests to pass through to HandleGraphQL so USER_LOGIN and REFRESH_ACCESS_TOKEN
		//   can execute, and unauthorized queries return the GraphQL UnauthorizedException without native browser popups.
		// - allow browser page requests (e.g. /, /history, /library, /settings, /login) to serve index.html (SPA).
		// - return 401 JSON for REST API/feed endpoints without WWW-Authenticate header.
		if isModernAuth {
			if path == "/api/graphql" || path == "/graphql" {
				return c.Next()
			}
			// For REST API or feeds, return 401 JSON without WWW-Authenticate header
			if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/opds") {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error":   "Unauthorized",
					"message": "Authentication required. Provide valid Bearer token or credentials.",
				})
			}
			// For browser page requests, let it serve index.html (SPA handles login UI)
			return c.Next()
		}

		// In basic auth mode (deprecated): challenge via WWW-Authenticate
		isAPIOrFeed := strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/opds") ||
			strings.HasPrefix(path, "/graphql")

		if isAPIOrFeed {
			c.Set(fiber.HeaderWWWAuthenticate, `Basic realm="Suwayomi"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "Unauthorized",
				"message": "Authentication required. Provide valid HTTP Basic credentials or session token.",
			})
		}

		// Browser request: if basic mode, challenge via WWW-Authenticate; otherwise redirect cleanly to /login
		if cfg.Auth.Mode == "basic" || cfg.Auth.Mode == "basic_auth" {
			c.Set(fiber.HeaderWWWAuthenticate, `Basic realm="Suwayomi"`)
			return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
		}

		cleanPrefix := strings.TrimRight(cfg.Server.Subpath, "/")
		redirectTarget := cleanPrefix + "/login?redirect=" + url.QueryEscape(path)
		if !strings.HasPrefix(redirectTarget, "/") {
			redirectTarget = "/" + redirectTarget
		}
		for strings.HasPrefix(redirectTarget, "//") {
			redirectTarget = redirectTarget[1:]
		}
		return c.Redirect(redirectTarget)
	})

	// Login & Logout routes
	root.Get("/login.html", func(c *fiber.Ctx) error {
		authMode := strings.ToLower(strings.TrimSpace(cfg.Auth.Mode))
		if authMode == "simple_login" || authMode == "ui_login" {
			redirect := c.Query("redirect", "/")
			return c.Redirect("/login?redirect=" + url.QueryEscape(redirect))
		}
		return handlers.GetLogin(c)
	})
	root.Post("/login.html", handlers.PostLogin)
	root.Get("/logout", handlers.GetLogout)
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
	api.Get("/webview/proxy", handlers.GetWebviewProxy)
	api.Get("/webview/ws", websocket.New(handlers.WebviewWS))
	api.Get("/webview", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return websocket.New(handlers.WebviewWS)(c)
		}
		return handlers.GetWebview(c)
	})

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
	api.Post("/download/reconcile", handlers.ReconcileDownloadsHandler)
	api.Get("/download/reconcile", handlers.ReconcileDownloadsHandler)

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

	// System Integrity & Auto-Healing Routes
	api.Get("/system/audit", handlers.SystemAuditHandler)
	api.Post("/system/heal", handlers.SystemHealHandler)
	api.Get("/system/heal", handlers.SystemHealHandler)

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

	serveIndexHTML := func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-cache, no-store, must-revalidate")
		c.Set(fiber.HeaderPragma, "no-cache")
		c.Set(fiber.HeaderExpires, "0")
		return c.SendFile(filepath.Join(cfg.Data.RootDir, "webUI", "index.html"))
	}
	root.Get("/", serveIndexHTML)
	root.Get("/index.html", serveIndexHTML)

	// Ensure browser always validates webUI assets against server
	root.Use("/assets", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-cache")
		return c.Next()
	})

	// Serve WebUI static files from root_dir + "/webUI"
	root.Static("/", filepath.Join(cfg.Data.RootDir, "webUI"), fiber.Static{
		Compress:  true,
		ByteRange: true,
		Index:     "index.html",
	})

	// GraphQL Endpoints (Tachidesk-WebUI Compatibility - protected under root)
	root.Get("/api/graphql/files/backup/:token", handlers.GetBackupFileByToken)
	root.All("/api/graphql", handlers.HandleGraphQL)
	root.All("/graphql", handlers.HandleGraphQL)

	// SPA Fallback: Serve index.html for all non-file, non-API requests (e.g. /library, /settings)
	root.Get("/*", func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/opds") || strings.HasPrefix(path, "/graphql") || strings.HasSuffix(path, "/update-server") || strings.Contains(path, ".") {
			return c.Next()
		}
		return serveIndexHTML(c)
	})

	return app
}
