# Native Manga Sources & Scrapers (`internal/sources`)

## Overview
The `internal/sources` package provides high-performance native Go scrapers, source handler interfaces, and chapter persistence pipelines for Tsuruyomi. It manages a thread-safe registry mapping canonical 64-bit Tachiyomi source IDs to dedicated Go handlers, coordinates automated Cloudflare challenge solving via FlareSolverr, configures dynamic SOCKS5 proxy routing, predicts future chapter release schedules via historical release intervals, and persists scraped chapter hierarchies into PostgreSQL.

## Interface
- **Registry & Configuration**:
  - `[source.go:SetConfig]`: Binds global application configuration to sources subsystem.
  - `[source.go:ConfigureProxy]`: Dynamically applies SOCKS5 proxy configuration from server settings.
  - `[source.go:Register]`, `[source.go:Unregister]`, `[source.go:RestoreNative]`, `[source.go:Get]`, `[source.go:Has]`: Registry lifecycle methods.
- **Pipeline Operations**:
  - `[source.go:FetchHTML]`, `[source.go:FetchHTMLPost]`: HTTP client with automated FlareSolverr Cloudflare bypass.
  - `[source.go:FetchAndSaveChapterPages]`: Scrapes page image URLs and records them in `suwayomi.page`.
  - `[source.go:FetchAndSaveMangaChapters]`: Scrapes all chapters for a manga and syncs to `suwayomi.chapter`.
  - `[prediction.go:PredictNextChapter]`: Computes next estimated chapter release date and day count.
- **Source Implementations**:
  - `[lectormanga.go:LectorMangaHandler]`: Native scraper for LectorManga.
  - `[lectorxd.go:LectorXDHandler]`: Native scraper for LectorXD.
  - `[madara.go:MadaraHandler]`: Generic adapter for WordPress Madara engine sites.
  - `[mangadex.go:MangaDexHandler]`: Scraper and REST API integration for MangaDex.
  - `[mangaoni.go:MangaOniHandler]`: Scraper for MangaOni.
  - `[mangathemesia.go:MangaThemesiaHandler]`: Generic adapter for MangaThemesia sites.
  - `[manhwaweb.go:ManhwaWebHandler]`: Native scraper for ManhwaWeb.
  - `[shadowmanga.go:ShadowMangaHandler]`: Scraper with NSFW filtering for ShadowManga.
  - `[submanhwa.go:SubmanhwaHandler]`: Native scraper for Submanhwa.

## Invariants
- `[source.go:registryMu]`: All registry lookups, registrations, and dynamic unregistrations are synchronized via `sync.RWMutex`.
- `[source.go:FetchAndSaveMangaChapters]`: Source chapter ordering strictly adheres to 1-based chronological ordering (`oldest = 1`, `newest = total`); inverted lists are reversed and renumbered via `ROW_NUMBER()`.
- `[prediction.go:PredictNextChapter]`: Release prediction requires `database.DB != nil`, `mangaID > 0`, at least 3 timestamp samples, and at least 3 distinct UTC calendar days.
- `[prediction.go:PredictNextChapter]`: Median gap between releases is clamped to `[1, 28]` days, ensuring non-negative countdown values.
- `[lectorxd.go, madara.go, mangadex.go, mangaoni.go, mangathemesia.go, manhwaweb.go, shadowmanga.go, submanhwa.go]`: Pagination parameters `page <= 0` are automatically normalized to `1`.
- `[shadowmanga.go]`: Series are strictly filtered by NSFW configuration flag (`it.EsMayorDeEdad == h.isNSFW`).
- `[mangadex.go:reUUID]`: Manga and chapter identifiers must match standard 36-character UUID pattern.

## Side Effects
- `[source.go:init]`: Automatically registers native scrapers into global registry during package initialization.
- `[source.go:ConfigureProxy]`: Mutates `DefaultClient.Transport` dynamically when SOCKS5 proxy settings change.
- `[source.go:FetchHTML]`: Dispatches outbound HTTP requests and invokes `util.InvalidateSession` / FlareSolverr when anti-bot challenges are detected.
- `[source.go:savePagesForChapter]`: Executes database transactions deleting stale pages and inserting new records into `suwayomi.page`.
- `[source.go:FetchAndSaveMangaChapters]`: Executes database transactions inserting or updating records in `suwayomi.chapter`, `suwayomi.chapteruser`, and `suwayomi.manga`.

## Verification
- Run: `go test ./...`
