package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/extensions"
	"tsuruyomi/internal/models"
	"tsuruyomi/internal/sources"
)

// ListSources lists all configured sources.
func ListSources(c *fiber.Ctx) error {
	list, err := database.GetSourceList(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.JSON(list)
}

// GetSource retrieves a single source info by its ID.
func GetSource(c *fiber.Ctx) error {
	sourceIdVal, err := strconv.ParseInt(c.Params("sourceId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid source ID")
	}

	src, err := database.GetSourceByID(c.Context(), sourceIdVal)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	if src == nil {
		return c.Status(fiber.StatusNotFound).SendString("Source not found")
	}

	return c.JSON(src)
}

func saveAndFormatMangaList(ctx context.Context, sourceID int64, items []sources.MangaItem, hasNext bool) models.PagedMangaListDataClass {
	var resultList []models.MangaDataClass
	if database.DB == nil {
		for _, it := range items {
			thumb := it.ThumbnailURL
			resultList = append(resultList, models.MangaDataClass{
				SourceID:     fmt.Sprintf("%d", sourceID),
				URL:          it.URL,
				Title:        it.Title,
				ThumbnailURL: &thumb,
				Status:       models.MangaStatusFromInt(it.Status),
			})
		}
		return models.PagedMangaListDataClass{
			MangaList:   resultList,
			HasNextPage: hasNext,
		}
	}

	for _, it := range items {
		if it.SourceID == 0 {
			it.SourceID = sourceID
		}
		genreStr := strings.Join(it.Genres, ", ")
		var mID int64
		errFind := database.DB.QueryRow(ctx, "SELECT id FROM suwayomi.manga WHERE source = $1 AND url = $2", it.SourceID, it.URL).Scan(&mID)
		if errFind == nil && mID > 0 {
			_, _ = database.DB.Exec(ctx, `
				UPDATE suwayomi.manga SET
					title = $1,
					thumbnail_url = COALESCE(NULLIF($2, ''), thumbnail_url),
					description = $3,
					genre = $4,
					status = $5,
					artist = $6,
					author = $7
				WHERE id = $8
			`, it.Title, it.ThumbnailURL, it.Description, genreStr, it.Status, it.Artist, it.Author, mID)
		} else {
			errInsert := database.DB.QueryRow(ctx, `
				INSERT INTO suwayomi.manga (source, url, title, artist, author, description, genre, status, thumbnail_url, initialized, memo)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, '{}')
				RETURNING id
			`, it.SourceID, it.URL, it.Title, it.Artist, it.Author, it.Description, genreStr, it.Status, it.ThumbnailURL).Scan(&mID)
			if errInsert == nil && mID > 0 {
				_, _ = database.DB.Exec(ctx, `
					INSERT INTO suwayomi.mangauser (manga, user_id, in_library, in_library_at)
					VALUES ($1, 1, false, 0)
					ON CONFLICT (manga, user_id) DO NOTHING
				`, mID)
			}
		}

		if mID > 0 {
			if m, err := database.GetMangaByID(ctx, int(mID)); err == nil && m != nil {
				resultList = append(resultList, *m)
				continue
			}
		}

		thumb := it.ThumbnailURL
		resultList = append(resultList, models.MangaDataClass{
			ID:           int(mID),
			SourceID:     fmt.Sprintf("%d", sourceID),
			URL:          it.URL,
			Title:        it.Title,
			ThumbnailURL: &thumb,
			Status:       models.MangaStatusFromInt(it.Status),
		})
	}

	return models.PagedMangaListDataClass{
		MangaList:   resultList,
		HasNextPage: hasNext,
	}
}

// GetPopularManga retrieves popular mangas from a specific source using native Go scraper or the JS extension engine.
func GetPopularManga(c *fiber.Ctx) error {
	sourceId := c.Params("sourceId")
	pageNum, err := strconv.Atoi(c.Params("pageNum"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid page number")
	}

	if sid, err := strconv.ParseInt(sourceId, 10, 64); err == nil {
		if h, ok := sources.Get(sid); ok {
			if pop, ok := h.(sources.PopularLister); ok {
				items, hasNext, errPop := pop.GetPopularManga(c.Context(), pageNum)
				if errPop == nil && len(items) > 0 {
					return c.JSON(saveAndFormatMangaList(c.Context(), sid, items, hasNext))
				}
			}
		}
	}

	resVal, err := extensions.GlobalEngine.RunExtensionMethod(sourceId, "getPopularManga", pageNum)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("JS Extension error: " + err.Error())
	}

	return c.JSON(resVal.Export())
}

// GetLatestManga retrieves latest mangas from a specific source using native Go scraper or the JS extension engine.
func GetLatestManga(c *fiber.Ctx) error {
	sourceId := c.Params("sourceId")
	pageNum, err := strconv.Atoi(c.Params("pageNum"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid page number")
	}

	if sid, err := strconv.ParseInt(sourceId, 10, 64); err == nil {
		if h, ok := sources.Get(sid); ok {
			if latest, ok := h.(sources.LatestLister); ok {
				items, hasNext, errLat := latest.GetLatestManga(c.Context(), pageNum)
				if errLat == nil && len(items) > 0 {
					return c.JSON(saveAndFormatMangaList(c.Context(), sid, items, hasNext))
				}
			} else if pop, ok := h.(sources.PopularLister); ok {
				items, hasNext, errPop := pop.GetPopularManga(c.Context(), pageNum)
				if errPop == nil && len(items) > 0 {
					return c.JSON(saveAndFormatMangaList(c.Context(), sid, items, hasNext))
				}
			}
		}
	}

	resVal, err := extensions.GlobalEngine.RunExtensionMethod(sourceId, "getLatestManga", pageNum)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("JS Extension error: " + err.Error())
	}

	return c.JSON(resVal.Export())
}

// GetSourcePreferences retrieves current config preferences for a source.
func GetSourcePreferences(c *fiber.Ctx) error {
	var list []models.PreferenceObject
	return c.JSON(list)
}

// SetSourcePreference sets preference value for a source preference key.
func SetSourcePreference(c *fiber.Ctx) error {
	var body models.SourcePreferenceChange
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	return c.SendStatus(fiber.StatusOK)
}

// GetSourceFilters gets all custom filters configuration of a source.
func GetSourceFilters(c *fiber.Ctx) error {
	var list []models.FilterObject
	return c.JSON(list)
}

// SetSourceFilters saves custom filter selections for a source.
func SetSourceFilters(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// SearchManga performs search operation on a source using native Go scraper or the JS extension engine.
func SearchManga(c *fiber.Ctx) error {
	sourceId := c.Params("sourceId")
	searchTerm := c.Query("searchTerm")
	if searchTerm == "" {
		searchTerm = c.Query("query")
	}
	pageNum, err := strconv.Atoi(c.Query("pageNum", "1"))
	if err != nil {
		pageNum = 1
	}

	if sid, err := strconv.ParseInt(sourceId, 10, 64); err == nil {
		if h, ok := sources.Get(sid); ok {
			if searcher, ok := h.(sources.MangaSearcher); ok {
				items, hasNext, errSrc := searcher.SearchManga(c.Context(), searchTerm, pageNum)
				if errSrc == nil && len(items) > 0 {
					return c.JSON(saveAndFormatMangaList(c.Context(), sid, items, hasNext))
				}
			}
		}
	}

	resVal, err := extensions.GlobalEngine.RunExtensionMethod(sourceId, "searchManga", pageNum, searchTerm)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("JS Extension error: " + err.Error())
	}

	return c.JSON(resVal.Export())
}

// QuickSearchManga performs composite filter quick search on a source.
func QuickSearchManga(c *fiber.Ctx) error {
	var body models.FilterData
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	return c.JSON(models.PagedMangaListDataClass{})
}
