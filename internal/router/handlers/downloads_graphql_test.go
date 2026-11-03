package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"tsuruyomi/internal/config"
)

func TestHandleGraphQL_DownloadMutations(t *testing.T) {
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
		name          string
		operationName string
		query         string
		variables     map[string]interface{}
		expectedKey   string
	}{
		{
			name:          "EnqueueChapterDownloads PascalCase (Tsumiru format)",
			operationName: "EnqueueChapterDownloads",
			query:         "mutation EnqueueChapterDownloads($input: EnqueueChapterDownloadsInput!) { enqueueChapterDownloads(input: $input) { __typename } }",
			variables: map[string]interface{}{
				"input": map[string]interface{}{
					"ids": []interface{}{float64(94972), float64(94973)},
				},
			},
			expectedKey: "enqueueChapterDownloads",
		},
		{
			name:          "DequeueChapterDownloads PascalCase (Tsumiru single format)",
			operationName: "DequeueChapterDownloads",
			query:         "mutation DequeueChapterDownloads($input: DequeueChapterDownloadInput!) { dequeueChapterDownload(input: $input) { __typename } }",
			variables: map[string]interface{}{
				"input": map[string]interface{}{
					"id": float64(94972),
				},
			},
			expectedKey: "dequeueChapterDownload",
		},
		{
			name:          "DequeueChapterDownloadsBatch PascalCase (Tsumiru batch format)",
			operationName: "DequeueChapterDownloadsBatch",
			query:         "mutation DequeueChapterDownloadsBatch($input: DequeueChapterDownloadsInput!) { dequeueChapterDownloads(input: $input) { __typename } }",
			variables: map[string]interface{}{
				"input": map[string]interface{}{
					"ids": []interface{}{float64(94972), float64(94973)},
				},
			},
			expectedKey: "dequeueChapterDownloads",
		},
		{
			name:          "StartDownloader PascalCase",
			operationName: "StartDownloader",
			query:         "mutation StartDownloader($input: StartDownloaderInput!) { startDownloader(input: $input) { __typename } }",
			variables: map[string]interface{}{
				"input": map[string]interface{}{},
			},
			expectedKey: "startDownloader",
		},
		{
			name:          "StopDownloader PascalCase",
			operationName: "StopDownloader",
			query:         "mutation StopDownloader($input: StopDownloaderInput!) { stopDownloader(input: $input) { __typename } }",
			variables: map[string]interface{}{
				"input": map[string]interface{}{},
			},
			expectedKey: "stopDownloader",
		},
		{
			name:          "ClearDownloader PascalCase",
			operationName: "ClearDownloader",
			query:         "mutation ClearDownloader($input: ClearDownloaderInput!) { clearDownloader(input: $input) { __typename } }",
			variables: map[string]interface{}{
				"input": map[string]interface{}{},
			},
			expectedKey: "clearDownloader",
		},
		{
			name:          "ReorderChapterDownload PascalCase",
			operationName: "ReorderChapterDownload",
			query:         "mutation ReorderChapterDownload($input: ReorderChapterDownloadInput!) { reorderChapterDownload(input: $input) { __typename } }",
			variables: map[string]interface{}{
				"input": map[string]interface{}{
					"chapterId": float64(94972),
					"to":        float64(0),
				},
			},
			expectedKey: "reorderChapterDownload",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"operationName": tc.operationName,
				"query":         tc.query,
				"variables":     tc.variables,
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
				t.Fatalf("expected non-nil data in response, got %v", resMap)
			}

			if _, exists := data[tc.expectedKey]; !exists {
				t.Fatalf("expected key %q in data, got keys: %v", tc.expectedKey, data)
			}
		})
	}
}

func TestStartDownloader_EmptyQueueReturnsStopped(t *testing.T) {
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

	// Ensure queue is empty
	payload := map[string]interface{}{
		"query": `
			mutation {
				startDownloader(input: {clientMutationId: "test-client-id"}) {
					clientMutationId
					downloadStatus {
						state
						queue {
							state
							errorMessage
						}
					}
				}
			}
		`,
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

	data := resMap["data"].(map[string]interface{})
	startDownloader := data["startDownloader"].(map[string]interface{})
	if startDownloader["clientMutationId"] != "test-client-id" {
		t.Fatalf("expected clientMutationId 'test-client-id', got %v", startDownloader["clientMutationId"])
	}

	status := startDownloader["downloadStatus"].(map[string]interface{})
	if status["state"] != "STOPPED" {
		t.Fatalf("expected state STOPPED for empty queue, got %v", status["state"])
	}
	queue := status["queue"].([]interface{})
	if len(queue) != 0 {
		t.Fatalf("expected empty queue, got %d items", len(queue))
	}
}
