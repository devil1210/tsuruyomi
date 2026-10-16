package handlers

import (
	"encoding/json"
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"tsuruyomi/internal/database"
	"tsuruyomi/internal/models"
	"tsuruyomi/internal/service"
)

// DownloadsWS processes WebSocket connection for download queue.
func DownloadsWS(conn *websocket.Conn) {
	service.GlobalDownloadManager.AddClient(conn)

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			break
		}

		msg := string(p)
		if msg == "STATUS" {
			status := service.GlobalDownloadManager.GetStatus()
			// Respond to client directly
			if resp, err := conn.Conn.NextWriter(messageType); err == nil {
				_ = jsonWrite(resp, status)
				resp.Close()
			}
		} else {
			_ = conn.WriteMessage(messageType, []byte("Invalid command.\nSupported commands are:\n    - STATUS\n       sends the current download status\n"))
		}
	}
}

func jsonWrite(w io.Writer, val interface{}) error {
	enc := json.NewEncoder(w)
	return enc.Encode(val)
}

// StartDownloader starts downloading queue processing.
func StartDownloader(c *fiber.Ctx) error {
	service.GlobalDownloadManager.Start()
	return c.SendStatus(fiber.StatusOK)
}

// StopDownloader pauses downloader execution.
func StopDownloader(c *fiber.Ctx) error {
	service.GlobalDownloadManager.Stop()
	return c.SendStatus(fiber.StatusOK)
}

// ClearDownloaderQueue purges all items from download queue.
func ClearDownloaderQueue(c *fiber.Ctx) error {
	service.GlobalDownloadManager.Clear()
	return c.SendStatus(fiber.StatusOK)
}

// QueueChapter adds a chapter to downloads queue.
func QueueChapter(c *fiber.Ctx) error {
	mangaId, err1 := strconv.Atoi(c.Params("mangaId"))
	chapterIndex, err2 := strconv.Atoi(c.Params("chapterIndex"))
	if err1 != nil || err2 != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga or chapter parameter")
	}

	// Retrieve chapter ID from DB
	var chID int
	query := `SELECT id FROM suwayomi.chapter WHERE manga = $1 AND source_order = $2 LIMIT 1`
	err := database.DB.QueryRow(c.Context(), query, mangaId, chapterIndex).Scan(&chID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Chapter not found in database")
	}

	err = service.GlobalDownloadManager.Enqueue(c.Context(), []int{chID})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

// UnqueueChapter deletes an item from downloads queue.
func UnqueueChapter(c *fiber.Ctx) error {
	mangaId, err1 := strconv.Atoi(c.Params("mangaId"))
	chapterIndex, err2 := strconv.Atoi(c.Params("chapterIndex"))
	if err1 != nil || err2 != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manga or chapter parameter")
	}

	service.GlobalDownloadManager.DequeueByIndex(c.Context(), mangaId, chapterIndex)
	return c.SendStatus(fiber.StatusOK)
}

// ReorderChapterInQueue moves download priority of a queued chapter.
func ReorderChapterInQueue(c *fiber.Ctx) error {
	mangaId, err1 := strconv.Atoi(c.Params("mangaId"))
	chapterIndex, err2 := strconv.Atoi(c.Params("chapterIndex"))
	to, err3 := strconv.Atoi(c.Params("to"))
	if err1 != nil || err2 != nil || err3 != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid parameter values")
	}

	service.GlobalDownloadManager.Reorder(mangaId, chapterIndex, to)
	return c.SendStatus(fiber.StatusOK)
}

// QueueChaptersBatch inserts multiple chapters to queue.
func QueueChaptersBatch(c *fiber.Ctx) error {
	var body models.EnqueueInput
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	err := service.GlobalDownloadManager.Enqueue(c.Context(), body.ChapterIds)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

// UnqueueChaptersBatch removes multiple chapters from queue.
func UnqueueChaptersBatch(c *fiber.Ctx) error {
	var body models.EnqueueInput
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	service.GlobalDownloadManager.Dequeue(body.ChapterIds)
	return c.SendStatus(fiber.StatusOK)
}
