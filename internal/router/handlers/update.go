package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/service"
)

// GetRecentChapters retrieves paginated list of recently updated chapters.
func GetRecentChapters(c *fiber.Ctx) error {
	pageNum, err := strconv.Atoi(c.Params("pageNum"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid page number")
	}

	list, err := database.GetRecentChapters(c.Context(), pageNum)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(list)
}

// StartUpdater schedules library check updates.
func StartUpdater(c *fiber.Ctx) error {
	service.GlobalUpdateManager.StartUpdate()
	return c.SendStatus(fiber.StatusOK)
}

// ResetUpdater aborts running updates check.
func ResetUpdater(c *fiber.Ctx) error {
	service.GlobalUpdateManager.CancelUpdate()
	return c.SendStatus(fiber.StatusOK)
}

// GetUpdaterSummary returns simple statistics on update state.
func GetUpdaterSummary(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"running": service.GlobalUpdateManager.IsRunning(),
	})
}

// UpdatesWS handles update updates web socket updates.
func UpdatesWS(conn *websocket.Conn) {
	service.GlobalUpdateManager.AddClient(conn)

	// Keep connection alive
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
