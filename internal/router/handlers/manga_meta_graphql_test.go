package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"tsuruyomi/internal/config"
)

func TestHandleGraphQL_MangaMetaMutations(t *testing.T) {
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

	t.Run("SetMangaMeta with readerMode", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"operationName": "SetMangaMeta",
			"query":         "mutation SetMangaMeta($input: SetMangaMetaInput!) { setMangaMeta(input: $input) { meta { mangaId key value } } }",
			"variables": map[string]interface{}{
				"input": map[string]interface{}{
					"meta": map[string]interface{}{
						"mangaId": float64(1547),
						"key":     "flutter_readerMode",
						"value":   "singleHorizontalLTR",
					},
				},
			},
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}
		req := httptest.NewRequest("POST", "/api/graphql", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("Expected 200, got %d, err: %v", resp.StatusCode, err)
		}

		var respBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&respBody)
		data := respBody["data"].(map[string]interface{})
		payload := data["setMangaMeta"].(map[string]interface{})
		meta := payload["meta"].(map[string]interface{})
		if meta["mangaId"] != float64(1547) {
			t.Errorf("Expected mangaId 1547, got %v", meta["mangaId"])
		}
		if meta["key"] != "flutter_readerMode" {
			t.Errorf("Expected key flutter_readerMode, got %v", meta["key"])
		}
		if meta["value"] != "singleHorizontalLTR" {
			t.Errorf("Expected value singleHorizontalLTR, got %v", meta["value"])
		}
	})

	t.Run("DeleteMangaMeta", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"operationName": "DeleteMangaMeta",
			"query":         "mutation DeleteMangaMeta($input: DeleteMangaMetaInput!) { deleteMangaMeta(input: $input) { meta { mangaId key value } } }",
			"variables": map[string]interface{}{
				"input": map[string]interface{}{
					"mangaId": float64(1547),
					"key":     "flutter_readerMode",
				},
			},
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}
		req := httptest.NewRequest("POST", "/api/graphql", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("Expected 200, got %d, err: %v", resp.StatusCode, err)
		}

		var respBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&respBody)
		data := respBody["data"].(map[string]interface{})
		payload := data["deleteMangaMeta"].(map[string]interface{})
		meta := payload["meta"].(map[string]interface{})
		if meta["mangaId"] != float64(1547) {
			t.Errorf("Expected mangaId 1547, got %v", meta["mangaId"])
		}
		if meta["key"] != "flutter_readerMode" {
			t.Errorf("Expected key flutter_readerMode, got %v", meta["key"])
		}
	})

	t.Run("SetMangaMetas batch", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"operationName": "SetMangaMetas",
			"query":         "mutation SetMangaMetas($input: SetMangaMetasInput!) { setMangaMetas(input: $input) { metas { mangaId key value } } }",
			"variables": map[string]interface{}{
				"input": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{
							"mangaIds": []interface{}{float64(1547)},
							"metas": []interface{}{
								map[string]interface{}{
									"key":   "flutter_readerMode",
									"value": "webtoon",
								},
							},
						},
					},
				},
			},
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}
		req := httptest.NewRequest("POST", "/api/graphql", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("Expected 200, got %d, err: %v", resp.StatusCode, err)
		}

		var respBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&respBody)
		data := respBody["data"].(map[string]interface{})
		payload := data["setMangaMetas"].(map[string]interface{})
		metas := payload["metas"].([]interface{})
		if len(metas) != 1 {
			t.Fatalf("Expected 1 meta, got %d", len(metas))
		}
	})

	t.Run("DeleteMangaMetas batch", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"operationName": "DeleteMangaMetas",
			"query":         "mutation DeleteMangaMetas($input: DeleteMangaMetasInput!) { deleteMangaMetas(input: $input) { metas { mangaId key value } } }",
			"variables": map[string]interface{}{
				"input": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{
							"mangaIds": []interface{}{float64(1547)},
							"keys":     []interface{}{"flutter_readerMode"},
						},
					},
				},
			},
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}
		req := httptest.NewRequest("POST", "/api/graphql", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("Expected 200, got %d, err: %v", resp.StatusCode, err)
		}

		var respBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&respBody)
		data := respBody["data"].(map[string]interface{})
		payload := data["deleteMangaMetas"].(map[string]interface{})
		metas := payload["metas"].([]interface{})
		if len(metas) != 1 {
			t.Fatalf("Expected 1 meta, got %d", len(metas))
		}
	})
}
