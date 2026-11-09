package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseSpanishOrEnglishDate(t *testing.T) {
	tests := []struct {
		input    string
		expected string // YYYY-MM-DD
	}{
		{"03 Nov 2026", "2026-11-03"},
		{"27 Oct 2026", "2026-10-27"},
		{"14 Oct 2026", "2026-10-14"},
		{"20 Jun 2026", "2026-06-20"},
		{"15 Ago 2026", "2026-08-15"},
		{"10 Dic 2026", "2026-12-10"},
		{"05 Ene 2026", "2026-01-05"},
		{"12 Feb 2026", "2026-02-12"},
		{"18 Mar 2026", "2026-03-18"},
		{"22 Abr 2026", "2026-04-22"},
		{"01 May 2026", "2026-05-01"},
		{"08 Jul 2026", "2026-07-08"},
		{"30 Sep 2026", "2026-09-30"},
		{"30 Set 2026", "2026-09-30"},
		{"14 de octubre de 2026", "2026-10-14"},
		{"November 3, 2026", "2026-11-03"},
	}

	for _, tc := range tests {
		ts := ParseSpanishOrEnglishDate(tc.input)
		if ts == 0 {
			t.Errorf("ParseSpanishOrEnglishDate(%q) returned 0, expected %s", tc.input, tc.expected)
			continue
		}
		parsed := time.UnixMilli(ts).UTC().Format("2006-01-02")
		if parsed != tc.expected {
			t.Errorf("ParseSpanishOrEnglishDate(%q) = %s, expected %s", tc.input, parsed, tc.expected)
		}
	}
}

func TestLeerManhwasChapterParsing(t *testing.T) {
	mockHTML := `
	<html>
	<body>
		<ul class="chapter-list">
			<li class="myfut">
				<a class="leermos" href="/manhwa/test-manga/capitulo-35/" title="Test Manga - Capítulo 35">
					<span class="chapter-name">Capítulo 35</span>
					<span class="ct-update">03 Nov 2026</span>
				</a>
			</li>
			<li class="myfut">
				<a class="leermos" href="/manhwa/test-manga/capitulo-36/" title="Test Manga - Capítulo 36">
					<span class="chapter-name">Capítulo 36</span>
					<span class="ct-update">
						<img src="/images/new.gif" alt="NEW"/>
					</span>
				</a>
			</li>
		</ul>
	</body>
	</html>
	`

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(mockHTML))
	}))
	defer mockServer.Close()

	handler := NewMadaraHandlerWithPath("LeerManhwas", mockServer.URL, "leer-manhwa-18")
	chapters, err := handler.GetChapterList(context.Background(), "/manhwa/test-manga")
	if err != nil {
		t.Fatalf("GetChapterList failed: %v", err)
	}

	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}

	// First chapter in HTML is Capítulo 35 with "03 Nov 2026"
	ch35 := chapters[0]
	if ch35.Name != "Capítulo 35" {
		t.Errorf("expected name 'Capítulo 35', got %q", ch35.Name)
	}
	if ch35.DateUpload == 0 {
		t.Errorf("ch35 DateUpload should not be 0")
	}
	parsedDate := time.UnixMilli(ch35.DateUpload).UTC().Format("2006-01-02")
	if parsedDate != "2026-11-03" {
		t.Errorf("expected parsedDate '2026-11-03', got %q", parsedDate)
	}

	// Second chapter in HTML is Capítulo 36 with "NEW" image
	ch36 := chapters[1]
	if ch36.Name != "Capítulo 36" {
		t.Errorf("expected name 'Capítulo 36', got %q", ch36.Name)
	}
	if ch36.DateUpload == 0 {
		t.Errorf("ch36 DateUpload should not be 0 for NEW chapter")
	}
	// Difference between now and ch36.DateUpload should be less than 5 seconds
	if time.Since(time.UnixMilli(ch36.DateUpload)) > 5*time.Second {
		t.Errorf("ch36 DateUpload was not assigned current time: %v", ch36.DateUpload)
	}
}
