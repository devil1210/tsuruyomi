# Common Utilities & Subsystems (`internal/util`)

## Overview
The `internal/util` package provides shared operational utilities and subsystems across Tsuruyomi: disk space monitoring and page cache pruning, cross-platform filename sanitization, Cloudflare/FlareSolverr bypass session management, image resizing and MIME sniffing, downloaded chapter page retrieval from local directories or CBZ archives, manga thumbnail resolution, WebUI revision detection, and cached ZIP file indexing.

## Interface
- **Safe Goroutines & Structured Diagnostics**:
  - `[safego.go:SafeGo]`: Spawns asynchronous tasks with deferred panic recovery, stack trace capture, and failure callbacks.
  - `[logger.go:InitLogger]`: Configures global `slog.Default()` logger with text or JSON format, buffered stdout writes, and automated secret redaction.
  - `[logger.go:InitLoggerWithWriter]`: Configures `slog.Logger` writing to an arbitrary `io.Writer` (for testing/redirection).
  - `[logger.go:LoggerFromContext]`: Retrieves request-scoped or task-scoped `*slog.Logger` from context, falling back to `slog.Default()`.
  - `[logger.go:FlushLogger]`: Flushes buffered log stream to stdout.
- **Cache & Disk**:
  - `[cache_cleanup.go:CleanOrphanedPageCache]`: Deletes stale page cache folders or purges cache directory completely.
  - `[cache_cleanup.go:EnsureSufficientDiskSpace]`: Clears page caches if available disk space drops below threshold.
  - `[disk_linux.go:DiskFreeBytes]`, `[disk_windows.go:DiskFreeBytes]`: Cross-platform free disk space inspection.
  - `[filename.go:SanitizeFilename]`: Sanitizes string into a filesystem-safe filename across OS platforms.
- **FlareSolverr & Cloudflare Bypass**:
  - `[flaresolverr.go:EnsureSession]`: Solves anti-bot challenges and returns valid cookies and User-Agent headers.
  - `[flaresolverr.go:DownloadWithFlareSolverrAndReferer]`: Downloads URLs using solved Cloudflare session credentials.
  - `[flaresolverr.go:SolveAndFetchHTML]`: Fetches HTML pages through FlareSolverr proxy.
  - `[flaresolverr.go:InvalidateSession]`: Evicts expired or flagged challenge sessions.
- **Images & Files**:
  - `[image.go:ResizeImage]`: Bilinear image rescaling with JPEG encoding at quality 80.
  - `[image.go:DetectImageType]`: Magic header byte sniffing (`JPEG`, `PNG`, `WEBP`, `GIF`).
  - `[pages.go:GetChapterPath]`: Resolves relative or absolute filesystem directory path for downloaded chapters.
  - `[pages.go:ServePage]`: Streams a chapter page image from a `.cbz` archive or extracted folder.
  - `[thumbnail.go:GetMangaThumbnailPath]`: Resolves cached cover thumbnail path on disk.
  - `[webui.go:GetWebUIVersion]`: Detects bundled WebUI build revision number.
  - `[zip_cache.go:GetIndexedFiles]`: Thread-safe caching for CBZ file index tables.

## Invariants
- `[safego.go:SafeGo]`: Intercepts and recovers from runtime panics in worker goroutines, logging stack traces via `slog.Error`; recovers from panics occurring within the failure handler hook itself without terminating the OS process.
- `[logger.go:redactSensitiveAttrs]`: Redacts attribute values whose keys contain `password`, `secret`, `cookie`, `authorization`, `token`, or `api_key`; sanitizes string and error attribute values matching PostgreSQL DSN credentials (`postgres://user:pass@host`) or Bearer tokens.
- `[cache_cleanup.go:CleanOrphanedPageCache]`: If `maxAge <= 0`, deletes and recreates `<rootDir>/cache/pages` with `0755` permissions; if `maxAge > 0`, prunes folders older than `maxAge`.
- `[filename.go:SanitizeFilename]`: Trims leading/trailing `'.'` and `' '`; returns `'(invalid)'` if empty; replaces runes `<= 0x1f`, `0x7f`, and FAT reserved characters (`"`, `*`, `/`, `:`, `<`, `>`, `?`, `\`, `|`) with `'_'`; limits length to 240 bytes.
- `[flaresolverr.go:EnsureSession]`: Requires `cfg != nil` and `cfg.FlareSolverr.Enabled`; deduplicates concurrent solve requests per domain using `singleflight` and limits concurrency via semaphore `flareSolverrSem` (slot = 1).
- `[image.go:ResizeImage]`: Returns an error if image width or height `<= 0`; scales to `maxWidth` maintaining exact aspect ratio.
- `[pages.go:ServePage]`: Inspects `.cbz` archive before checking unpacked directory; filters out metadata files like `comicinfo.xml`.

## Side Effects
- `[safego.go:SafeGo]`: Spawns a background goroutine; emits structured `slog.Error` logs on panic.
- `[logger.go:InitLogger]`: Sets global `slog.Default()` and spawns background periodic buffer flushing goroutine.
- `[cache_cleanup.go:CleanOrphanedPageCache]`: Deletes files and directories via `os.RemoveAll` and creates cache directories via `os.MkdirAll`.
- `[flaresolverr.go:EnsureSession]`: Executes outbound HTTP POST requests to FlareSolverr service and updates in-memory `sessionCache` map under write locks.
- `[thumbnail.go:GetMangaThumbnailPath]`: Automatically removes 0-byte corrupt stub files via `os.Remove`.
- `[zip_cache.go:GetIndexedFiles]`: Reads ZIP archives from disk and caches parsed file trees in `sync.Map`.

## Verification
- Run: `go test ./...`
