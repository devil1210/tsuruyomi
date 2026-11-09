package handlers

import (
	"github.com/gofiber/fiber/v2"
	"tsuruyomi/internal/service"
)

// SystemAuditHandler returns a health and integrity audit across all sources and chapters.
func SystemAuditHandler(c *fiber.Ctx) error {
	report, err := service.Audit(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(report)
}

// SystemHealHandler executes automated self-healing across all sources and chapters.
func SystemHealHandler(c *fiber.Ctx) error {
	report, err := service.Heal(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(report)
}
