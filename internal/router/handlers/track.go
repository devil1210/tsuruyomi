package handlers

import (
	"github.com/gofiber/fiber/v2"
	"tsuruyomi/internal/models"
)

// ListTrackers returns tracking configurations.
func ListTrackers(c *fiber.Ctx) error {
	list := []models.TrackerDataClass{
		{ID: 1, Name: "MyAnimeList"},
		{ID: 2, Name: "AniList"},
		{ID: 3, Name: "Kitsu"},
		{ID: 4, Name: "Shikimori"},
		{ID: 5, Name: "Bangumi"},
	}
	return c.JSON(list)
}

// TrackerLogin updates tracker session details.
func TrackerLogin(c *fiber.Ctx) error {
	// Accept credentials and stub success
	return c.SendStatus(fiber.StatusOK)
}

// TrackerLogout clears tracker credentials.
func TrackerLogout(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// TrackerSearch searches catalog matching track search options.
func TrackerSearch(c *fiber.Ctx) error {
	return c.JSON([]models.TrackRecordDataClass{})
}

// TrackerBindManga links manga records to tracking items.
func TrackerBindManga(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// TrackerUpdateProgress updates read chapters values on tracker server.
func TrackerUpdateProgress(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

// GetTrackerThumbnail serves the tracker custom icon.
func GetTrackerThumbnail(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "image/png")
	stub := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0xda, 0x63, 0x60, 0x60, 0x60, 0x60,
		0x00, 0x00, 0x00, 0x05, 0x00, 0x01, 0xa5, 0xf6, 0x1e, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
		0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	return c.Send(stub)
}
