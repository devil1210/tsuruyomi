package handlers

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestGetFallbackIconURL(t *testing.T) {
	tests := []struct {
		pkgName     string
		expectedSub string
	}{
		{"eu.kanade.tachiyomi.extension.all.mangadex", "mangadex"},
		{"eu.kanade.tachiyomi.extension.es.mangamx", "mangamx"},
		{"eu.kanade.tachiyomi.extension.es.lectormangalat", "lectormangalat"},
		{"eu.kanade.tachiyomi.extension.es.zonatmoto", "zonatmoto"},
		{"eu.kanade.tachiyomi.extension.es.tumangaonline", "zonatmoto"},
		{"eu.kanade.tachiyomi.extension.es.inmanga", "inmanga"},
		{"eu.kanade.tachiyomi.extension.es.manhuaonline", "manhuaonline"},
		{"eu.kanade.tachiyomi.extension.es.randomcustomscan", "randomcustomscan"},
	}

	for _, tt := range tests {
		url := getFallbackIconURL(tt.pkgName)
		if !strings.Contains(url, tt.expectedSub) {
			t.Errorf("getFallbackIconURL(%q) = %q; expected to contain %q", tt.pkgName, url, tt.expectedSub)
		}
	}
}

func TestDefaultIconFallback(t *testing.T) {
	app := fiber.New()
	app.Get("/api/v1/source/:sourceId/icon", GetSourceIcon)
	app.Get("/api/v1/extension/icon/:pkgName", GetExtensionIcon)

	// Test unknown source returns valid embedded PNG (defaultIconPNG)
	req := httptest.NewRequest("GET", "/api/v1/source/9999999999999999/icon", nil)
	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "image/png" {
		t.Errorf("expected Content-Type image/png, got %s", resp.Header.Get("Content-Type"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	// Should be > 100 bytes (our embedded 96x96 PNG is ~10KB)
	if len(body) < 100 {
		t.Errorf("expected real PNG icon, got only %d bytes", len(body))
	}
}
