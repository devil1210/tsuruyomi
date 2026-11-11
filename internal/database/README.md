# Database Persistence & Schema (`internal/database`)

## Overview
The `internal/database` package provides the PostgreSQL persistence layer for Tsuruyomi using `github.com/jackc/pgx/v5`. It encapsulates connection pooling, embedded schema bootstrapping (`schema.sql`), dynamic server configuration storage, category management, chapter ordering normalization, extension store synchronizations, and native source catalog seeding.

## Interface
- `[db.go:DB]`: `var DB *pgxpool.Pool` global PostgreSQL connection pool.
- `[db.go:Connect]`: `func Connect(ctx context.Context, cfg config.DatabaseConfig) error` initializing connection pool and repairing chapter orders.
- `[db.go:Close]`: `func Close()` terminating connection pool cleanly.
- `[db.go:EnsureSchema]`: `func EnsureSchema(ctx context.Context) error` executing schema DDL and seeding default records.
- `[db.go:GetServerSetting]`: `func GetServerSetting(ctx context.Context, key string, fallback string) string`
- `[db.go:SetServerSetting]`: `func SetServerSetting(ctx context.Context, key string, value string) error`
- `[db.go:SetServerSettings]`: `func SetServerSettings(ctx context.Context, settings map[string]string) error`
- `[db.go:GetAllServerSettings]`: `func GetAllServerSettings(ctx context.Context) (map[string]string, error)`
- `[category.go:GetCategoryList]`: `func GetCategoryList(ctx context.Context) ([]models.CategoryDataClass, error)`
- `[category.go:GetCategoryMangas]`: `func GetCategoryMangas(ctx context.Context, categoryId int) ([]models.MangaDataClass, error)`
- `[category.go:GetMangaCategories]`: `func GetMangaCategories(ctx context.Context, mangaId int) ([]models.CategoryDataClass, error)`
- `[category.go:CreateCategory]`: `func CreateCategory(ctx context.Context, name string) (int, error)`
- `[category.go:UpdateCategory]`: `func UpdateCategory(ctx context.Context, categoryId int, name *string, isDefault *bool, includeInUpdate *int, includeInDownload *int) error`
- `[category.go:DeleteCategory]`: `func DeleteCategory(ctx context.Context, categoryId int) error`
- `[category.go:ReorderCategory]`: `func ReorderCategory(ctx context.Context, from, to int) error`
- `[category.go:GetCategoryByID]`: `func GetCategoryByID(ctx context.Context, categoryId int) (*models.CategoryDataClass, error)`
- `[chapter.go:GetChaptersByMangaID]`: `func GetChaptersByMangaID(ctx context.Context, mangaId int) ([]models.ChapterDataClass, error)`
- `[chapter.go:GetChapterByMangaAndIndex]`: `func GetChapterByMangaAndIndex(ctx context.Context, mangaId int, chapterIndex int) (*models.ChapterDataClass, error)`
- `[chapter.go:GetRecentChapters]`: `func GetRecentChapters(ctx context.Context, pageNum int) (*models.PaginatedList[models.MangaChapterDataClass], error)`
- `[chapter.go:EnsureCleanChapterSourceOrders]`: `func EnsureCleanChapterSourceOrders(ctx context.Context) error`
- `[extension_store.go:GetExtensionStores]`: `func GetExtensionStores(ctx context.Context) ([]models.ExtensionStore, error)`
- `[extension_store.go:AddExtensionStore]`: `func AddExtensionStore(ctx context.Context, store models.ExtensionStore) error`
- `[extension_store.go:RemoveExtensionStore]`: `func RemoveExtensionStore(ctx context.Context, indexURL string) error`
- `[extension_store.go:SetExtensionStores]`: `func SetExtensionStores(ctx context.Context, urls []string) error`
- `[manga.go:GetMangaByID]`: `func GetMangaByID(ctx context.Context, id int) (*models.MangaDataClass, error)`
- `[source.go:KnownSourcesByID]`: `var KnownSourcesByID map[int64]KnownSourceInfo`
- `[source.go:EnsureNativeSources]`: `func EnsureNativeSources(ctx context.Context) error`
- `[source.go:GetSourceList]`: `func GetSourceList(ctx context.Context) ([]models.SourceDataClass, error)`
- `[source.go:GetSourceByID]`: `func GetSourceByID(ctx context.Context, id int64) (*models.SourceDataClass, error)`

## Invariants
- `[category.go:CreateCategory]`: If category name is lowercase `'default'`, `CreateCategory` returns ID 0 without insertion; if name already exists (case-insensitive match), it returns the existing category ID.
- `[category.go:UpdateCategory, category.go:DeleteCategory]`: Category ID 0 is immutable (cannot have its name or default status altered, and cannot be deleted).
- `[category.go:ReorderCategory]`: Rejects reordering default category ID 0 and rejects out-of-bounds index requests.
- `[chapter.go:GetChaptersByMangaID]`: Missing `ChapterNumber` (`<= 0`) falls back to chapter `Index`; missing `UploadDate` falls back to `FetchedAt * 1000` or current timestamp; empty `Name` falls back to `'Capítulo <number/index>'`.
- `[chapter.go:EnsureCleanChapterSourceOrders]`: Automatically detects broken mangas where `min(source_order) != 1` or `max(source_order) != count(*)` or `count(distinct source_order) != count(*)`, resetting `source_order` to contiguous `1..N` order.
- `[chapter.go:EnsureCleanChapterSourceOrders]`: Backfills `chapter_number` using regex `'([0-9]+(\\.[0-9]+)?)'` when chapter number is `0` or `-1`.
- `[db.go:EnsureSchema]`: Triggers `schema.sql` execution only when `suwayomi.category` table is absent from `information_schema.tables`, guaranteeing default useraccount (`id 1`, `'admin'`) and default category (`id 0`, `'Default'`).
- `[db.go:GetServerSetting]`: Returns fallback value if database connection pool is uninitialized, key is not found, or retrieved value is empty.
- `[manga.go:GetMangaByID]`: Returns `(nil, nil)` when `pgx.ErrNoRows` is encountered.
- `[source.go:GetSourceList]`: Restricts returned sources to ID 0 (local source) or installed non-imported extensions.
- `[source.go:GetSourceList, source.go:GetSourceByID]`: Content warning integer mapping: `1` -> `'SUGGESTIVE'`, `2` -> `'NSFW'`, default -> `'SAFE'`.

## Side Effects
- `[db.go:Connect]`: Initializes connection pool `DB` and triggers `EnsureCleanChapterSourceOrders`.
- `[db.go:Close]`: Terminates the global connection pool.
- `[db.go:EnsureSchema]`: Executes `schema.sql`, populates default user and category rows, and invokes `EnsureNativeSources`.
- `[db.go:SetServerSetting, db.go:SetServerSettings]`: Upserts records in `suwayomi.serversetting`.
- `[category.go:CreateCategory, category.go:UpdateCategory, category.go:DeleteCategory, category.go:ReorderCategory]`: Mutates records and sort orders in `suwayomi.category`.
- `[chapter.go:EnsureCleanChapterSourceOrders]`: Updates `source_order` and `chapter_number` in `suwayomi.chapter`.
- `[extension_store.go:AddExtensionStore, extension_store.go:RemoveExtensionStore, extension_store.go:SetExtensionStores]`: Inserts, updates, or deletes records in `suwayomi.extensionstore`.
- `[source.go:EnsureNativeSources]`: Inserts native sources/extensions into `suwayomi.source` and `suwayomi.extension`, removes deprecated aliases, and advances `suwayomi.extension_id_seq`.

## Verification
- Run: `go test ./...`
