package util

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tsuruyomi/internal/config"
)

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://mangasnosekai.com/manga/cover.jpg", "mangasnosekai.com"},
		{"http://SUB.DOMAIN.COM:8080/path/test?q=1", "sub.domain.com"},
		{"https://bokugents.com", "bokugents.com"},
		{"invalid-url", ""},
		{"", ""},
	}

	for _, tt := range tests {
		actual := ExtractDomain(tt.input)
		if actual != tt.expected {
			t.Errorf("ExtractDomain(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestDomainSessionCache(t *testing.T) {
	domain := "testdomain.com"
	invalidateSession(domain)

	// Initially no session
	if HasSession(domain) {
		t.Fatalf("expected no session for %s", domain)
	}

	cookies := []flareSolverrCookie{
		{Name: "cf_clearance", Value: "token123", Domain: ".testdomain.com", Path: "/"},
	}
	userAgent := "CustomUA/1.0"

	// Set session with 100ms TTL
	setSession(domain, cookies, userAgent, 100*time.Millisecond)

	if !HasSession(domain) {
		t.Fatalf("expected active session for %s", domain)
	}

	gotCookies, gotUA, ok := getSession(domain)
	if !ok || len(gotCookies) != 1 || gotCookies[0].Value != "token123" || gotUA != userAgent {
		t.Fatalf("unexpected session data: %+v, ua: %s, ok: %v", gotCookies, gotUA, ok)
	}

	// Verify expiration
	time.Sleep(120 * time.Millisecond)
	if HasSession(domain) {
		t.Fatalf("session should have expired for %s", domain)
	}

	// Test InvalidateSession
	setSession(domain, cookies, userAgent, 1*time.Hour)
	if !HasSession(domain) {
		t.Fatalf("expected active session")
	}
	invalidateSession(domain)
	if HasSession(domain) {
		t.Fatalf("session should be invalidated")
	}
}

func TestDomainSessionConcurrentRace(t *testing.T) {
	domain := "concurrent.com"
	cookies := []flareSolverrCookie{
		{Name: "cf_clearance", Value: "race_test", Domain: domain, Path: "/"},
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			setSession(domain, cookies, fmt.Sprintf("UA-%d", idx), 1*time.Hour)
		}(i)

		go func() {
			defer wg.Done()
			_, _, _ = getSession(domain)
			_ = HasSession(domain)
		}()
	}
	wg.Wait()
	invalidateSession(domain)
}

func TestDownloadWithFlareSolverrAndReferer_SingleflightAndCache(t *testing.T) {
	var flareSolverrCalls int64
	var targetServerDirectCalls int64
	var targetServerBypassCalls int64

	// Mock FlareSolverr Server
	fsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&flareSolverrCalls, 1)
		// Simulate small processing latency
		time.Sleep(50 * time.Millisecond)

		resp := flareSolverrResponse{
			Status:  "ok",
			Message: "Challenge solved",
			Solution: flareSolverrSolution{
				Status: 200,
				Cookies: []flareSolverrCookie{
					{Name: "cf_clearance", Value: "valid_clearance_secret", Domain: "127.0.0.1", Path: "/"},
				},
				UserAgent: "FlareSolverr-Solved-Browser/1.0",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer fsServer.Close()

	// Mock Protected Target Server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for valid cf_clearance cookie and User-Agent
		cookie, err := r.Cookie("cf_clearance")
		if err == nil && cookie.Value == "valid_clearance_secret" && r.Header.Get("User-Agent") == "FlareSolverr-Solved-Browser/1.0" {
			atomic.AddInt64(&targetServerBypassCalls, 1)
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) // PNG magic bytes
			return
		}

		// Cloudflare challenge triggered
		atomic.AddInt64(&targetServerDirectCalls, 1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html>Just a moment... Cloudflare challenge</html>"))
	}))
	defer targetServer.Close()

	cfg := &config.Config{
		FlareSolverr: config.FlareSolverrConfig{
			Enabled: true,
			URL:     fsServer.URL,
			Timeout: 10,
		},
	}

	targetDomain := ExtractDomain(targetServer.URL)
	invalidateSession(targetDomain)

	// Step 1: Fire 10 concurrent requests to simulate image loading storm
	var wg sync.WaitGroup
	concurrentCount := 10
	errChan := make(chan error, concurrentCount)

	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			imageURL := fmt.Sprintf("%s/page_%d.png", targetServer.URL, idx)
			data, mime, err := DownloadWithFlareSolverrAndReferer(context.Background(), cfg, imageURL, targetServer.URL)
			if err != nil {
				errChan <- err
				return
			}
			if mime != "image/png" || len(data) == 0 {
				errChan <- fmt.Errorf("unexpected mime or empty data: %s, len: %d", mime, len(data))
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Fatalf("concurrent download failed: %v", err)
	}

	// Verify that singleflight deduplicated the 10 concurrent requests to exactly 1 FlareSolverr call!
	fsCalls := atomic.LoadInt64(&flareSolverrCalls)
	if fsCalls != 1 {
		t.Errorf("expected exactly 1 FlareSolverr call due to singleflight, got %d", fsCalls)
	}

	// Step 2: Now that session is cached, a subsequent request to a new URL on the same domain
	// should succeed immediately using the cached session without touching FlareSolverr again!
	nextURL := fmt.Sprintf("%s/page_next.png", targetServer.URL)
	data, mime, err := DownloadWithFlareSolverrAndReferer(context.Background(), cfg, nextURL, targetServer.URL)
	if err != nil {
		t.Fatalf("subsequent cached request failed: %v", err)
	}
	if mime != "image/png" || len(data) == 0 {
		t.Fatalf("unexpected data on subsequent request")
	}

	// FlareSolverr calls should STILL be 1!
	if calls := atomic.LoadInt64(&flareSolverrCalls); calls != 1 {
		t.Errorf("FlareSolverr calls increased to %d, session cache was not reused!", calls)
	}
}

func TestSolveAndFetchHTML_SemaphoreAndCache(t *testing.T) {
	var fsCalls int64

	// Mock FlareSolverr Server
	fsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&fsCalls, 1)
		time.Sleep(30 * time.Millisecond)

		resp := flareSolverrResponse{
			Status:  "ok",
			Message: "Challenge solved",
			Solution: flareSolverrSolution{
				Status:   200,
				Response: "<html><body><h1>Manga Chapter 1</h1></body></html>",
				Cookies: []flareSolverrCookie{
					{Name: "cf_clearance", Value: "html_token", Domain: "127.0.0.1", Path: "/"},
				},
				UserAgent: "HTML-Solver-UA/1.0",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer fsServer.Close()

	// Target Server that requires cookie
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("cf_clearance")
		if err == nil && cookie.Value == "html_token" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body><h1>Direct Cached HTML</h1></body></html>"))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html>Forbidden</html>"))
	}))
	defer targetServer.Close()

	cfg := &config.Config{
		FlareSolverr: config.FlareSolverrConfig{
			Enabled: true,
			URL:     fsServer.URL,
			Timeout: 10,
		},
	}

	targetDomain := ExtractDomain(targetServer.URL)
	invalidateSession(targetDomain)

	// Call 1: Should call FlareSolverr and cache the session
	html, err := SolveAndFetchHTML(context.Background(), cfg, targetServer.URL, "")
	if err != nil {
		t.Fatalf("SolveAndFetchHTML failed: %v", err)
	}
	if html != "<html><body><h1>Manga Chapter 1</h1></body></html>" {
		t.Fatalf("unexpected HTML: %s", html)
	}
	if atomic.LoadInt64(&fsCalls) != 1 {
		t.Fatalf("expected 1 FlareSolverr call, got %d", atomic.LoadInt64(&fsCalls))
	}

	// Verify session was cached
	if !HasSession(targetDomain) {
		t.Fatalf("session should be cached for %s", targetDomain)
	}

	// Call 2: Should use cached session and fetch directly from targetServer without FlareSolverr!
	html2, err := SolveAndFetchHTML(context.Background(), cfg, targetServer.URL, "")
	if err != nil {
		t.Fatalf("second SolveAndFetchHTML failed: %v", err)
	}
	if html2 != "<html><body><h1>Direct Cached HTML</h1></body></html>" {
		t.Fatalf("unexpected second HTML: %s", html2)
	}

	// FlareSolverr calls should STILL be 1!
	if calls := atomic.LoadInt64(&fsCalls); calls != 1 {
		t.Errorf("FlareSolverr calls increased to %d, cache was not reused!", calls)
	}
}

