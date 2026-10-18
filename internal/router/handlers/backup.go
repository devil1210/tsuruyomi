package handlers

import (
	"fmt"
	"io"
	"time"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/service"
)

// ImportBackup handles importing a backup from a URL or body payload.
func ImportBackup(c *fiber.Ctx) error {
	body := c.Body()
	if len(body) == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Empty backup payload")
	}

	err := service.ImportBackup(c.Context(), body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

// ImportBackupFile handles importing a backup via file upload.
func ImportBackupFile(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Failed to get file: " + err.Error())
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open file: " + err.Error())
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read file: " + err.Error())
	}

	err = service.ImportBackup(c.Context(), data)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

// ValidateBackup validates backup content from a URL or body payload.
func ValidateBackup(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK) // Stub success
}

// ValidateBackupFile validates backup content from an uploaded file.
func ValidateBackupFile(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK) // Stub success
}

// ExportBackup exports backup data.
func ExportBackup(c *fiber.Ctx) error {
	data, err := service.ExportBackup(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Set(fiber.HeaderContentType, "application/json")
	return c.Send(data)
}

// ExportBackupFile exports backup file attachment.
func ExportBackupFile(c *fiber.Ctx) error {
	data, err := service.ExportBackup(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	filename := fmt.Sprintf("tsuruyomi_backup_%s.json", time.Now().Format("2006-01-02_150405"))
	c.Set(fiber.HeaderContentType, "application/json")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}
