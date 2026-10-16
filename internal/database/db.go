package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"tsuruyomi/internal/config"
)

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
