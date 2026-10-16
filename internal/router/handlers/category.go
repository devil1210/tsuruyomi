package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/database"
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
	nameVal := c.FormValue("name")
	if nameVal != "" {
		namePtr = &nameVal
	}

	var defaultPtr *bool
	defVal := c.FormValue("default")
	if defVal != "" {
		b, err := strconv.ParseBool(defVal)
		if err == nil {
			defaultPtr = &b
		}
	}

	var includeInUpdatePtr *int
	upVal := c.FormValue("includeInUpdate")
	if upVal != "" {
		i, err := strconv.Atoi(upVal)
		if err == nil {
			includeInUpdatePtr = &i
		}
	}

	var includeInDownloadPtr *int
	downVal := c.FormValue("includeInDownload")
	if downVal != "" {
		i, err := strconv.Atoi(downVal)
		if err == nil {
			includeInDownloadPtr = &i
		}
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
	return c.SendStatus(fiber.StatusOK)
}
