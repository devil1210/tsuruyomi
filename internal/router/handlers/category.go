package handlers

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
)

// ListCategories returns all categories.
func ListCategories(c *fiber.Ctx) error {
	list, err := database.GetCategoryList(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.JSON(list)
}

// CreateCategory adds a new manga category.
func CreateCategory(c *fiber.Ctx) error {
	name := c.FormValue("name")
	if name == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing name parameter")
	}

	id, err := database.CreateCategory(c.Context(), name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	_ = id // Success, return 200 StatusOK
	return c.SendStatus(fiber.StatusOK)
}

// ReorderCategories shifts categories sorting order.
func ReorderCategories(c *fiber.Ctx) error {
	fromStr := c.FormValue("from")
	toStr := c.FormValue("to")
	if fromStr == "" || toStr == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing from/to parameters")
	}

	from, err1 := strconv.Atoi(fromStr)
	to, err2 := strconv.Atoi(toStr)
	if err1 != nil || err2 != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid from/to parameters")
	}

	err := database.ReorderCategory(c.Context(), from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

// GetCategoryMangas lists all mangas attached to a category.
func GetCategoryMangas(c *fiber.Ctx) error {
	categoryId, err := strconv.Atoi(c.Params("categoryId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid category ID")
	}

	list, err := database.GetCategoryMangas(c.Context(), categoryId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(list)
}

// ModifyCategory modifies category settings.
func ModifyCategory(c *fiber.Ctx) error {
	categoryId, err := strconv.Atoi(c.Params("categoryId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid category ID")
	}

	var namePtr *string
	var defaultPtr *bool
	var includeInUpdatePtr *int
	var includeInDownloadPtr *int

	// Try reading JSON body if present
	var jsonBody map[string]interface{}
	if err := c.BodyParser(&jsonBody); err == nil && len(jsonBody) > 0 {
		if n, ok := jsonBody["name"].(string); ok && n != "" {
			namePtr = &n
		}
		if d, ok := jsonBody["default"].(bool); ok {
			defaultPtr = &d
		}
		if u, ok := jsonBody["includeInUpdate"]; ok {
			status := int(models.ParseIncludeOrExclude(u))
			includeInUpdatePtr = &status
		}
		if d, ok := jsonBody["includeInDownload"]; ok {
			status := int(models.ParseIncludeOrExclude(d))
			includeInDownloadPtr = &status
		}
	}

	// Form values override if set
	if nameVal := c.FormValue("name"); nameVal != "" {
		namePtr = &nameVal
	}
	if defVal := c.FormValue("default"); defVal != "" {
		if b, err := strconv.ParseBool(defVal); err == nil {
			defaultPtr = &b
		}
	}
	if upVal := c.FormValue("includeInUpdate"); upVal != "" {
		status := int(models.ParseIncludeOrExclude(upVal))
		includeInUpdatePtr = &status
	}
	if downVal := c.FormValue("includeInDownload"); downVal != "" {
		status := int(models.ParseIncludeOrExclude(downVal))
		includeInDownloadPtr = &status
	}

	err = database.UpdateCategory(c.Context(), categoryId, namePtr, defaultPtr, includeInUpdatePtr, includeInDownloadPtr)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

// DeleteCategory deletes a category.
func DeleteCategory(c *fiber.Ctx) error {
	categoryId, err := strconv.Atoi(c.Params("categoryId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid category ID")
	}

	err = database.DeleteCategory(c.Context(), categoryId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

// ModifyCategoryMeta alters key-value properties of a category.
func ModifyCategoryMeta(c *fiber.Ctx) error {
	categoryId, err := strconv.Atoi(c.Params("categoryId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid category ID")
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	if database.DB != nil {
		for k, v := range body {
			var valStr string
			if s, ok := v.(string); ok {
				valStr = s
			} else {
				vBytes, _ := json.Marshal(v)
				valStr = string(vBytes)
			}
			_, _ = database.DB.Exec(c.Context(), `
				INSERT INTO suwayomi.categorymeta (user_id, category_ref, meta_key, value)
				VALUES (1, $1, $2, $3)
				ON CONFLICT (user_id, category_ref, meta_key) DO UPDATE SET value = EXCLUDED.value
			`, categoryId, k, valStr)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}
