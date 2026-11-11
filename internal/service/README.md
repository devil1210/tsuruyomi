# Background Services & Operations (`internal/service`)

## Overview
The `internal/service` package orchestrates asynchronous background tasks, scheduled maintenance routines, and operational workflows for Tsuruyomi:
1. **Protobuf & Backup Management (`backup_proto.go`, `backup_service.go`)**: Handles Protobuf serialization/deserialization for Tachiyomi/Suwayomi `.tachibk` archives, automated daily backups with TTL retention pruning, and asynchronous restore execution with WebSocket progress reporting.
2. **Download Pipeline (`chapter_downloader.go`, `download_manager.go`, `downloads_reconciler.go`)**: Manages the chapter download queue with concurrent image fetching, ComicInfo metadata embedding, CBZ packaging, disk reconciliation (discovering offline files and cleaning phantom flags), and real-time state broadcasts.
3. **Extension Management (`extension_installer.go`)**: Caches remote extension repositories, manages dynamic installation/uninstallation of JS and manifest assets, and activates native Go or dynamic sources.
4. **Library Maintenance & Repair (`healer.go`)**: Runs periodic automated integrity audits and repairs for corrupted upload dates, blank chapter names, and chapter numbers.
5. **Manga Migration (`migration_service.go`)**: Searches target series across sources (MangaDex, MangaOni, Madara, MangaThemesia) and migrates reading history, categories, and cover artwork.
6. **Temporary File Storage (`temporary_file_storage.go`)**: Thread-safe ephemeral storage for exported archives and tokens with hourly expiration sweeps.
7. **Library Updates (`update_manager.go`)**: Coordinates full library chapter updates, filter evaluations (unread, status, category rules), wall-clock scheduled intervals, and auto-download queuing.

## Interface
- **Backups**:
  - `[backup_proto.go:DecodeProtoBackup]`, `[backup_proto.go:EncodeProtoBackup]`, `[backup_proto.go:ExportTachibk]`, `[backup_proto.go:RestoreFromProtoBackupWithProgress]`
  - `[backup_service.go:GlobalRestoreManager]`: `*RestoreManager` handling async backup restoration jobs.
  - `[backup_service.go:StartBackupScheduler]`: Initiates background automated backup timer.
- **Downloads**:
  - `[chapter_downloader.go:EnsureChapterCBZ]`: Fetches chapter page images, creates `ComicInfo.xml`, and packages `.cbz` archive on disk.
  - `[download_manager.go:GlobalDownloadManager]`: `*DownloadManager` queue controller.
  - `[downloads_reconciler.go:ReconcileDownloads]`: Scans disk directories and reconciles database download flags.
- **Extensions**:
  - `[extension_installer.go:SyncExtensionRepositories]`, `[extension_installer.go:InstallExtension]`, `[extension_installer.go:UninstallExtension]`, `[extension_installer.go:UpdateExtension]`
- **Healer**:
  - `[healer.go:Audit]`, `[healer.go:Heal]`, `[healer.go:StartHealerScheduler]`
- **Migration**:
  - `[migration_service.go:FindBestMigrationTarget]`, `[migration_service.go:MigrateSingleManga]`
- **Temporary Files**:
  - `[temporary_file_storage.go:GlobalTempFileStorage]`: Ephemeral tokenized file storage instance.
- **Updater**:
  - `[update_manager.go:GlobalUpdateManager]`: Library update coordinator.
  - `[update_manager.go:CalculateNextWallClockRun]`: Calculates wall-clock scheduled run times.

## Invariants
- `[backup_proto.go:RestoreFromProtoBackupWithProgress]`: Requires `database.DB != nil`; returns error `'database not initialized'` if nil.
- `[chapter_downloader.go:EnsureChapterCBZ]`: Enforces single in-flight download concurrency invariant per chapter ID via `inFlightDownloads` `sync.Map`.
- `[download_manager.go:DownloadManager.Enqueue]`: Enforces unique chapter invariant; ignores chapters already present in queue.
- `[download_manager.go:DownloadManager.workerLoop]`: Executes each download task inside `util.SafeGo`; limits download retries to 3; marks `DownloadStateError` when `Tries >= 3` or upon intercepted panic, else requeues as `DownloadStateQueued`.
- `[healer.go:Heal]`: Serial execution across goroutines is strictly enforced by `healerMu` mutex.
- `[healer.go:StartHealerScheduler]`: Executes startup and periodic 6-hour audit and healing passes inside `util.SafeGo`.
- `[backup_service.go:RestoreManager.StartAsyncRestore]`: Executes background restoration tasks inside `util.SafeGo` and marks `FAILED` status upon panic.
- `[update_manager.go:UpdateManager.StartUpdate]`: Enforces single concurrent library update invariant via `um.running` flag; rejects duplicate triggers while running; isolates individual manga update tasks inside `util.SafeGo`.

## Side Effects
- `[backup_proto.go, backup_service.go]`: Mutates PostgreSQL tables in `suwayomi.*`, writes `.tachibk` files to disk, and broadcasts WebSocket progress frames.
- `[chapter_downloader.go]`: Makes outbound HTTP calls to fetch images, writes files to disk, and builds CBZ archives.
- `[download_manager.go]`: Spawns background worker goroutines, updates queue item status, and broadcasts state to `websocket.GlobalHub`.
- `[downloads_reconciler.go]`: Updates `is_downloaded` boolean columns in `suwayomi.chapter` and `suwayomi.chapteruser`.
- `[extension_installer.go]`: Fetches remote catalogs, downloads extension scripts to disk, and updates extension records in PostgreSQL.
- `[healer.go]`: Scrapes remote sources and updates `date_upload`, `name`, and `chapter_number` in `suwayomi.chapter`.
- `[migration_service.go]`: Queries external APIs, creates new manga entries, and transfers user reading histories.
- `[temporary_file_storage.go]`: Deletes expired disk files during hourly cleanup sweeps.
- `[update_manager.go]`: Spawns background goroutines scraping source updates and enqueues new chapters for download.

## Verification
- Run: `go test ./...`
