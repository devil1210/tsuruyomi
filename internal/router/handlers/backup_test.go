package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/service"
)

func TestGetBackupFileByToken(t *testing.T) {
	app := fiber.New()
	app.Get("/api/graphql/files/backup/:token", GetBackupFileByToken)

	// 1. Nonexistent token returns 404
	req404 := httptest.NewRequest("GET", "/api/graphql/files/backup/invalid-token.tachibk", nil)
	resp404, err := app.Test(req404)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp404.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp404.StatusCode)
	}

	// 2. Save a file into TemporaryFileStorage and retrieve via route
	content := []byte("dummy backup payload")
	filename := "test_export.tachibk"
	token, err := service.GlobalTempFileStorage.SaveFile(filename, content)
	if err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}

	req200 := httptest.NewRequest("GET", "/api/graphql/files/backup/"+token, nil)
	resp200, err := app.Test(req200)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp200.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp200.StatusCode)
	}

	disposition := resp200.Header.Get("Content-Disposition")
	if !strings.Contains(disposition, filename) {
		t.Fatalf("expected Content-Disposition to contain %q, got %q", filename, disposition)
	}
}

func TestHandleGraphQL_CreateBackupMutation(t *testing.T) {
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

	payload := map[string]interface{}{
		"query": `
			mutation {
				createBackup(input: {clientMutationId: "test-backup-id"}) {
					clientMutationId
					url
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
	cb := data["createBackup"].(map[string]interface{})
	if cb["clientMutationId"] != "test-backup-id" {
		t.Fatalf("expected clientMutationId 'test-backup-id', got %v", cb["clientMutationId"])
	}

	urlStr, ok := cb["url"].(string)
	if !ok || !strings.HasPrefix(urlStr, "/api/graphql/files/backup/") {
		t.Fatalf("expected url to start with /api/graphql/files/backup/, got %v", cb["url"])
	}
	if !strings.HasSuffix(urlStr, ".tachibk") {
		t.Fatalf("expected url to end with .tachibk, got %v", urlStr)
	}
}
