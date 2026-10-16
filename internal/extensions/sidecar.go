package extensions

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

var httpClient = &http.Client{
	Timeout: 60 * time.Second,
}

// ForwardToSidecar forwards the incoming Fiber request to the configured Sidecar JVM endpoint.
// It copies headers and streams the response body back to the client for memory efficiency.
func ForwardToSidecar(c *fiber.Ctx, baseURL string) error {
	// Construct sidecar URL (e.g. baseURL + "/api/v1/...")
	targetURL := baseURL + c.OriginalURL()

	var bodyReader io.Reader
	if len(c.Body()) > 0 {
		bodyReader = bytes.NewReader(c.Body())
	}

	// Create request
	req, err := http.NewRequest(c.Method(), targetURL, bodyReader)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to build request to sidecar: %v", err))
	}

	// Copy headers
	c.Request().Header.VisitAll(func(key, value []byte) {
		req.Header.Set(string(key), string(value))
	})

	// Perform request
	resp, err := httpClient.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).SendString(fmt.Sprintf("Sidecar connection error: %v", err))
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Set(k, v)
		}
	}

	// Set status
	c.Status(resp.StatusCode)

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to read response from sidecar: %v", err))
	}
	return c.Send(data)
}
