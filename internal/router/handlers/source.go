package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/extensions"
	"tsuruyomi/internal/models"
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

// GetPopularManga retrieves popular mangas from a specific source using the JS extension engine.
func GetPopularManga(c *fiber.Ctx) error {
	sourceId := c.Params("sourceId")
	pageNum, err := strconv.Atoi(c.Params("pageNum"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid page number")
	}

	resVal, err := extensions.GlobalEngine.RunExtensionMethod(sourceId, "getPopularManga", pageNum)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("JS Extension error: " + err.Error())
	}

	return c.JSON(resVal.Export())
}

// GetLatestManga retrieves latest mangas from a specific source using the JS extension engine.
func GetLatestManga(c *fiber.Ctx) error {
	sourceId := c.Params("sourceId")
	pageNum, err := strconv.Atoi(c.Params("pageNum"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid page number")
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

// SearchManga performs search operation on a source using the JS extension engine.
func SearchManga(c *fiber.Ctx) error {
	sourceId := c.Params("sourceId")
	searchTerm := c.Query("searchTerm")
	pageNum, err := strconv.Atoi(c.Query("pageNum", "1"))
	if err != nil {
		pageNum = 1
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
