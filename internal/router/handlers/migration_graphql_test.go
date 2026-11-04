package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"tsuruyomi/internal/config"
)

func TestHandleGraphQL_MigrationOperations(t *testing.T) {
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

	t.Run("UPDATE_CHAPTER_METADATA response structure", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"operationName": "UPDATE_CHAPTER_METADATA",
			"query":         "mutation UPDATE_CHAPTER_METADATA($hasUpdates: Boolean!, $updates: SetChapterMetasInput, $hasPreUpdateDeletions: Boolean!, $preUpdateDeletions: DeleteChapterMetasInput, $hasPostUpdateDeletions: Boolean!, $postUpdateDeletions: DeleteChapterMetasInput, $isMigration: Boolean!, $migrationUpdates: SetChapterMetasInput) { preUpdateDeletedMeta: deleteChapterMetas(input: $preUpdateDeletions) @include(if: $hasPreUpdateDeletions) { metas { chapterId key value } } updatedMeta: setChapterMetas(input: $updates) @include(if: $hasUpdates) { metas { chapterId key value } } postUpdateDeletedMeta: deleteChapterMetas(input: $postUpdateDeletions) @include(if: $hasPostUpdateDeletions) { metas { chapterId key value } } migrationMeta: setChapterMetas(input: $migrationUpdates) @include(if: $isMigration) { metas { chapterId key value } } }",
			"variables": map[string]interface{}{
				"hasUpdates":              true,
				"hasPreUpdateDeletions":   false,
				"hasPostUpdateDeletions":  false,
				"isMigration":             true,
				"updates": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{
							"chapterIds": []interface{}{int(101), int64(102), float64(103), "104"},
							"metas": []interface{}{
								map[string]interface{}{
									"key":   "koreader_read",
									"value": "true",
								},
							},
						},
					},
				},
				"migrationUpdates": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{
							"chapterIds": []interface{}{int(201)},
							"metas": []interface{}{
								map[string]interface{}{
									"key":   "migrated_from",
									"value": "101",
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
		data, ok := respBody["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data object in response, got %v", respBody)
		}

		// Verify updatedMeta is present
		if _, ok := data["updatedMeta"]; !ok {
			t.Errorf("Expected updatedMeta in response data")
		}
		// Verify migrationMeta is present because isMigration = true
		if _, ok := data["migrationMeta"]; !ok {
			t.Errorf("Expected migrationMeta in response data")
		}
	})

	t.Run("UPDATE_MANGA with updateCategories true returns updateMangaCategories", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"operationName": "UPDATE_MANGA",
			"query":         "mutation UPDATE_MANGA($input: UpdateMangaInput!, $updateCategories: Boolean!) { updateManga(input: $input) { manga { id title inLibrary } } updateMangaCategories(input: $input) @include(if: $updateCategories) { manga { id categories { nodes { id name } } } } }",
			"variables": map[string]interface{}{
				"updateCategories": true,
				"input": map[string]interface{}{
					"id": 99999,
					"patch": map[string]interface{}{
						"inLibrary": true,
					},
					"updateMangaCategories": map[string]interface{}{
						"addToCategories": []interface{}{int(1), "2", float64(3)},
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
		data, ok := respBody["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data object in response, got %v", respBody)
		}

		// updateMangaCategories MUST be present because $updateCategories is true
		catPayload, ok := data["updateMangaCategories"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected updateMangaCategories in response data, got %v", data)
		}
		mangaObj, ok := catPayload["manga"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected manga in updateMangaCategories, got %v", catPayload)
		}
		if mangaObj["id"] != float64(99999) && mangaObj["id"] != int64(99999) && mangaObj["id"] != 99999 {
			t.Errorf("Expected manga id 99999, got %v", mangaObj["id"])
		}
	})

	t.Run("GET_MANGA_TO_MIGRATE fallback empty categories and trackRecords", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"operationName": "GET_MANGA_TO_MIGRATE",
			"query":         "query GET_MANGA_TO_MIGRATE($id: Int!) { manga(id: $id) { id title categories { nodes { id } } trackRecords { nodes { id } } chapters { nodes { id } } } }",
			"variables": map[string]interface{}{
				"id": "12345",
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
		data, ok := respBody["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data object in response, got %v", respBody)
		}

		mangaObj, ok := data["manga"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected manga in response data, got %v", data)
		}

		// categories must have nodes array, never null (prevents 'Categories are missing' crash in webui)
		cats, ok := mangaObj["categories"].(map[string]interface{})
		if !ok || cats["nodes"] == nil {
			t.Errorf("Expected non-nil categories.nodes, got %v", mangaObj["categories"])
		}

		// trackRecords must have nodes array, never null
		tracks, ok := mangaObj["trackRecords"].(map[string]interface{})
		if !ok || tracks["nodes"] == nil {
			t.Errorf("Expected non-nil trackRecords.nodes, got %v", mangaObj["trackRecords"])
		}
	})
}
