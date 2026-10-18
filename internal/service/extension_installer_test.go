package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchRepoIndex(t *testing.T) {
	mockIndexJSON := `[
		{
			"name": "ManhwaWeb",
			"pkg": "eu.kanade.tachiyomi.extension.es.manhwaweb",
			"apk": "manhwaweb.json",
			"lang": "es",
			"code": 1,
			"version": "1.0.0",
			"nsfw": 0,
			"icon": "https://example.com/icon.png",
			"sources": [
				{
					"id": "5943212552955476721",
					"name": "ManhwaWeb",
					"lang": "es",
					"baseUrl": "https://manhwa-web.com",
					"engine": "mangathemesia",
					"contentWarning": 0
				}
			]
		}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockIndexJSON))
	}))
	defer server.Close()

	ctx := context.Background()
	entries, err := FetchRepoIndex(ctx, server.URL+"/index.json")
	if err != nil {
		t.Fatalf("FetchRepoIndex failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Name != "ManhwaWeb" {
		t.Errorf("Expected Name 'ManhwaWeb', got %q", e.Name)
	}
	if e.Pkg != "eu.kanade.tachiyomi.extension.es.manhwaweb" {
		t.Errorf("Expected Pkg 'eu.kanade.tachiyomi.extension.es.manhwaweb', got %q", e.Pkg)
	}
	if len(e.Sources) != 1 {
		t.Fatalf("Expected 1 source, got %d", len(e.Sources))
	}
	if e.Sources[0].ID != 5943212552955476721 {
		t.Errorf("Expected source ID 5943212552955476721, got %d", e.Sources[0].ID)
	}
	if e.Sources[0].Engine != "mangathemesia" {
		t.Errorf("Expected engine 'mangathemesia', got %q", e.Sources[0].Engine)
	}
}
