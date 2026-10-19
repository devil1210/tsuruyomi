package database

import (
	"context"
	"fmt"
	"log"
	"strings"

	"tsuruyomi/internal/models"
)

// NativeExtensionSeed defines metadata to seed native and popular extensions into suwayomi.extension.
type NativeExtensionSeed struct {
	ID             int
	PkgName        string
	Name           string
	Lang           string
	IconURL        string
	ContentWarning int
	Sources        []NativeSourceSeed
}

// NativeSourceSeed defines metadata to seed sources into suwayomi.source.
type NativeSourceSeed struct {
	ID             int64
	Name           string
	DisplayName    string
	Lang           string
	ContentWarning int
}

// NativeExtensionsSeed lists all out-of-the-box native Go extensions and primary sources.
var NativeExtensionsSeed = []NativeExtensionSeed{
	{
		ID:             1,
		PkgName:        "eu.kanade.tachiyomi.extension.all.mangadex",
		Name:           "MangaDex",
		Lang:           "all",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.all.mangadex",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 6400665728063187402, Name: "MangaDex (ES)", DisplayName: "MangaDex (ES)", Lang: "es"},
			{ID: 2499283573021220255, Name: "MangaDex (EN)", DisplayName: "MangaDex (EN)", Lang: "en"},
			{ID: 4938773340256184018, Name: "MangaDex (ES-419)", DisplayName: "MangaDex (ES-419)", Lang: "es-419"},
			{ID: 4638673959522768501, Name: "MangaDex (ALL)", DisplayName: "MangaDex (ALL)", Lang: "all"},
		},
	},
	{
		ID:             2,
		PkgName:        "eu.kanade.tachiyomi.extension.es.mangamx",
		Name:           "MangaOni",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.mangamx",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 2202687009511923782, Name: "MangaOni", DisplayName: "MangaOni (ES)", Lang: "es"},
		},
	},
	{
		ID:             3,
		PkgName:        "eu.kanade.tachiyomi.extension.es.lectormangalat",
		Name:           "LectorManga.lat",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.lectormangalat",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 7178038133263703638, Name: "LectorManga", DisplayName: "LectorManga (ES)", Lang: "es"},
		},
	},
	{
		ID:             4,
		PkgName:        "eu.kanade.tachiyomi.extension.es.shadowmanga",
		Name:           "Shadow Manga",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.shadowmanga",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 5649269152264667286, Name: "Shadow Manga", DisplayName: "Shadow Manga (ES)", Lang: "es", ContentWarning: 0},
			{ID: 1340622632921674920, Name: "Shadow Manga (+18)", DisplayName: "Shadow Manga (+18) (ES)", Lang: "es", ContentWarning: 2},
		},
	},
	{
		ID:             5,
		PkgName:        "eu.kanade.tachiyomi.extension.es.leermanhwas",
		Name:           "LeerManhwas",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.leermanhwas",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 8701445043633550960, Name: "LeerManhwas", DisplayName: "LeerManhwas (ES)", Lang: "es"},
		},
	},
	{
		ID:             6,
		PkgName:        "eu.kanade.tachiyomi.extension.es.bokugentranslation",
		Name:           "BokugenTranslation",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.bokugentranslation",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 1598762687295904337, Name: "BokugenTranslation", DisplayName: "BokugenTranslation (ES)", Lang: "es"},
		},
	},
	{
		ID:             7,
		PkgName:        "eu.kanade.tachiyomi.extension.es.apollcomics",
		Name:           "ApollComics",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.apollcomics",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 3044113561993269522, Name: "ApollComics", DisplayName: "ApollComics (ES)", Lang: "es"},
		},
	},
	{
		ID:             8,
		PkgName:        "eu.kanade.tachiyomi.extension.es.dragontranslationorg",
		Name:           "DragonTranslation",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.dragontranslationorg",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 4647982373411241498, Name: "DragonTranslation", DisplayName: "DragonTranslation (ES)", Lang: "es"},
			{ID: 7541593615312027809, Name: "DragonTranslation.net", DisplayName: "DragonTranslation.net (ES)", Lang: "es"},
		},
	},
	{
		ID:             9,
		PkgName:        "eu.kanade.tachiyomi.extension.es.cerberusseries",
		Name:           "Cerberus Series",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.cerberusseries",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 1814471239661920836, Name: "Cerberus Series", DisplayName: "Cerberus Series (ES)", Lang: "es"},
		},
	},
	{
		ID:             10,
		PkgName:        "eu.kanade.tachiyomi.extension.es.manhwalatino",
		Name:           "Manhwa-Latino",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.manhwalatino",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 3548601371328353875, Name: "Manhwa-Latino", DisplayName: "Manhwa-Latino (ES)", Lang: "es"},
		},
	},
	{
		ID:             11,
		PkgName:        "eu.kanade.tachiyomi.extension.es.hadesnofansub",
		Name:           "Hades no Fansub",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.hadesnofansub",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 8311505150576608267, Name: "Hades no Fansub", DisplayName: "Hades no Fansub (ES)", Lang: "es"},
		},
	},
	{
		ID:             12,
		PkgName:        "eu.kanade.tachiyomi.extension.es.begatranslation",
		Name:           "Bega Translation",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.begatranslation",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 6757330205319558057, Name: "Bega Translation", DisplayName: "Bega Translation (ES)", Lang: "es"},
		},
	},
	{
		ID:             13,
		PkgName:        "eu.kanade.tachiyomi.extension.es.uchuujinprojects",
		Name:           "Uchuujin Projects",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.uchuujinprojects",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 3953107753690791431, Name: "Uchuujin Projects", DisplayName: "Uchuujin Projects (ES)", Lang: "es"},
		},
	},
	{
		ID:             14,
		PkgName:        "eu.kanade.tachiyomi.extension.es.manhwaweb",
		Name:           "ManhwaWeb",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.manhwaweb",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 5943212552955476721, Name: "ManhwaWeb", DisplayName: "ManhwaWeb (ES)", Lang: "es"},
		},
	},
	{
		ID:             15,
		PkgName:        "eu.kanade.tachiyomi.extension.es.submanhwa",
		Name:           "Submanhwa",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.submanhwa",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 1136717301008230074, Name: "Submanhwa", DisplayName: "Submanhwa (ES)", Lang: "es"},
		},
	},
	{
		ID:             16,
		PkgName:        "eu.kanade.tachiyomi.extension.es.mangasnosekai",
		Name:           "Mangas No Sekai",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.mangasnosekai",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 829053423099062956, Name: "Mangas No Sekai", DisplayName: "Mangas No Sekai (ES)", Lang: "es"},
		},
	},
	{
		ID:             17,
		PkgName:        "eu.kanade.tachiyomi.extension.es.ravenmanga",
		Name:           "RavenManga",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.ravenmanga",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 698913079501532785, Name: "RavenManga", DisplayName: "RavenManga (ES)", Lang: "es"},
		},
	},
	{
		ID:             18,
		PkgName:        "eu.kanade.tachiyomi.source.local",
		Name:           "Local Source",
		Lang:           "all",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.source.local",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 0, Name: "Local Source", DisplayName: "Local Source", Lang: "all"},
		},
	},
	{
		ID:             19,
		PkgName:        "eu.kanade.tachiyomi.extension.es.tumangaonline",
		Name:           "TuMangaOnline",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.tumangaonline",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 7925520943983324764, Name: "TuMangaOnline", DisplayName: "TuMangaOnline (ES)", Lang: "es"},
		},
	},
	{
		ID:             20,
		PkgName:        "eu.kanade.tachiyomi.extension.es.inmanga",
		Name:           "InManga",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.inmanga",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 4146344224513899730, Name: "InManga", DisplayName: "InManga (ES)", Lang: "es"},
		},
	},
	{
		ID:             21,
		PkgName:        "eu.kanade.tachiyomi.extension.es.manhuaonline",
		Name:           "Manhua Online",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.manhuaonline",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 8385948079634749158, Name: "Manhua Online", DisplayName: "Manhua Online (ES)", Lang: "es"},
		},
	},
	{
		ID:             22,
		PkgName:        "eu.kanade.tachiyomi.extension.es.nineanime",
		Name:           "NineAnime",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.nineanime",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 3053251231047124074, Name: "NineAnime", DisplayName: "NineAnime (ES)", Lang: "es"},
		},
	},
	{
		ID:             23,
		PkgName:        "eu.kanade.tachiyomi.extension.es.plottwistnofansub",
		Name:           "Plot Twist No Fansub",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.plottwistnofansub",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 6935004473807791201, Name: "Plot Twist No Fansub", DisplayName: "Plot Twist No Fansub (ES)", Lang: "es"},
		},
	},
	{
		ID:             24,
		PkgName:        "eu.kanade.tachiyomi.extension.es.ragnarokscanlation",
		Name:           "Ragnarok Scanlation",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.ragnarokscanlation",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 1348827658894665970, Name: "Ragnarok Scanlation", DisplayName: "Ragnarok Scanlation (ES)", Lang: "es"},
		},
	},
	{
		ID:             25,
		PkgName:        "eu.kanade.tachiyomi.extension.es.lmtoonline",
		Name:           "LMTO Online",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.lmtoonline",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 3752522006902890093, Name: "LMTO Online", DisplayName: "LMTO Online (ES)", Lang: "es"},
			{ID: 7732736838916328250, Name: "Lmtos", DisplayName: "Lmtos (ES)", Lang: "es"},
		},
	},
	{
		ID:             26,
		PkgName:        "eu.kanade.tachiyomi.extension.es.ninemanga",
		Name:           "NineManga",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.ninemanga",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 4097111295486074350, Name: "NineMangaEs", DisplayName: "NineManga (ES)", Lang: "es"},
		},
	},
	{
		ID:             27,
		PkgName:        "eu.kanade.tachiyomi.extension.en.ninemanga",
		Name:           "NineManga (EN)",
		Lang:           "en",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.en.ninemanga",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 120391793502126753, Name: "NineMangaEn", DisplayName: "NineManga (EN)", Lang: "en"},
		},
	},
	{
		ID:             28,
		PkgName:        "eu.kanade.tachiyomi.extension.es.manhwasnet",
		Name:           "Manhwas.net",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.manhwasnet",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 1326771034671033762, Name: "Manhwas.net", DisplayName: "Manhwas.net (ES)", Lang: "es"},
		},
	},
	{
		ID:             29,
		PkgName:        "eu.kanade.tachiyomi.extension.all.batoto",
		Name:           "Bato.to",
		Lang:           "all",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.all.batoto",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 4531444389842992129, Name: "Bato.to", DisplayName: "Bato.to", Lang: "all"},
		},
	},
	{
		ID:             30,
		PkgName:        "eu.kanade.tachiyomi.extension.es.jobsibe",
		Name:           "Jobsibe",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.jobsibe",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 3903750533251938295, Name: "Jobsibe", DisplayName: "Jobsibe (ES)", Lang: "es"},
		},
	},
	{
		ID:             31,
		PkgName:        "eu.kanade.tachiyomi.extension.es.zevep",
		Name:           "Zevep",
		Lang:           "es",
		IconURL:        "/api/v1/extension/icon/eu.kanade.tachiyomi.extension.es.zevep",
		ContentWarning: 0,
		Sources: []NativeSourceSeed{
			{ID: 8590845094935985528, Name: "Zevep", DisplayName: "Zevep (ES)", Lang: "es"},
		},
	},
}

// KnownSourceInfo contains metadata for a known source ID.
type KnownSourceInfo struct {
	Name        string
	DisplayName string
	Lang        string
	PkgName     string
	ExtName     string
	IconURL     string
}

// KnownSourcesByID provides an in-memory lookup of all known native and popular source IDs.
var KnownSourcesByID = make(map[int64]KnownSourceInfo)

func init() {
	for _, ext := range NativeExtensionsSeed {
		for _, src := range ext.Sources {
			dispName := src.DisplayName
			if dispName == "" {
				dispName = src.Name
			}
			KnownSourcesByID[src.ID] = KnownSourceInfo{
				Name:        src.Name,
				DisplayName: dispName,
				Lang:        src.Lang,
				PkgName:     ext.PkgName,
				ExtName:     ext.Name,
				IconURL:     ext.IconURL,
			}
		}
	}
	// Well-known third-party / legacy Tachiyomi source mappings from backups
	KnownSourcesByID[1424373335432920755] = KnownSourceInfo{
		Name:        "YugenMangas",
		DisplayName: "YugenMangas (ES)",
		Lang:        "es",
		PkgName:     "eu.kanade.tachiyomi.extension.es.yugenmangas",
		ExtName:     "YugenMangas",
		IconURL:     "",
	}
}

// EnsureNativeSources populates suwayomi.extension and suwayomi.source tables with native Go sources and resolves unmapped mangas.
func EnsureNativeSources(ctx context.Context) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	for _, ext := range NativeExtensionsSeed {
		var extID int
		err := DB.QueryRow(ctx, `
			INSERT INTO suwayomi.extension (id, apk_name, icon_url, name, pkg_name, version_name, version_code, lang, is_installed, has_update, is_obsolete, class_name, store_index_url, extension_lib, apk_url, content_warning)
			VALUES ($1, $2, $3, $4, $5, '1.0-go', 100, $6, true, false, false, '', 'https://raw.githubusercontent.com/devil1210/tsuruyomi-extensions/main/index.json', '1.5', '', $7)
			ON CONFLICT (pkg_name) DO UPDATE SET
				name = EXCLUDED.name,
				icon_url = EXCLUDED.icon_url,
				lang = EXCLUDED.lang,
				is_installed = true
			RETURNING id
		`, ext.ID, ext.PkgName+".apk", ext.IconURL, ext.Name, ext.PkgName, ext.Lang, ext.ContentWarning).Scan(&extID)
		if err != nil {
			log.Printf("[DATABASE] Warning inserting native extension %s: %v", ext.Name, err)
			continue
		}

		for _, s := range ext.Sources {
			_, err := DB.Exec(ctx, `
				INSERT INTO suwayomi.source (id, name, lang, extension, content_warning)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					lang = EXCLUDED.lang,
					extension = EXCLUDED.extension,
					content_warning = EXCLUDED.content_warning
			`, s.ID, s.Name, s.Lang, extID, s.ContentWarning)
			if err != nil {
				log.Printf("[DATABASE] Warning inserting native source %s (%d): %v", s.Name, s.ID, err)
			}
		}
	}

	// Also ensure any mangas with sources not yet in suwayomi.source have placeholder rows so foreign keys and migrations succeed
	_, _ = DB.Exec(ctx, `
		INSERT INTO suwayomi.extension (apk_name, icon_url, name, pkg_name, version_name, version_code, lang, is_installed, has_update, is_obsolete, class_name, store_index_url, extension_lib, apk_url, content_warning)
		SELECT 'imported.apk', '', 'Imported Source', 'imported.source.' || m.source, '1.0', 1, 'all', false, false, false, '', '', '1.0', '', 0
		FROM suwayomi.manga m
		LEFT JOIN suwayomi.source s ON m.source = s.id
		WHERE s.id IS NULL
		GROUP BY m.source
		ON CONFLICT (pkg_name) DO UPDATE SET is_installed = false;

		INSERT INTO suwayomi.source (id, name, lang, extension, content_warning)
		SELECT m.source, 'Source ' || m.source, 'all', e.id, 0
		FROM suwayomi.manga m
		LEFT JOIN suwayomi.source s ON m.source = s.id
		LEFT JOIN suwayomi.extension e ON e.pkg_name = 'imported.source.' || m.source
		WHERE s.id IS NULL
		GROUP BY m.source, e.id
		ON CONFLICT (id) DO NOTHING;

		UPDATE suwayomi.extension SET is_installed = false WHERE pkg_name LIKE 'imported.source.%' OR name = 'Imported Source';
		UPDATE suwayomi.source SET name = 'YugenMangas', lang = 'es' WHERE id = 1424373335432920755;
	`)

	// Re-link sources that point to alias extension packages to canonical extension packages
	_, _ = DB.Exec(ctx, `
		UPDATE suwayomi.source s
		SET extension = c.id
		FROM suwayomi.extension a
		JOIN suwayomi.extension c ON (
			(a.pkg_name = 'eu.kanade.tachiyomi.extension.es.cerberuseries' AND c.pkg_name = 'eu.kanade.tachiyomi.extension.es.cerberusseries') OR
			(a.pkg_name = 'eu.kanade.tachiyomi.extension.es.dragontranslation' AND c.pkg_name = 'eu.kanade.tachiyomi.extension.es.dragontranslationorg') OR
			(a.pkg_name = 'eu.kanade.tachiyomi.extension.es.mangaoni' AND c.pkg_name = 'eu.kanade.tachiyomi.extension.es.mangamx') OR
			(a.pkg_name = 'eu.kanade.tachiyomi.extension.es.lectormanga' AND c.pkg_name = 'eu.kanade.tachiyomi.extension.es.lectormangalat')
		)
		WHERE s.extension = a.id;

		DELETE FROM suwayomi.extension
		WHERE pkg_name IN (
			'eu.kanade.tachiyomi.extension.es.cerberuseries',
			'eu.kanade.tachiyomi.extension.es.dragontranslation',
			'eu.kanade.tachiyomi.extension.es.mangaoni',
			'eu.kanade.tachiyomi.extension.es.lectormanga'
		);
	`)

	// Clean up and heal any leftover imported placeholder sources and extensions
	_, _ = DB.Exec(ctx, `
		DELETE FROM suwayomi.extension e
		WHERE (e.pkg_name LIKE 'imported.source.%' OR e.name = 'Imported Source')
		  AND NOT EXISTS (SELECT 1 FROM suwayomi.source s WHERE s.extension = e.id);
	`)

	// Align sequence with highest ID
	_, _ = DB.Exec(ctx, `
		SELECT setval('suwayomi.extension_id_seq', GREATEST(COALESCE((SELECT MAX(id) FROM suwayomi.extension), 1), 50))
	`)

	return nil
}

// GetSourceList returns all configured sources from the database.
func GetSourceList(ctx context.Context) ([]models.SourceDataClass, error) {
	if DB == nil {
		return []models.SourceDataClass{}, nil
	}

	// If source table is empty, auto-seed native sources first
	var count int
	_ = DB.QueryRow(ctx, "SELECT COUNT(*) FROM suwayomi.source").Scan(&count)
	if count == 0 {
		_ = EnsureNativeSources(ctx)
	}

	query := `
		SELECT s.id, s.name, s.lang, COALESCE(e.icon_url, ''), COALESCE(e.pkg_name, ''), COALESCE(s.content_warning, e.content_warning, 0)
		FROM suwayomi.source s
		LEFT JOIN suwayomi.extension e ON s.extension = e.id
		WHERE s.id = 0 OR (COALESCE(e.is_installed, false) = true AND COALESCE(e.pkg_name, '') NOT LIKE 'imported.source.%')
		ORDER BY s.name ASC
	`

	rows, err := DB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying sources: %w", err)
	}
	defer rows.Close()

	list := []models.SourceDataClass{}
	for rows.Next() {
		var src models.SourceDataClass
		var idVal int64
		var iconURL, pkgName string
		var cwInt int
		err := rows.Scan(&idVal, &src.Name, &src.Lang, &iconURL, &pkgName, &cwInt)
		if err != nil {
			return nil, fmt.Errorf("scanning source row: %w", err)
		}
		src.ID = fmt.Sprintf("%d", idVal)
		if known, ok := KnownSourcesByID[idVal]; ok {
			if src.Name == "" || strings.HasPrefix(src.Name, "Source ") || src.Name == "Source" {
				src.Name = known.Name
			}
			if src.Lang == "" || (src.Lang == "all" && known.Lang != "all") {
				src.Lang = known.Lang
			}
			if pkgName == "" || strings.HasPrefix(pkgName, "imported.source.") {
				pkgName = known.PkgName
			}
			if iconURL == "" || strings.Contains(iconURL, "/api/v1/") {
				iconURL = known.IconURL
			}
			if known.DisplayName != "" {
				src.DisplayName = known.DisplayName
			}
		}
		if src.DisplayName == "" {
			if src.Lang != "" && src.Lang != "all" {
				src.DisplayName = fmt.Sprintf("%s (%s)", src.Name, strings.ToUpper(src.Lang))
			} else {
				src.DisplayName = src.Name
			}
		}
		src.SupportsLatest = true
		src.IconURL = iconURL
		src.PkgName = pkgName
		switch cwInt {
		case 1:
			src.ContentWarning = "SUGGESTIVE"
			src.IsNsfw = false
		case 2:
			src.ContentWarning = "NSFW"
			src.IsNsfw = true
		default:
			src.ContentWarning = "SAFE"
			src.IsNsfw = false
		}
		list = append(list, src)
	}

	return list, nil
}

// GetSourceByID retrieves a single source details by its ID.
func GetSourceByID(ctx context.Context, id int64) (*models.SourceDataClass, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT s.id, s.name, s.lang, COALESCE(e.icon_url, ''), COALESCE(e.pkg_name, ''), COALESCE(s.content_warning, e.content_warning, 0)
		FROM suwayomi.source s
		LEFT JOIN suwayomi.extension e ON s.extension = e.id
		WHERE s.id = $1
		LIMIT 1
	`

	var src models.SourceDataClass
	var idVal int64
	var iconURL, pkgName string
	var cwInt int
	err := DB.QueryRow(ctx, query, id).Scan(&idVal, &src.Name, &src.Lang, &iconURL, &pkgName, &cwInt)
	if err != nil {
		// Fallback to NativeExtensionsSeed
		for _, ext := range NativeExtensionsSeed {
			for _, s := range ext.Sources {
				if s.ID == id {
					cw := "SAFE"
					if s.ContentWarning == 2 {
						cw = "NSFW"
					}
					return &models.SourceDataClass{
						ID:             fmt.Sprintf("%d", id),
						Name:           s.Name,
						DisplayName:    s.DisplayName,
						Lang:           s.Lang,
						IconURL:        "/api/v1/source/" + fmt.Sprintf("%d", id) + "/icon",
						PkgName:        ext.PkgName,
						ContentWarning: cw,
						SupportsLatest: true,
						IsNsfw:         s.ContentWarning == 2,
					}, nil
				}
			}
		}
		return nil, err
	}

	src.ID = fmt.Sprintf("%d", idVal)
	if src.Lang != "" && src.Lang != "all" {
		src.DisplayName = fmt.Sprintf("%s (%s)", src.Name, strings.ToUpper(src.Lang))
	} else {
		src.DisplayName = src.Name
	}
	src.SupportsLatest = true
	src.IconURL = iconURL
	src.PkgName = pkgName
	switch cwInt {
	case 1:
		src.ContentWarning = "SUGGESTIVE"
		src.IsNsfw = false
	case 2:
		src.ContentWarning = "NSFW"
		src.IsNsfw = true
	default:
		src.ContentWarning = "SAFE"
		src.IsNsfw = false
	}

	return &src, nil
}
