package handlers

import (
	"github.com/gofiber/fiber/v2"

	apperrors "tsuruyomi/internal/errors"
	"tsuruyomi/internal/service"
)

// SystemAuditHandler returns a health and integrity audit across all sources and chapters.
func SystemAuditHandler(c *fiber.Ctx) error {
	report, err := service.Audit(c.Context())
	if err != nil {
		return apperrors.Internal("SYSTEM_AUDIT_FAILED", "Failed to run system audit", err)
	}
	return c.JSON(report)
}

// SystemHealHandler executes automated self-healing across all sources and chapters.
func SystemHealHandler(c *fiber.Ctx) error {
	report, err := service.Heal(c.Context())
	if err != nil {
		return apperrors.Internal("SYSTEM_HEAL_FAILED", "Failed to run system heal", err)
	}
	return c.JSON(report)
}
