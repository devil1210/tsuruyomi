package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLectorXDParsing(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/catalogo":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`
				<html>
				<body>
					<article>
						<a href="/manhwa/solo-leveling">
							<img src="https://s1.cdnlxd.xyz/covers/solo.webp" alt="Solo Leveling" />
						</a>
					</article>
				</body>
				</html>
			`))

		case r.URL.Path == "/api/search":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"results": [
					{
						"id": 6910,
						"title": "Solo Leveling",
						"slug": "comic-solo-leveling",
						"coverImage": "https://s1.cdnlxd.xyz/covers/solo.webp",
						"type": "manhwa"
					}
				]
			}`))

		case r.URL.Path == "/manhwa/comic-solo-leveling":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`
				<html>
				<head>
					<script>
					const mangaSlug = "comic-solo-leveling";
					const mangaType = "manhwa";
					const chaptersList = [{"chapter":"1"},{"chapter":"2"}];
					</script>
				</head>
				<body></body>
				</html>
			`))

		case r.URL.Path == "/manhwa/comic-solo-leveling/leer/2":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`
				<html>
				<body>
					<div id="reader-app">
						<img class="page-image" data-src="https://s1.cdnlxd.xyz/6910/2/1.webp" />
						<img class="page-image" data-src="https://s1.cdnlxd.xyz/6910/2/2.webp" />
					</div>
				</body>
				</html>
			`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	h := &LectorXDHandler{
		name:    "LectorXD",
		baseURL: mockServer.URL,
	}

	ctx := context.Background()

	// 1. Test Popular
	items, _, err := h.GetPopularManga(ctx, 1)
	if err != nil {
		t.Fatalf("GetPopularManga failed: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Solo Leveling" {
		t.Errorf("Unexpected popular items: %+v", items)
	}

	// 2. Test Search
	searchItems, _, err := h.SearchManga(ctx, "solo", 1)
	if err != nil {
		t.Fatalf("SearchManga failed: %v", err)
	}
	if len(searchItems) != 1 || searchItems[0].URL != "/manhwa/comic-solo-leveling" {
		t.Errorf("Unexpected search items: %+v", searchItems)
	}

	// 3. Test Chapters
	chapters, err := h.GetChapterList(ctx, "/manhwa/comic-solo-leveling")
	if err != nil {
		t.Fatalf("GetChapterList failed: %v", err)
	}
	if len(chapters) != 2 || chapters[0].ChapterNumber != 2.0 {
		t.Errorf("Unexpected chapters: %+v", chapters)
	}

	// 4. Test Pages
	pages, err := h.GetPageList(ctx, "/manhwa/comic-solo-leveling/leer/2", "/manhwa/comic-solo-leveling")
	if err != nil {
		t.Fatalf("GetPageList failed: %v", err)
	}
	if len(pages) != 2 || pages[0] != "https://s1.cdnlxd.xyz/6910/2/1.webp" {
		t.Errorf("Unexpected pages: %+v", pages)
	}
}
