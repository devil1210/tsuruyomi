package extensions

import (
	"os"
	"path/filepath"
	"testing"

	"tsuruyomi/internal/sources"
)

func TestManifestUnmarshalAndRegistration(t *testing.T) {
	jsonContent := `{
		"name": "Test Madara Ext",
		"pkgName": "eu.kanade.tachiyomi.extension.es.testmadara",
		"lang": "es",
		"version": "1.0.0",
		"versionCode": 1,
		"icon": "/icon.png",
		"sources": [
			{
				"id": "999111222333",
				"name": "Test Madara",
				"lang": "es",
				"baseUrl": "https://testmadara.com",
				"engine": "madara",
				"contentWarning": 0
			},
			{
				"id": 999111222444,
				"name": "Test Themesia",
				"lang": "es",
				"baseUrl": "https://testthemesia.com",
				"engine": "mangathemesia",
				"contentWarning": 1
			}
		]
	}`

	tmpDir, err := os.MkdirTemp("", "ext-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manifestFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(manifestFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	m, err := LoadManifestFile(manifestFile)
	if err != nil {
		t.Fatalf("LoadManifestFile failed: %v", err)
	}

	if len(m.Sources) != 2 {
		t.Fatalf("Expected 2 sources, got %d", len(m.Sources))
	}

	if m.Sources[0].ID != 999111222333 {
		t.Fatalf("Expected source 0 ID 999111222333, got %d", m.Sources[0].ID)
	}
	if m.Sources[1].ID != 999111222444 {
		t.Fatalf("Expected source 1 ID 999111222444, got %d", m.Sources[1].ID)
	}

	// Test registration
	RegisterManifestSources(m)

	h0, ok0 := sources.Get(999111222333)
	if !ok0 || h0.Name() != "Test Madara" {
		t.Fatalf("Source 999111222333 not registered properly")
	}

	h1, ok1 := sources.Get(999111222444)
	if !ok1 || h1.Name() != "Test Themesia" {
		t.Fatalf("Source 999111222444 not registered properly")
	}

	// Test unregister
	UnregisterManifestSources(m)

	if _, ok := sources.Get(999111222333); ok {
		t.Fatalf("Source 999111222333 was not unregistered")
	}
	if _, ok := sources.Get(999111222444); ok {
		t.Fatalf("Source 999111222444 was not unregistered")
	}
}
