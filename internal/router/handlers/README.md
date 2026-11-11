# HTTP Handlers & WebSockets (`internal/router/handlers`)

## Overview
The `internal/router/handlers` package provides the complete HTTP Fiber endpoint handlers and real-time WebSocket connection controllers for Tsuruyomi. It manages automated backup imports/exports, category hierarchies, download queue scheduling, extension installs/uninstalls, global server metadata, an extensive GraphQL compatibility engine (compatible with Tachidesk-WebUI), self-healing audits/repairs, manga viewing and reader image streaming, OPDS 1.2 and OPDS 2.0 catalog feeds with PSE page streaming, source scraping, tracker integrations, and library batch updates.

## Interface
- **Backups (`backup.go`)**:
  - `[backup.go:ExportBackup]`, `[backup.go:ExportBackupFile]`: Generates and exports Tachiyomi protobuf `.tachibk` archives.
  - `[backup.go:ImportBackup]`, `[backup.go:ImportBackupFile]`: Deserializes backup payloads and triggers asynchronous database restoration.
  - `[backup.go:GetBackupFileByToken]`: Streams generated backup archives identified by temporary security tokens.
  - `[backup.go:ReconcileDownloadsHandler]`: Reconciles filesystem download storage against database records.
- **Categories (`category.go`)**:
  - `[category.go:ListCategories]`, `[category.go:CreateCategory]`, `[category.go:ModifyCategory]`, `[category.go:DeleteCategory]`, `[category.go:ReorderCategories]`, `[category.go:GetCategoryMangas]`, `[category.go:ModifyCategoryMeta]`
- **Downloads (`download.go`)**:
  - `[download.go:DownloadsWS]`: Real-time WebSocket connection broadcasting download queue updates.
  - `[download.go:StartDownloader]`, `[download.go:StopDownloader]`, `[download.go:ClearDownloaderQueue]`, `[download.go:QueueChapter]`, `[download.go:UnqueueChapter]`, `[download.go:ReorderChapterInQueue]`, `[download.go:QueueChaptersBatch]`, `[download.go:UnqueueChaptersBatch]`
- **Extensions (`extension.go`)**:
  - `[extension.go:ListExtensions]`, `[extension.go:InstallExtension]`, `[extension.go:InstallExtensionFile]`, `[extension.go:UninstallExtension]`, `[extension.go:UpdateExtension]`, `[extension.go:GetExtensionIcon]`, `[extension.go:GetSourceIcon]`, `[extension.go:InstallMissingExtensions]`
- **Global & Admin (`global.go`)**:
  - `[global.go:GetAbout]`, `[global.go:CheckUpdate]`, `[global.go:GetLogin]`, `[global.go:PostLogin]`, `[global.go:GetLogout]`, `[global.go:GetWebview]`, `[global.go:GetWebviewProxy]`, `[global.go:WebviewWS]`, `[global.go:UpdateServer]`, `[global.go:PostUpdateServer]`
- **GraphQL Engine (`graphql.go`)**:
  - `[graphql.go:HandleGraphQL]`: Central GraphQL handler processing queries, mutations, and introspections for Tachidesk-WebUI compatibility.
- **Self-Healing (`healer.go`)**:
  - `[healer.go:SystemAuditHandler]`, `[healer.go:SystemHealHandler]`
- **Manga & Chapters (`manga.go`)**:
  - `[manga.go:GetManga]`, `[manga.go:GetChaptersList]`, `[manga.go:GetChapter]`, `[manga.go:GetPageImage]`, `[manga.go:GetMangaThumbnail]`, `[manga.go:AddMangaToLibrary]`, `[manga.go:RemoveMangaFromLibrary]`, `[manga.go:ModifyChapter]`, `[manga.go:ChapterBatchManga]`, `[manga.go:ChapterBatchGlobal]`, `[manga.go:DownloadChapter]`
- **OPDS Feeds (`opds.go`, `opds2.go`)**:
  - `[opds.go:GetOpdsRoot]`, `[opds.go:GetOpdsLibrarySeries]`, `[opds.go:GetOpdsSeriesChapters]`, `[opds2.go:GetOpds2Root]`, `[opds2.go:GetOpds2LibrarySeries]`, `[opds2.go:GetOpds2SeriesChapters]`
- **Sources (`source.go`)**:
  - `[source.go:ListSources]`, `[source.go:GetSource]`, `[source.go:GetPopularManga]`, `[source.go:GetLatestManga]`, `[source.go:SearchManga]`, `[source.go:SetSourcePreference]`
- **Trackers & Updates (`track.go`, `update.go`)**:
  - `[track.go:ListTrackers]`, `[track.go:TrackerBindManga]`, `[track.go:TrackerUpdateProgress]`
  - `[update.go:StartUpdater]`, `[update.go:ResetUpdater]`, `[update.go:UpdatesWS]`, `[update.go:GetRecentChapters]`

## Invariants
- `[handlers]`: Handlers return typed domain errors (`apperrors.AppError`) matching semantic kinds (`KindNotFound`, `KindInvalidInput`, `KindInternal`, `KindUpstreamFailed`), which are centrally translated into REST JSON, GraphQL 200 envelopes, or OPDS text responses by `CentralErrorHandler`.
- `[backup.go:ImportBackup]`: Request body must be non-empty; returns `KindInvalidInput` with code `EMPTY_BACKUP_PAYLOAD` if empty.
- `[backup.go:GetBackupFileByToken]`: Parameter `:token` must match a non-expired entry in `GlobalTempFileStorage`; returns `KindNotFound` with code `BACKUP_NOT_FOUND` if expired or missing.
- `[category.go:CreateCategory]`: Form field `name` must be non-empty; returns `KindInvalidInput` with code `MISSING_CATEGORY_NAME` if empty.
- `[category.go:ReorderCategories]`: Form fields `from` and `to` must parse as integers via `strconv.Atoi`; returns `KindInvalidInput` on error.
- `[download.go:DownloadsWS]`: WebSocket command messages must strictly equal `'STATUS'`; unknown commands receive an error message.
- `[global.go:PostLogin]`: Redirect target must begin with `'/'` and not `'//'` to prevent open redirect vulnerabilities; defaults to `'/'`.
- `[manga.go:GetManga, manga.go:GetChapter]`: URL parameters must parse to integers and corresponding entities must exist in PostgreSQL; returns `KindInvalidInput` or `KindNotFound`.
- `[manga.go:GetPageImage]`: Parameters `:mangaId`, `:chapterIndex`, and `:index` must parse as integers; returns `KindInvalidInput` on parsing failure.

## Side Effects
- `[backup.go:ImportBackup]`: Triggers `service.GlobalRestoreManager.StartAsyncRestore` mutating database tables.
- `[backup.go:ReconcileDownloadsHandler]`: Invokes `service.ReconcileDownloads` modifying download flags in PostgreSQL.
- `[category.go]`: Mutates records in `suwayomi.category` and `suwayomi.categorymeta`.
- `[download.go]`: Alters in-memory queue in `service.GlobalDownloadManager` and broadcasts WebSocket status frames.
- `[extension.go]`: Downloads extension files, writes disk manifests, and executes database updates.
- `[global.go:PostLogin]`: Sets HTTP session cookie `auth.SessionCookieName` and returns HTTP 303.
- `[global.go:GetLogout]`: Overwrites session cookie with an expired timestamp.
- `[manga.go:GetChaptersList]`: Automatically fetches and saves chapters over the network into database if local records are absent.
- `[manga.go:GetPageImage]`: Downloads and writes page image cache files to disk under `<rootDir>/cache/pages/...`.
- `[manga.go:ModifyChapter, manga.go:ChapterBatchManga]`: Updates read, bookmark, and page position in `suwayomi.chapteruser`.

## Verification
- Run: `go test ./...`
