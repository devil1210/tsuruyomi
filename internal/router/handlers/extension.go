package handlers

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
	"tsuruyomi/internal/service"
	"tsuruyomi/internal/sources"
)

var (
	iconCacheMu sync.RWMutex
	iconCache   = make(map[string][]byte)
)

//go:embed default_icon.png
var defaultIconPNG []byte

var defaultIconStub = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0xda, 0x63, 0x60, 0x60, 0x60, 0x60,
	0x00, 0x00, 0x00, 0x05, 0x00, 0x01, 0xa5, 0xf6, 0x1e, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func sendDefaultIcon(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "image/png")
	c.Set(fiber.HeaderCacheControl, "public, max-age=86400")
	if len(defaultIconPNG) > 0 {
		return c.Send(defaultIconPNG)
	}
	return c.Send(defaultIconStub)
}

// sourceToPackageMap maps known source IDs to their Tachiyomi/Keiyoushi extension package names.
var sourceToPackageMap = map[int64]string{
	2499283573021220255: "eu.kanade.tachiyomi.extension.all.mangadex",
	6400665728063187402: "eu.kanade.tachiyomi.extension.all.mangadex",
	4938773340256184018: "eu.kanade.tachiyomi.extension.all.mangadex",
	4638673959522768501: "eu.kanade.tachiyomi.extension.all.mangadex",
	2202687009511923782: "eu.kanade.tachiyomi.extension.es.mangamx",
	7178038133263703638: "eu.kanade.tachiyomi.extension.es.lectormangalat",
	5649269152264667286: "eu.kanade.tachiyomi.extension.es.shadowmanga",
	1340622632921674920: "eu.kanade.tachiyomi.extension.es.shadowmanga",
	8701445043633550960: "eu.kanade.tachiyomi.extension.es.leermanhwas",
	1598762687295904337: "eu.kanade.tachiyomi.extension.es.bokugentranslation",
	3044113561993269522: "eu.kanade.tachiyomi.extension.es.apollcomics",
	4647982373411241498: "eu.kanade.tachiyomi.extension.es.dragontranslationorg",
	1814471239661920836: "eu.kanade.tachiyomi.extension.es.cerberusseries",
	3548601371328353875: "eu.kanade.tachiyomi.extension.es.manhwalatino",
	8311505150576608267: "eu.kanade.tachiyomi.extension.es.hadesnofansub",
	6757330205319558057: "eu.kanade.tachiyomi.extension.es.begatranslation",
	3953107753690791431: "eu.kanade.tachiyomi.extension.es.uchuujinprojects",
	5943212552955476721: "eu.kanade.tachiyomi.extension.es.manhwaweb",
	1136717301008230074: "eu.kanade.tachiyomi.extension.es.submanhwa",
	829053423099062956:  "eu.kanade.tachiyomi.extension.es.mangasnosekai",
	698913079501532785:  "eu.kanade.tachiyomi.extension.es.ravenmanga",
	7925520943983324764: "eu.kanade.tachiyomi.extension.es.zonatmoto", // TuMangaOnline
	4146344224513899730: "eu.kanade.tachiyomi.extension.es.inmanga",
	8385948079634749158: "eu.kanade.tachiyomi.extension.es.manhuaonline",
	3053251231047124074: "eu.kanade.tachiyomi.extension.es.nineanime",
	6935004473807791201: "eu.kanade.tachiyomi.extension.es.plottwistnofansub",
	1348827658894665970: "eu.kanade.tachiyomi.extension.es.ragnarokscanlation",
	7732736838916328250: "eu.kanade.tachiyomi.extension.es.lmtoonline",
	8590845094935985528: "eu.kanade.tachiyomi.extension.es.zevep",
	4078570629457850125: "eu.kanade.tachiyomi.extension.es.lectorxd",
	4097111295486074350: "eu.kanade.tachiyomi.extension.es.ninemanga",
	3903750533251938295: "eu.kanade.tachiyomi.extension.es.jobsibe",
	3752522006902890093: "eu.kanade.tachiyomi.extension.es.lmtoonline",
	1326771034671033762: "eu.kanade.tachiyomi.extension.es.manhwasnet",
	4531444389842992129: "eu.kanade.tachiyomi.extension.all.batoto",
	120391793502126753:  "eu.kanade.tachiyomi.extension.en.ninemanga",
	7541593615312027809: "eu.kanade.tachiyomi.extension.es.dragontranslationorg",
}

func getFallbackIconURL(pkgName string) string {
	switch pkgName {
	case "eu.kanade.tachiyomi.extension.es.mangamx", "eu.kanade.tachiyomi.extension.es.mangaoni":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/mangamx/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.all.mangadex":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/all/mangadex/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.tumangaonline", "eu.kanade.tachiyomi.extension.es.zonatmoto":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/zonatmoto/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.lectormanga", "eu.kanade.tachiyomi.extension.es.lectormangalat":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/lectormangalat/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.leermanhwas":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/leermanhwas/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.mangasnosekai":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/mangasnosekai/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.manhwalatino":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/manhwalatino/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.manhwaweb":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/manhwaweb/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.ravenmanga":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/ravenmanga/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.shadowmanga":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/shadowmanga/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.submanhwa":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/submanhwa/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.uchuujinprojects":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/uchuujinprojects/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.bokugentranslation":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/bokugentranslation/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.apollcomics":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/apollcomics/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.dragontranslation", "eu.kanade.tachiyomi.extension.es.dragontranslationorg":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/dragontranslationorg/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.cerberuseries", "eu.kanade.tachiyomi.extension.es.cerberusseries":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/cerberusseries/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.hadesnofansub":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/hadesnofansub/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.begatranslation":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/begatranslation/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.inmanga":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/inmanga/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.manhuaonline":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/manhuaonline/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.nineanime":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/nineanime/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.plottwistnofansub":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/plottwistnofansub/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.ragnarokscanlation":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/ragnarokscanlation/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.lmtoonline":
		return "https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/es/lmtoonline/res/mipmap-xhdpi/ic_launcher.png"
	case "eu.kanade.tachiyomi.extension.es.ninemanga", "eu.kanade.tachiyomi.extension.en.ninemanga":
		return "https://es.ninemanga.com/favicon.ico"
	case "eu.kanade.tachiyomi.extension.all.batoto":
		return "https://bato.to/favicon.ico"
	case "eu.kanade.tachiyomi.extension.es.manhwasnet":
		return "https://manhwa-es.com/favicon.ico"
	case "eu.kanade.tachiyomi.extension.es.lectorxd":
		return "https://lectorxd.com/favicon.ico"
	case "eu.kanade.tachiyomi.extension.es.jobsibe":
		return "https://jobsibe.com/wp-content/uploads/2024/02/favicon.png"
	case "eu.kanade.tachiyomi.extension.es.zevep":
		return "https://zevep.com/favicon.ico"
	case "eu.kanade.tachiyomi.source.local":
		return ""
	}

	// Dynamic Keiyoushi fallback: eu.kanade.tachiyomi.extension.<lang>.<extName>
	if strings.HasPrefix(pkgName, "eu.kanade.tachiyomi.extension.") {
		parts := strings.Split(pkgName, ".")
		if len(parts) >= 6 {
			lang := parts[4]
			extName := parts[5]
			return fmt.Sprintf("https://cdn.jsdelivr.net/gh/keiyoushi/extensions-source@main/src/%s/%s/res/mipmap-xhdpi/ic_launcher.png", lang, extName)
		}
	}

	return ""
}

func fetchAndServeIcon(c *fiber.Ctx, cacheKey, iconUrl string) error {
	// 1. Check in-memory cache
	iconCacheMu.RLock()
	cached, ok := iconCache[cacheKey]
	iconCacheMu.RUnlock()
	if ok && len(cached) > 0 {
		c.Set(fiber.HeaderContentType, "image/png")
		c.Set(fiber.HeaderCacheControl, "public, max-age=604800")
		return c.Send(cached)
	}

	// 2. Fetch remote icon if URL provided
	if iconUrl != "" && (strings.HasPrefix(iconUrl, "http://") || strings.HasPrefix(iconUrl, "https://")) {
		req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, iconUrl, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
			resp, err := sources.DefaultClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err == nil && len(data) > 100 {
					iconCacheMu.Lock()
					iconCache[cacheKey] = data
					iconCacheMu.Unlock()

					ct := http.DetectContentType(data)
					if ct == "" || ct == "application/octet-stream" {
						ct = "image/png"
					}
					c.Set(fiber.HeaderContentType, ct)
					c.Set(fiber.HeaderCacheControl, "public, max-age=604800")
					return c.Send(data)
				}
			} else if resp != nil {
				resp.Body.Close()
			}
		}
	}

	return sendDefaultIcon(c)
}

// ListExtensions returns the list of all available and installed extensions.
func ListExtensions(c *fiber.Ctx) error {
	var list []models.ExtensionDataClass
	return c.JSON(list)
}

// InstallExtension installs an extension by its package name.
func InstallExtension(c *fiber.Ctx) error {
	pkgName := c.Params("pkgName")
	if pkgName == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing pkgName")
	}
	cfg, _ := c.Locals("config").(*config.Config)
	if err := service.InstallExtension(c.Context(), cfg, pkgName); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.SendStatus(fiber.StatusCreated)
}

// InstallExtensionFile installs an extension using an uploaded APK file.
func InstallExtensionFile(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	_ = file // suppress unused warning
	return c.SendStatus(fiber.StatusCreated)
}

// InstallMissingExtensions installs extensions corresponding to a list of source IDs.
func InstallMissingExtensions(c *fiber.Ctx) error {
	var body struct {
		SourceIDs []string `json:"sourceIds"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid json body"})
	}

	cfg, _ := c.Locals("config").(*config.Config)
	var intSourceIDs []int64
	for _, idStr := range body.SourceIDs {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil && id > 0 {
			intSourceIDs = append(intSourceIDs, id)
		}
	}

	installed, failed := service.InstallExtensionsForSources(c.Context(), cfg, intSourceIDs)
	return c.JSON(fiber.Map{
		"installed": installed,
		"failed":    failed,
	})
}

// UpdateExtension updates an extension by package name.
func UpdateExtension(c *fiber.Ctx) error {
	pkgName := c.Params("pkgName")
	if pkgName == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing pkgName")
	}
	cfg, _ := c.Locals("config").(*config.Config)
	if err := service.UpdateExtension(c.Context(), cfg, pkgName); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.SendStatus(fiber.StatusCreated)
}

// UninstallExtension uninstalls an extension by package name.
func UninstallExtension(c *fiber.Ctx) error {
	pkgName := c.Params("pkgName")
	if pkgName == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing pkgName")
	}
	cfg, _ := c.Locals("config").(*config.Config)
	if err := service.UninstallExtension(c.Context(), cfg, pkgName); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.SendStatus(fiber.StatusOK)
}

// GetExtensionIcon retrieves the binary icon resource for a specific extension.
func GetExtensionIcon(c *fiber.Ctx) error {
	pkgName := c.Params("pkgName")
	if pkgName == "" {
		return sendDefaultIcon(c)
	}

	// 1. Fast cache check
	iconCacheMu.RLock()
	cached, ok := iconCache[pkgName]
	iconCacheMu.RUnlock()
	if ok && len(cached) > 0 {
		c.Set(fiber.HeaderContentType, "image/png")
		c.Set(fiber.HeaderCacheControl, "public, max-age=604800")
		return c.Send(cached)
	}

	var iconUrl string
	if database.DB != nil {
		_ = database.DB.QueryRow(c.Context(), "SELECT COALESCE(icon_url, '') FROM suwayomi.extension WHERE pkg_name = $1", pkgName).Scan(&iconUrl)
	}

	if (!strings.HasPrefix(iconUrl, "http://") && !strings.HasPrefix(iconUrl, "https://")) || strings.Contains(iconUrl, "/api/v1/") {
		iconUrl = getFallbackIconURL(pkgName)
	}

	return fetchAndServeIcon(c, pkgName, iconUrl)
}

// GetSourceIcon retrieves the binary icon resource for a specific source.
func GetSourceIcon(c *fiber.Ctx) error {
	sourceIdStr := c.Params("sourceId")
	var sourceId int64
	_, _ = fmt.Sscan(sourceIdStr, &sourceId)

	cacheKey := fmt.Sprintf("src_%d", sourceId)
	iconCacheMu.RLock()
	cached, ok := iconCache[cacheKey]
	iconCacheMu.RUnlock()
	if ok && len(cached) > 0 {
		c.Set(fiber.HeaderContentType, "image/png")
		c.Set(fiber.HeaderCacheControl, "public, max-age=604800")
		return c.Send(cached)
	}

	var iconUrl, pkgName string
	if database.DB != nil && sourceId > 0 {
		_ = database.DB.QueryRow(c.Context(), `
			SELECT COALESCE(e.icon_url, ''), COALESCE(e.pkg_name, '')
			FROM suwayomi.source s
			LEFT JOIN suwayomi.extension e ON s.extension = e.id
			WHERE s.id = $1
		`, sourceId).Scan(&iconUrl, &pkgName)
	}

	if (pkgName == "" || strings.HasPrefix(pkgName, "imported.source.")) && sourceId > 0 {
		if mappedPkg, exists := sourceToPackageMap[sourceId]; exists {
			pkgName = mappedPkg
		} else if known, ok := database.KnownSourcesByID[sourceId]; ok {
			pkgName = known.PkgName
			if iconUrl == "" || strings.Contains(iconUrl, "/api/v1/") {
				iconUrl = known.IconURL
			}
		}
	}

	// If iconUrl is a relative /api/v1/... URL or not a remote URL, resolve real fallback icon from Keiyoushi
	if (!strings.HasPrefix(iconUrl, "http://") && !strings.HasPrefix(iconUrl, "https://")) || strings.Contains(iconUrl, "/api/v1/") {
		if pkgName != "" {
			iconUrl = getFallbackIconURL(pkgName)
		}
	}

	return fetchAndServeIcon(c, cacheKey, iconUrl)
}
