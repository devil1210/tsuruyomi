package service

import (
	"context"
	"testing"

	"tsuruyomi/internal/config"
)

func TestReconcileDownloads_NilDatabase(t *testing.T) {
	cfg := &config.Config{}
	cfg.Data.RootDir = t.TempDir()

	// When DB is nil (e.g. uninitialized in standard unit tests)
	_, err := ReconcileDownloads(context.Background(), cfg)
	if err == nil {
		t.Fatalf("expected error when database is nil, got nil")
	}
}
