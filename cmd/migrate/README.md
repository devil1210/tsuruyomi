# Migration CLI (`cmd/migrate`)

## Overview
The `cmd/migrate` package provides the standalone command-line entrypoint (`main.go`) for the Tsuruyomi manga migration utility. It automates migrating library manga records—specifically targeting TuMangaOnline (source ID `7925520943983324764`) or explicitly specified manga IDs—to alternative sources (such as MangaDex, MangaOni, or other installed Spanish-language scrapers). It supports dry-run previewing without database mutations, configuration fallback resolution, rate-limited sequential migrations, and printing an execution report table.

## Interface
- `[main.go:-config]`: `string` flag specifying the path to configuration file (default: `/config/config.yaml`).
- `[main.go:-ids]`: `string` flag containing comma-separated list of manga IDs to migrate (e.g. `2,3,4`).
- `[main.go:-all-tmo]`: `bool` flag to migrate all TuMangaOnline mangas in library (source ID: `7925520943983324764`).
- `[main.go:-limit]`: `int` flag to limit the number of mangas to migrate (`0` for no limit).
- `[main.go:-dry-run]`: `bool` flag to preview migration without modifying database state.

## Invariants
- `[main.go:main]`: Valid configuration must load from `-config` or standard fallbacks (`config.yaml`, `/data/config.json`); terminates with `log.Fatalf` on failure.
- `[main.go:main]`: Database connection via `database.Connect` must succeed; terminates with `log.Fatalf` on failure.
- `[main.go:-ids]`: Manga IDs passed to `-ids` must parse as positive integer values (`id > 0`) to be included in the migration queue.
- `[main.go:main]`: At least one target manga ID must be resolved (`len(targetIDs) > 0`) from `-ids` or `-all-tmo`; terminates with `log.Fatalf` on failure.
- `[main.go:-all-tmo]`: `-all-tmo` queries only manga records where `m.source = 7925520943983324764` and `mu.in_library = true`.
- `[main.go:truncate]`: If `len(s) <= max`, returns `s` unchanged; if `len(s) > max`, truncates to `max-3` characters and appends `...`.

## Side Effects
- `[main.go:main]`: Modifies global source configuration state via `sources.SetConfig(cfg)`.
- `[main.go:main]`: Reads and loads extension files from disk via `extensions.LoadInstalledExtensions(cfg.Data.RootDir)`.
- `[main.go:main]`: Establishes PostgreSQL connection via `database.Connect` and defers `database.Close()`.
- `[main.go:-all-tmo]`: Executes SQL query on PostgreSQL database via `database.DB.Query`.
- `[main.go:-dry-run]`: Mutates database state by calling `service.MigrateSingleManga` when `-dry-run` is false.
- `[main.go:-dry-run]`: Queries database and external sources via `database.DB.QueryRow` and `service.FindBestMigrationTarget` when `-dry-run` is true.
- `[main.go:main]`: Writes logs, progress messages, and markdown summary table to stdout via `fmt.Printf`, `fmt.Println`, and `log.Printf`.
- `[main.go:main]`: Terminates process with non-zero exit code via `log.Fatalf` upon configuration, DB connection, SQL, or empty target ID failures.
- `[main.go:main]`: Pauses execution thread for 200 milliseconds between iterations via `time.Sleep`.

## Verification
- Run: `go test ./...`
