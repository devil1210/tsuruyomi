package main

import (
	"context"
	"flag"
	"log"
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
)

func main() {
	// Parse cmdline flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	sources.SetConfig(cfg)

	log.Printf("Starting Tsuruyomi in log level: %s", cfg.Logging.Level)

	// Context for database connection check with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize database pool
	log.Printf("Connecting to database...")
	if err := database.Connect(ctx, cfg.Database); err != nil {
		log.Printf("Warning: Could not connect to database: %v. Server running in offline stub mode.", err)
	} else {
		log.Printf("Successfully connected to database.")
		defer database.Close()

		if err := database.EnsureSchema(ctx); err != nil {
			log.Printf("Warning: Database schema initialization error: %v", err)
		}

		// Load persisted server settings from database and overlay on config
		if dbSettings, err := database.GetAllServerSettings(ctx); err == nil && len(dbSettings) > 0 {
			cfg.ApplyServerSettings(dbSettings)
			sources.SetConfig(cfg)
			log.Printf("Loaded %d persisted server settings from database.", len(dbSettings))
		}

		log.Printf("[CONFIG] FlareSolverr enabled: %t, URL: %s, timeout: %ds", cfg.FlareSolverr.Enabled, cfg.FlareSolverr.URL, cfg.FlareSolverr.Timeout)

		// Run index optimization for library queries
		_, _ = database.DB.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_chapter_manga_read ON suwayomi.chapter (manga, read);")
	}

	// Initialize dynamic extension manifests and JS Extension Engine
	log.Printf("Loading extensions from root dir: %s", cfg.Data.RootDir)
	if err := extensions.LoadInstalledExtensions(cfg.Data.RootDir); err != nil {
		log.Printf("Warning: failed to load installed extensions: %v", err)
	}

	// Initialize download manager with config
	service.GlobalDownloadManager.SetConfig(cfg)

	// Start background periodic library update scheduler
	service.GlobalUpdateManager.StartScheduler(context.Background())

	// Initialize Fiber app setup
	app := router.Setup(cfg)

	// Graceful shutdown channel
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on %s%s", cfg.Server.Addr(), cfg.Server.Subpath)
		if err := app.Listen(cfg.Server.Addr()); err != nil {
			log.Printf("Server shut down error: %v", err)
		}
	}()

	// Wait for termination signal
	<-shutdown
	log.Println("Gracefully shutting down...")

	// Cancel any running library updates
	service.GlobalUpdateManager.CancelUpdate()

	// Shutdown fiber app with a timeout of 10s
	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		log.Printf("Fiber shutdown error: %v", err)
	}

	log.Println("Shutdown complete.")
}
