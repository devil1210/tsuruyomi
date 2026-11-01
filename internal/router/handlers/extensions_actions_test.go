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

	// Test 2: Tsumiru format for UpdateExtension: variables: { id: "pkg", update: true }
	payloadTsumiru := map[string]interface{}{
		"operationName": "UpdateExtension",
		"query": "mutation UpdateExtension($id: String!, $update: Boolean = false) { updateExtension(input: {id: $id, patch: {update: $update}}) { extension { pkgName } } }",
		"variables": map[string]interface{}{
			"id":     "eu.kanade.tachiyomi.extension.es.submanhwa",
			"update": true,
		},
	}
	bodyTsumiru, _ := json.Marshal(payloadTsumiru)
	reqTsumiru := httptest.NewRequest("POST", "/api/graphql", bytes.NewReader(bodyTsumiru))
	reqTsumiru.Header.Set("Content-Type", "application/json")

	respTsumiru, err := app.Test(reqTsumiru)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	var resMapTsumiru map[string]interface{}
	if err := json.NewDecoder(respTsumiru.Body).Decode(&resMapTsumiru); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	dataTsumiru := resMapTsumiru["data"].(map[string]interface{})
	updateExtTsumiru := dataTsumiru["updateExtension"].(map[string]interface{})
	extTsumiru := updateExtTsumiru["extension"].(map[string]interface{})
	if extTsumiru["pkgName"] != "eu.kanade.tachiyomi.extension.es.submanhwa" {
		t.Errorf("expected pkgName eu.kanade.tachiyomi.extension.es.submanhwa, got %v", extTsumiru["pkgName"])
	}

	// Test 3: Tsumiru format for UpdateExtensions (batch): variables: { ids: ["pkg"], update: true }
	payloadBatch := map[string]interface{}{
		"operationName": "UpdateExtensions",
		"query": "mutation UpdateExtensions($ids: [String!]!, $update: Boolean = false) { updateExtensions(input: {ids: $ids, patch: {update: $update}}) { extensions { pkgName } } }",
		"variables": map[string]interface{}{
			"ids":    []string{"eu.kanade.tachiyomi.extension.es.dragontranslationorg"},
			"update": true,
		},
	}
	bodyBatch, _ := json.Marshal(payloadBatch)
	reqBatch := httptest.NewRequest("POST", "/api/graphql", bytes.NewReader(bodyBatch))
	reqBatch.Header.Set("Content-Type", "application/json")

	respBatch, err := app.Test(reqBatch)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	var resMapBatch map[string]interface{}
	if err := json.NewDecoder(respBatch.Body).Decode(&resMapBatch); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	dataBatch := resMapBatch["data"].(map[string]interface{})
	updateExtBatch := dataBatch["updateExtensions"].(map[string]interface{})
	extensionsBatch := updateExtBatch["extensions"].([]interface{})
	if len(extensionsBatch) == 0 {
		t.Fatalf("expected at least 1 extension in UpdateExtensions response, got 0")
	}
	firstExt := extensionsBatch[0].(map[string]interface{})
	if firstExt["pkgName"] != "eu.kanade.tachiyomi.extension.es.dragontranslationorg" {
		t.Errorf("expected pkgName eu.kanade.tachiyomi.extension.es.dragontranslationorg, got %v", firstExt["pkgName"])
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
