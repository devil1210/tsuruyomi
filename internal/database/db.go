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
	// Ensure chapter source_order is normalized without gaps or duplicate values
	_ = EnsureCleanChapterSourceOrders(ctx)

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

		// Ensure default useraccount (ID 1) exists
		_, _ = DB.Exec(ctx, `
			INSERT INTO suwayomi.useraccount (id, username, password)
			VALUES (1, 'admin', '')
			ON CONFLICT (id) DO NOTHING
		`)

		// Ensure default category (ID 0) exists
		_, _ = DB.Exec(ctx, `
			INSERT INTO suwayomi.category (id, name, sort_order, is_default, include_in_update, include_in_download, version, uid, last_modified_at, user_id, is_default_category)
			VALUES (0, 'Default', 0, true, -1, -1, 0, 0, 0, 1, true)
			ON CONFLICT (id) DO NOTHING
		`)
	}

	// Always ensure default user and default category exist
	_, _ = DB.Exec(ctx, `
		INSERT INTO suwayomi.useraccount (id, username, password)
		VALUES (1, 'admin', '')
		ON CONFLICT (id) DO NOTHING
	`)
	_, _ = DB.Exec(ctx, `
		INSERT INTO suwayomi.category (id, name, sort_order, is_default, include_in_update, include_in_download, version, uid, last_modified_at, user_id, is_default_category)
		VALUES (0, 'Default', 0, true, -1, -1, 0, 0, 0, 1, true)
		ON CONFLICT (id) DO NOTHING
	`)

	// Ensure library manga placeholder sources exist if DB was restored with mangas
	_ = EnsureNativeSources(ctx)

	// Ensure server settings table exists
	_, _ = DB.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS suwayomi.serversetting (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`)

	return nil
}

// GetServerSetting retrieves a setting value from suwayomi.serversetting.
func GetServerSetting(ctx context.Context, key string, fallback string) string {
	if DB == nil {
		return fallback
	}
	var val string
	err := DB.QueryRow(ctx, "SELECT value FROM suwayomi.serversetting WHERE key = $1", key).Scan(&val)
	if err != nil || val == "" {
		return fallback
	}
	return val
}

// SetServerSetting sets a single key-value setting in suwayomi.serversetting.
func SetServerSetting(ctx context.Context, key string, value string) error {
	if DB == nil {
		return fmt.Errorf("database not connected")
	}
	_, err := DB.Exec(ctx, `
		INSERT INTO suwayomi.serversetting (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
	`, key, value)
	return err
}

// SetServerSettings batch sets multiple key-value settings.
func SetServerSettings(ctx context.Context, settings map[string]string) error {
	if DB == nil || len(settings) == 0 {
		return nil
	}
	for k, v := range settings {
		_ = SetServerSetting(ctx, k, v)
	}
	return nil
}

// GetAllServerSettings retrieves all server settings stored in the database.
func GetAllServerSettings(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)
	if DB == nil {
		return result, nil
	}
	rows, err := DB.Query(ctx, "SELECT key, value FROM suwayomi.serversetting")
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			result[k] = v
		}
	}
	return result, nil
}

