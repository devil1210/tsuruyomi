package database

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"tsuruyomi/internal/config"
)

//go:embed schema.sql
var schemaSQL string

// DB is the global database connection pool.
// All repository functions use this pool directly.
var DB *pgxpool.Pool

// Connect initialises the PostgreSQL connection pool using the provided config.
// It also verifies the connection with a ping.
func Connect(ctx context.Context, cfg config.DatabaseConfig) error {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return fmt.Errorf("parsing DB DSN: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("creating pgx pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	DB = pool
	return nil
}

// Close gracefully shuts down the connection pool.
func Close() {
	if DB != nil {
		DB.Close()
	}
}

// EnsureSchema verifies that suwayomi schema and tables exist.
// If not, it executes the embedded schema.sql and creates the Default category.
func EnsureSchema(ctx context.Context) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	var exists bool
	err := DB.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'suwayomi' AND table_name = 'category')").Scan(&exists)
	if err != nil || !exists {
		log.Println("[DATABASE] Schema suwayomi not detected. Initializing database schema from embedded schema.sql...")
		_, err = DB.Exec(ctx, schemaSQL)
		if err != nil {
			return fmt.Errorf("executing schema.sql: %w", err)
		}
		log.Println("[DATABASE] Schema successfully created.")

		// Ensure default category (ID 0) exists
		_, _ = DB.Exec(ctx, `
			INSERT INTO suwayomi.category (id, name, sort_order, is_default, include_in_update, include_in_download, version, uid, last_modified_at)
			VALUES (0, 'Default', 0, true, -1, -1, 0, 0, 0)
			ON CONFLICT (id) DO NOTHING
		`)
	}

	return nil
}
