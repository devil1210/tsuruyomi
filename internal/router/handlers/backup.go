package handlers

import (
	"fmt"
	"io"
	"time"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/service"
)

// ImportBackup handles importing a backup from a URL or body payload asynchronously.
func ImportBackup(c *fiber.Ctx) error {
	body := c.Body()
	if len(body) == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Empty backup payload")
	}

	jobID, totalManga, err := service.GlobalRestoreManager.StartAsyncRestore(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(fiber.Map{
		"id":         jobID,
		"state":      "RUNNING",
		"totalManga": totalManga,
	})
}

// ImportBackupFile handles importing a backup via file upload asynchronously.
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

	jobID, totalManga, err := service.GlobalRestoreManager.StartAsyncRestore(data)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(fiber.Map{
		"id":         jobID,
		"state":      "RUNNING",
		"totalManga": totalManga,
	})
}

// GetRestoreStatus returns current status of backup restoration.
func GetRestoreStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	st := service.GlobalRestoreManager.GetStatus(id)
	return c.JSON(st)
}

// ValidateBackup validates backup content from a URL or body payload.
func ValidateBackup(c *fiber.Ctx) error {
	res, err := service.ValidateBackup(c.Context(), c.Body())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(res)
}

// ValidateBackupFile validates backup content from an uploaded file.
func ValidateBackupFile(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to get file: " + err.Error()})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open file: " + err.Error()})
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file: " + err.Error()})
	}

	res, err := service.ValidateBackup(c.Context(), data)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(res)
}

// ExportBackup exports backup data.
func ExportBackup(c *fiber.Ctx) error {
	data, err := service.ExportBackup(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Set(fiber.HeaderContentType, "application/gzip")
	return c.Send(data)
}

// ExportBackupFile exports backup file attachment in standard .tachibk format.
func ExportBackupFile(c *fiber.Ctx) error {
	data, err := service.ExportBackup(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	filename := fmt.Sprintf("tsuruyomi_backup_%s.tachibk", time.Now().Format("2006-01-02_150405"))
	c.Set(fiber.HeaderContentType, "application/gzip")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// GetBackupFileByToken retrieves an exported backup file using its temporary opaque token (Suwayomi PR #2509).
func GetBackupFileByToken(c *fiber.Ctx) error {
	token := c.Params("token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Token required")
	}

	stored, ok := service.GlobalTempFileStorage.RetrieveFile(token)
	if !ok || stored == nil {
		return c.Status(fiber.StatusNotFound).SendString("Backup file not found or expired")
	}

	c.Set(fiber.HeaderContentType, "application/octet-stream")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, stored.Filename))
	return c.SendFile(stored.Path)
}

// ReconcileDownloadsHandler runs storage reconciliation between disk files and database.
func ReconcileDownloadsHandler(c *fiber.Ctx) error {
	cfg := c.Locals("config").(*config.Config)
	result, err := service.ReconcileDownloads(c.Context(), cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Reconciliation failed",
			"message": err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"result":  result,
	})
}
