package sources

import (
	"context"
	"testing"
	"time"
)

func TestSubmanhwaHandler_GetPopularManga(t *testing.T) {
	h := NewSubmanhwaHandler()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	items, hasNext, err := h.GetPopularManga(ctx, 1)
	if err != nil {
		t.Fatalf("GetPopularManga failed: %v", err)
	}

	if len(items) == 0 {
		t.Fatalf("expected at least 1 popular manga, got 0")
	}

	t.Logf("Fetched %d popular mangas (hasNext=%v): first=%q, URL=%q, thumb=%q", len(items), hasNext, items[0].Title, items[0].URL, items[0].ThumbnailURL)

	if items[0].Title == "" || items[0].URL == "" {
		t.Errorf("popular manga missing title or URL: %+v", items[0])
	}
}

func TestSubmanhwaHandler_GetLatestManga(t *testing.T) {
	h := NewSubmanhwaHandler()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	items, _, err := h.GetLatestManga(ctx, 1)
	if err != nil {
		t.Fatalf("GetLatestManga failed: %v", err)
	}

	if len(items) == 0 {
		t.Fatalf("expected at least 1 latest manga from homepage, got 0")
	}

	t.Logf("Fetched %d latest mangas: first=%q, URL=%q", len(items), items[0].Title, items[0].URL)
}

func TestSubmanhwaHandler_GetChapterListAndPages(t *testing.T) {
	h := NewSubmanhwaHandler()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	chapters, err := h.GetChapterList(ctx, "/serie/plan-de-intercambio-de-madres")
	if err != nil {
		t.Fatalf("GetChapterList failed: %v", err)
	}

	if len(chapters) == 0 {
		t.Fatalf("expected at least 1 chapter, got 0")
	}

	t.Logf("Fetched %d chapters: first=%q (order=%d), last=%q (order=%d)",
		len(chapters), chapters[0].Name, chapters[0].SourceOrder,
		chapters[len(chapters)-1].Name, chapters[len(chapters)-1].SourceOrder)

	if chapters[len(chapters)-1].SourceOrder != 0 {
		t.Fatalf("expected oldest chapter to have SourceOrder 0, got %d", chapters[len(chapters)-1].SourceOrder)
	}
	if chapters[0].SourceOrder != len(chapters)-1 {
		t.Fatalf("expected newest chapter to have SourceOrder %d, got %d", len(chapters)-1, chapters[0].SourceOrder)
	}

	pages, err := h.GetPageList(ctx, chapters[0].URL, "/serie/plan-de-intercambio-de-madres")
	if err != nil {
		t.Fatalf("GetPageList failed: %v", err)
	}

	if len(pages) == 0 {
		t.Fatalf("expected at least 1 page image, got 0")
	}

	t.Logf("Fetched %d pages for chapter %s: first=%q", len(pages), chapters[0].Name, pages[0])
}
