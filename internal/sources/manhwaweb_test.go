package sources

import (
	"context"
	"testing"
)

func TestManhwaWebHandler(t *testing.T) {
	h := NewManhwaWebHandler()

	ctx := context.Background()

	// 1. Test GetPageList with a known chapter slug
	pages, err := h.GetPageList(ctx, "/leer/mi-esposa-sigue-esperando-en-los-campos-de-trigo_1783885707780-1_01", "")
	if err != nil {
		t.Fatalf("GetPageList failed: %v", err)
	}
	if len(pages) == 0 {
		t.Fatalf("Expected pages > 0, got 0")
	}
	t.Logf("Got %d pages for chapter 1", len(pages))

	// 2. Test GetChapterList with known manga slug
	chapters, err := h.GetChapterList(ctx, "manhwa/mi-esposa-sigue-esperando-en-los-campos-de-trigo_1783885707780")
	if err != nil {
		t.Fatalf("GetChapterList failed: %v", err)
	}
	if len(chapters) == 0 {
		t.Fatalf("Expected chapters > 0, got 0")
	}
	t.Logf("Got %d chapters for manga", len(chapters))

	// 3. Test SearchManga
	items, next, err := h.SearchManga(ctx, "esposa", 1)
	if err != nil {
		t.Fatalf("SearchManga failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("Expected search items > 0, got 0")
	}
	t.Logf("Search returned %d items (next: %v)", len(items), next)
}
