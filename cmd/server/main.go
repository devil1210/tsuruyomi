package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/extensions"
	"tsuruyomi/internal/router"
	"tsuruyomi/internal/service"
	"tsuruyomi/internal/sources"
	"tsuruyomi/internal/util"
)

func main() {
	// Parse cmdline flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load configuration", slog.Any("err", err))
		os.Exit(1)
	}

	// Initialize structured logger early
	util.InitLogger(cfg.Logging)
	defer util.FlushLogger()

	sources.SetConfig(cfg)

	slog.Info("Starting Tsuruyomi", slog.String("log_level", cfg.Logging.Level), slog.String("log_format", cfg.Logging.Format))

	// Context for database connection check with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize database pool
	slog.Info("Connecting to database...")
	if err := database.Connect(ctx, cfg.Database); err != nil {
		slog.Warn("Could not connect to database, running in offline stub mode", slog.Any("err", err))
	} else {
		slog.Info("Successfully connected to database")
		defer database.Close()

		if err := database.EnsureSchema(ctx); err != nil {
			slog.Warn("Database schema initialization warning", slog.Any("err", err))
		}

		// Load persisted server settings from database and overlay on config
		if dbSettings, err := database.GetAllServerSettings(ctx); err == nil && len(dbSettings) > 0 {
			cfg.ApplyServerSettings(dbSettings)
			sources.SetConfig(cfg)
			slog.Info("Loaded persisted server settings from database", slog.Int("count", len(dbSettings)))
		}

		slog.Info("FlareSolverr configuration",
			slog.Bool("enabled", cfg.FlareSolverr.Enabled),
			slog.String("url", cfg.FlareSolverr.URL),
			slog.Int("timeout_sec", cfg.FlareSolverr.Timeout),
		)

		// Run index optimization for library queries
		_, _ = database.DB.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_chapter_manga_read ON suwayomi.chapter (manga, read);")

		// Configure network proxy if enabled in database settings
		sources.ConfigureProxy(context.Background())
	}

	// Initialize dynamic extension manifests and JS Extension Engine
	slog.Info("Loading extensions", slog.String("root_dir", cfg.Data.RootDir))
	if err := extensions.LoadInstalledExtensions(cfg.Data.RootDir); err != nil {
		slog.Warn("Failed to load installed extensions", slog.Any("err", err))
	}

	// Clean up any stale or orphaned temporary page cache on startup
	util.CleanOrphanedPageCache(cfg.Data.RootDir, 0)

	// Initialize download manager with config
	service.GlobalDownloadManager.SetConfig(cfg)

	// Start background periodic library update scheduler
	service.GlobalUpdateManager.StartScheduler(context.Background())

	// Start background automated backup scheduler
	service.StartBackupScheduler(context.Background(), cfg)

	// Start background automated integrity audit and self-healing scheduler
	service.StartHealerScheduler(context.Background())

	// Run background download storage reconciliation via SafeGo
	util.SafeGo(context.Background(), "startup-download-reconciler", func(ctx context.Context) {
		time.Sleep(2 * time.Second)
		if _, err := service.ReconcileDownloads(ctx, cfg); err != nil {
			slog.Error("Startup download reconciliation error", slog.Any("err", err))
		}
	}, nil)

	// Initialize Fiber app setup
	app := router.Setup(cfg)

	// Graceful shutdown channel
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	util.SafeGo(context.Background(), "http-server-listener", func(ctx context.Context) {
		slog.Info("Server listening", slog.String("addr", cfg.Server.Addr()), slog.String("subpath", cfg.Server.Subpath))
		if err := app.Listen(cfg.Server.Addr()); err != nil {
			slog.Error("Server listener error", slog.Any("err", err))
		}
	}, nil)

	// Wait for termination signal
	<-shutdown
	slog.Info("Gracefully shutting down...")

	// Cancel any running library updates
	service.GlobalUpdateManager.CancelUpdate()

	// Shutdown fiber app with a timeout of 10s
	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		slog.Error("Fiber shutdown error", slog.Any("err", err))
	}

	slog.Info("Shutdown complete")
}
