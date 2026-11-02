package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"tsuruyomi/internal/config"
)

func TestHandleGraphQL_CategoryMetaMutations(t *testing.T) {
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

	tests := []struct {
		name              string
		operationName     string
		query             string
		variables         map[string]interface{}
		expectedKey       string
		expectedCategory  float64
		expectedMetaKey   string
	}{
		{
			name:          "SetCategoryMeta with categoryId 0 (Default category)",
			operationName: "SetCategoryMeta",
			query:         "mutation SetCategoryMeta($input: SetCategoryMetaInput!) { setCategoryMeta(input: $input) { meta { categoryId key value } } }",
			variables: map[string]interface{}{
				"input": map[string]interface{}{
					"meta": map[string]interface{}{
						"categoryId": float64(0),
						"key":        "tsumiru.hidden",
						"value":      "true",
					},
				},
			},
			expectedKey:      "setCategoryMeta",
			expectedCategory: 0,
			expectedMetaKey:  "tsumiru.hidden",
		},
		{
			name:          "DeleteCategoryMeta with categoryId 0 (Default category)",
			operationName: "DeleteCategoryMeta",
			query:         "mutation DeleteCategoryMeta($input: DeleteCategoryMetaInput!) { deleteCategoryMeta(input: $input) { meta { categoryId key value } } }",
			variables: map[string]interface{}{
				"input": map[string]interface{}{
					"categoryId": float64(0),
					"key":        "tsumiru.hidden",
				},
			},
			expectedKey:      "deleteCategoryMeta",
			expectedCategory: 0,
			expectedMetaKey:  "tsumiru.hidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"operationName": tt.operationName,
				"query":         tt.query,
				"variables":     tt.variables,
			}
			bodyBytes, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/api/graphql", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			if resp.StatusCode != 200 {
				t.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}

			var respBody map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			data, ok := respBody["data"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected 'data' map in response, got %v", respBody)
			}

			payload, ok := data[tt.expectedKey].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected payload under '%s', got %v", tt.expectedKey, data)
			}

			meta, ok := payload["meta"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected 'meta' in payload, got %v", payload)
			}

			catID, ok := meta["categoryId"].(float64)
			if !ok || catID != tt.expectedCategory {
				t.Errorf("Expected categoryId %v, got %v", tt.expectedCategory, meta["categoryId"])
			}

			metaKey, ok := meta["key"].(string)
			if !ok || metaKey != tt.expectedMetaKey {
				t.Errorf("Expected key %v, got %v", tt.expectedMetaKey, meta["key"])
			}
		})
	}
}
