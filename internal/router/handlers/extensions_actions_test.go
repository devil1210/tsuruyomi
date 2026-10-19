package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"tsuruyomi/internal/config"
)

func TestHandleGraphQL_UpdateExtensionInput(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{
		Data: config.DataConfig{
			RootDir: t.TempDir(),
		},
	}
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("config", cfg)
		return c.Next()
	})
	app.Post("/api/graphql", HandleGraphQL)

	// Test 1: Uninstall mutation with Suwayomi-WebUI format: { input: { id: "pkg", patch: { uninstall: true } } }
	payload := map[string]interface{}{
		"operationName": "UPDATE_EXTENSION",
		"variables": map[string]interface{}{
			"input": map[string]interface{}{
				"id": "eu.kanade.tachiyomi.extension.es.mangamx",
				"patch": map[string]interface{}{
					"uninstall": true,
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var resMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&resMap); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	data, ok := resMap["data"].(map[string]interface{})
	if !ok || data == nil {
		t.Fatalf("missing data in response: %v", resMap)
	}

	updateExt, ok := data["updateExtension"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing updateExtension in data: %v", data)
	}

	ext, ok := updateExt["extension"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing extension in updateExtension: %v", updateExt)
	}

	if ext["pkgName"] != "eu.kanade.tachiyomi.extension.es.mangamx" {
		t.Errorf("expected pkgName eu.kanade.tachiyomi.extension.es.mangamx, got %v", ext["pkgName"])
	}
}

func TestHandleGraphQL_GetSourceMangasFetchStructure(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{
		Data: config.DataConfig{
			RootDir: t.TempDir(),
		},
		Sidecar: config.SidecarConfig{
			Enabled: false,
		},
	}
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("config", cfg)
		return c.Next()
	})
	app.Post("/api/graphql", HandleGraphQL)

	payload := map[string]interface{}{
		"operationName": "GET_SOURCE_MANGAS_FETCH",
		"variables": map[string]interface{}{
			"input": map[string]interface{}{
				"source": "2202687009511923782",
				"type":   "POPULAR",
				"page":   1,
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var resMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&resMap); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	data, ok := resMap["data"].(map[string]interface{})
	if !ok || data == nil {
		t.Fatalf("missing data in response: %v", resMap)
	}

	fetchPayload, ok := data["fetchSourceManga"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing fetchSourceManga in data: %v", data)
	}

	if _, hasMangas := fetchPayload["mangas"]; !hasMangas {
		t.Fatalf("missing mangas array in fetchSourceManga: %v", fetchPayload)
	}
}
