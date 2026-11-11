# Domain Models & Data Transfer Objects (`internal/models`)

## Overview
The `internal/models` package defines pure domain models, data transfer objects (DTOs), enumerations, and JSON/GraphQL conversion helpers for Suwayomi and Tachidesk entities across categories, chapters, downloads, extensions, manga, sources, and trackers.

## Interface
- `[category.go:CategoryDataClass]`: Category entity model with JSON tags and update indicators.
- `[category.go:IncludeOrExclude]`: Tri-state filtering enum (`Unset`, `Include`, `Exclude`) with GraphQL enum parsing (`ParseIncludeOrExclude`).
- `[chapter.go:ChapterDataClass]`: Chapter metadata model (ID, URL, name, upload date, read state, page count, scanlator).
- `[chapter.go:ChapterBatchEditInput]`: Input DTO for batch mutating chapter read and bookmark states.
- `[chapter.go:PaginatedList]`: Generic pagination wrapper (`[T any]`) for lists with total count and page indicators.
- `[download.go:DownloadQueueItem]`: Queue item model tracking chapter download progress, retry count, and state.
- `[download.go:DownloadState]`: Download state enumeration (`DownloadStateQueued`, `DownloadStateDownloading`, `DownloadStateFinished`, `DownloadStateError`).
- `[download.go:DownloadStatus]`: Comprehensive status DTO containing queue items and downloader running state.
- `[download.go:EnqueueInput]`: Batch enqueue request payload.
- `[extension.go:ExtensionDataClass]`: Extension manifest representation (package, name, version, libVersion, icon, installed status).
- `[extension.go:ExtensionStore]`: Repository store entity model.
- `[manga.go:MangaDataClass]`: Core manga series model (title, author, artist, description, genre, status, cover thumbnail, source ID).
- `[manga.go:MangaStatus]`: Manga publication status enum (`Unknown`, `Ongoing`, `Completed`, `Licensed`, `PublishingFinished`, `Cancelled`, `OnHiatus`) with integer conversion helpers.
- `[manga.go:UpdateStrategy]`: Manga update evaluation enum (`AlwaysUpdate`, `OnlyFetchOnce`).
- `[source.go:SourceDataClass]`: Manga source representation (ID, name, lang, icon, NSFW flag).
- `[source.go:FilterData, source.go:PreferenceObject]`: Dynamic source filter and configuration preference DTOs.
- `[tracker.go:TrackerDataClass, tracker.go:TrackRecordDataClass]`: Tracking provider integration models (AniList, MyAnimeList, Kitsu, Shikimori, Bangumi).

## Invariants
- `[models]`: None (pure data representations with JSON serialization mappings and zero runtime assertion constraints).

## Side Effects
- `[models]`: None (contains no I/O, database access, network calls, or memory state mutations).

## Verification
- Run: `go test ./...`
