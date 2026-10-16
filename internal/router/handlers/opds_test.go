package handlers

import (
	"encoding/xml"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestOpdsRootFeed(t *testing.T) {
	app := fiber.New()
	app.Get("/api/opds/v1.2", GetOpdsRoot)

	req := httptest.NewRequest("GET", "/api/opds/v1.2", nil)
	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/atom+xml") || !strings.Contains(contentType, "kind=navigation") {
		t.Errorf("expected navigation atom XML content type, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var feed OpdsFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		t.Fatalf("failed to parse returned OPDS XML: %v", err)
	}

	if feed.Title != "Tsuruyomi Catalog" {
		t.Errorf("expected feed title 'Tsuruyomi Catalog', got %q", feed.Title)
	}

	if len(feed.Entries) < 5 {
		t.Errorf("expected at least 5 navigation entries, got %d", len(feed.Entries))
	}
}

func TestOpdsSearchDescription(t *testing.T) {
	app := fiber.New()
	app.Get("/api/opds/v1.2/search", GetOpdsSearch)

	req := httptest.NewRequest("GET", "/api/opds/v1.2/search", nil)
	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/opensearchdescription+xml") {
		t.Errorf("expected opensearch XML content type, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	var desc OpenSearchDescription
	if err := xml.Unmarshal(body, &desc); err != nil {
		t.Fatalf("failed to unmarshal OpenSearchDescription: %v", err)
	}

	if desc.ShortName != "Tsuruyomi" {
		t.Errorf("expected ShortName 'Tsuruyomi', got %q", desc.ShortName)
	}
}
