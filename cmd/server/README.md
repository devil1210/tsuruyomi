# Server Daemon (`cmd/server`)

## Overview
The `cmd/server` package contains the primary daemon entrypoint (`main.go`) for the Tsuruyomi server application. It parses command-line flags, loads YAML configuration, initializes the PostgreSQL connection (falling back to an offline stub mode on failure), updates settings from database persistence, initializes JS extensions, cleans stale page caches, initializes background maintenance and scheduler services (download manager, periodic updates, automated backups, self-healing audits, download storage reconciliation), starts the Fiber HTTP web server, and manages graceful shutdown on `SIGINT`/`SIGTERM` signals.

## Interface
- `[main.go:main]`: Main application entrypoint function initializing all subsystems and starting the web server.
- `[main.go:-config]`: `string` flag specifying path to the configuration YAML file (default: `"config.yaml"`).

## Invariants
- `[main.go:main]`: Configuration file loading is mandatory; failures cause immediate fatal termination via `log.Fatalf`.
- `[main.go:main]`: Database connection failure is non-fatal; application continues running in offline stub mode.
- `[main.go:main]`: Schema initialization, server settings overlay, index creation, and proxy configuration only execute if database connection succeeds.
- `[main.go:main]`: Database connection check context timeout is strictly bounded to 10 seconds.
- `[main.go:main]`: Execution blocks until an `os.Interrupt` or `syscall.SIGTERM` signal is received.
- `[main.go:main]`: Fiber HTTP server shutdown timeout is strictly bounded to 10 seconds.

## Side Effects
- `[main.go:-config]`: Parses command-line flags via `flag.Parse()`.
- `[main.go:main]`: Reads configuration file from disk via `config.Load`.
- `[main.go:main]`: Connects to PostgreSQL database via `database.Connect` and defers `database.Close`.
- `[main.go:main]`: Executes schema verification and executes DDL `CREATE INDEX IF NOT EXISTS idx_chapter_manga_read`.
- `[main.go:main]`: Loads persisted server settings from database and modifies global source and proxy configurations.
- `[main.go:main]`: Reads and initializes dynamic JS extensions from `cfg.Data.RootDir`.
- `[main.go:main]`: Deletes orphaned temporary page cache files from disk via `util.CleanOrphanedPageCache`.
- `[main.go:main]`: Spawns background goroutines and schedulers for library updates, backups, healing, and download reconciliation.
- `[main.go:main]`: Binds and listens for HTTP requests on `cfg.Server.Addr()`.
- `[main.go:main]`: Listens for OS interrupt signals and terminates background updates and Fiber HTTP server on shutdown.
- `[main.go:main]`: Writes runtime operational logs to standard logger.

## Verification
- Run: `go test ./...`
