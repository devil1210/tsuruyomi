package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"tsuruyomi/internal/config"
)

type flareSolverrRequest struct {
	Cmd  string `json:"cmd"`
	URL  string `json:"url"`
	UUID string `json:"uuid,omitempty"`
}

type flareSolverrCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

type flareSolverrSolution struct {
	URL       string               `json:"url"`
	Status    int                  `json:"status"`
	Cookies   []flareSolverrCookie `json:"cookies"`
	UserAgent string               `json:"userAgent"`
	Response  string               `json:"response"`
}

type flareSolverrResponse struct {
	Status   string               `json:"status"`
	Message  string               `json:"message"`
	Solution flareSolverrSolution `json:"solution"`
}

type domainSession struct {
	Cookies   []flareSolverrCookie
	UserAgent string
	ExpiresAt time.Time
}

type challengeResult struct {
	Cookies   []flareSolverrCookie
	UserAgent string
}

const defaultSessionTTL = 1 * time.Hour

var (
	sessionCache    = make(map[string]*domainSession)
	sessionCacheMu  sync.RWMutex
	sfGroup         singleflight.Group
	flareSolverrSem = make(chan struct{}, 1)
)

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(u.Hostname()))
}

// ExtractDomain returns the lowercase hostname from a raw URL.
func ExtractDomain(rawURL string) string {
	return extractDomain(rawURL)
}

func getSession(domain string) ([]flareSolverrCookie, string, bool) {
	if domain == "" {
		return nil, "", false
	}
	sessionCacheMu.RLock()
	sess, ok := sessionCache[domain]
	sessionCacheMu.RUnlock()

	if !ok {
		return nil, "", false
	}

	if time.Now().After(sess.ExpiresAt) {
		invalidateSession(domain)
		return nil, "", false
	}

	cookies := make([]flareSolverrCookie, len(sess.Cookies))
	copy(cookies, sess.Cookies)
	return cookies, sess.UserAgent, true
}

func setSession(domain string, cookies []flareSolverrCookie, userAgent string, ttl time.Duration) {
	if domain == "" {
		return
	}
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	copied := make([]flareSolverrCookie, len(cookies))
	copy(copied, cookies)

	sessionCacheMu.Lock()
	defer sessionCacheMu.Unlock()
	sessionCache[domain] = &domainSession{
		Cookies:   copied,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func invalidateSession(domain string) {
	if domain == "" {
		return
	}
	sessionCacheMu.Lock()
	defer sessionCacheMu.Unlock()
	delete(sessionCache, domain)
}

// GetSession returns a cached FlareSolverr session for domain, if present and not expired.
func GetSession(domain string) ([]flareSolverrCookie, string, bool) {
	return getSession(domain)
}

// SetSession stores a FlareSolverr session for domain.
func SetSession(domain string, cookies []flareSolverrCookie, userAgent string, ttl time.Duration) {
	setSession(domain, cookies, userAgent, ttl)
}

// InvalidateSession removes a domain's cached session.
func InvalidateSession(domain string) {
	invalidateSession(domain)
}

// HasSession returns true if domain has a valid non-expired cached session.
func HasSession(domain string) bool {
	_, _, ok := getSession(domain)
	return ok
}

func doDownloadWithCookies(ctx context.Context, targetURL, referer string, cookies []flareSolverrCookie, userAgent string) ([]byte, string, error) {
	client := &http.Client{
		Timeout: 35 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, "", err
	}

	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	} else {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	for _, c := range cookies {
		if c.Name == "" {
			continue
		}
		req.AddCookie(&http.Cookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
			Path:   c.Path,
		})
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = DetectImageType(data)
	}

	return data, contentType, nil
}

func doDirectDownload(ctx context.Context, targetURL, referer string) ([]byte, string, int, error) {
	client := &http.Client{
		Timeout: 35 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, "", 0, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("initial request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", resp.StatusCode, err
		}
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = DetectImageType(data)
		}
		return data, contentType, resp.StatusCode, nil
	}

	return nil, "", resp.StatusCode, fmt.Errorf("request returned status: %d", resp.StatusCode)
}

// DownloadWithFlareSolverr performs an HTTP GET request to retrieve a remote resource.
func DownloadWithFlareSolverr(ctx context.Context, cfg *config.Config, targetURL string) ([]byte, string, error) {
	return DownloadWithFlareSolverrAndReferer(ctx, cfg, targetURL, "")
}

// DownloadWithFlareSolverrAndReferer performs an HTTP GET request with an optional Referer header.
// It uses cached domain sessions, deduplicates concurrent solves via singleflight, and limits
// FlareSolverr concurrency via a global semaphore.
func DownloadWithFlareSolverrAndReferer(ctx context.Context, cfg *config.Config, targetURL string, referer string) ([]byte, string, error) {
	domain := extractDomain(targetURL)
	attemptedSession := false

	// Paso A: Si existe una sesión válida en caché para ese dominio
	if domain != "" {
		if cookies, ua, ok := getSession(domain); ok && len(cookies) > 0 {
			attemptedSession = true
			log.Printf("[FLARESOLVERR] Using cached session for %s", domain)
			data, mimeType, err := doDownloadWithCookies(ctx, targetURL, referer, cookies, ua)
			if err == nil {
				return data, mimeType, nil
			}
			log.Printf("[FLARESOLVERR] Cached session for %s failed (%v), invalidating...", domain, err)
			invalidateSession(domain)
		}
	}

	// Paso B: Si no había sesión en caché (o la sesión previa falló):
	if !attemptedSession {
		data, mimeType, status, err := doDirectDownload(ctx, targetURL, referer)
		if err == nil {
			return data, mimeType, nil
		}
		// If not a Cloudflare challenge status or FlareSolverr is disabled, return original error
		if (status != http.StatusForbidden && status != http.StatusServiceUnavailable) || cfg == nil || !cfg.FlareSolverr.Enabled {
			return nil, "", err
		}
	} else if cfg == nil || !cfg.FlareSolverr.Enabled {
		return nil, "", fmt.Errorf("Cloudflare challenge encountered for %s but FlareSolverr is disabled", domain)
	}

	// Resolve Cloudflare challenge via singleflight deduplication
	sfKey := domain
	if sfKey == "" {
		sfKey = targetURL
	}

	v, err, _ := sfGroup.Do(sfKey, func() (any, error) {
		// Double-check session in case another concurrent goroutine just finished solving it
		if sfKey != "" {
			if cookies, ua, ok := getSession(sfKey); ok && len(cookies) > 0 {
				return &challengeResult{Cookies: cookies, UserAgent: ua}, nil
			}
		}

		log.Printf("[FLARESOLVERR] Solving challenge for %s (%s)", sfKey, targetURL)
		cookies, ua, solveErr := solveCloudflareChallenge(ctx, cfg, targetURL)
		if solveErr != nil {
			return nil, solveErr
		}

		if sfKey != "" {
			setSession(sfKey, cookies, ua, defaultSessionTTL)
			log.Printf("[FLARESOLVERR] Solved and cached session for %s (%d cookies)", sfKey, len(cookies))
		}

		return &challengeResult{Cookies: cookies, UserAgent: ua}, nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to solve challenge for %s: %w", sfKey, err)
	}

	res, ok := v.(*challengeResult)
	if !ok || len(res.Cookies) == 0 {
		return nil, "", fmt.Errorf("no cookies returned from FlareSolverr for %s", sfKey)
	}

	// Retry download with resolved cookies and User-Agent
	retryData, retryMime, retryErr := doDownloadWithCookies(ctx, targetURL, referer, res.Cookies, res.UserAgent)
	if retryErr != nil {
		return nil, "", fmt.Errorf("retry after solving challenge failed: %w", retryErr)
	}

	return retryData, retryMime, nil
}

// NormalizeFlareSolverrURL ensures the URL has http:// or https:// and strips trailing /v1.
func NormalizeFlareSolverrURL(rawURL string) string {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return ""
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "http://" + u
	}
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, "/v1")
	return u
}

// GetFlareSolverrAPIURL returns the endpoint URL ending in /v1.
func GetFlareSolverrAPIURL(rawURL string) string {
	u := NormalizeFlareSolverrURL(rawURL)
	if u == "" {
		return ""
	}
	return u + "/v1"
}

func getFlareSolverrAPIURL(rawURL string) string {
	return GetFlareSolverrAPIURL(rawURL)
}

func getFlareSolverrTimeout(t int) time.Duration {
	if t <= 0 {
		return 60 * time.Second
	}
	return time.Duration(t) * time.Second
}

func solveCloudflareChallenge(ctx context.Context, cfg *config.Config, targetURL string) ([]flareSolverrCookie, string, error) {
	if cfg == nil || !cfg.FlareSolverr.Enabled {
		return nil, "", fmt.Errorf("FlareSolverr is not enabled")
	}

	select {
	case flareSolverrSem <- struct{}{}:
		defer func() { <-flareSolverrSem }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}

	fsClient := &http.Client{
		Timeout: getFlareSolverrTimeout(cfg.FlareSolverr.Timeout),
	}

	apiURL := getFlareSolverrAPIURL(cfg.FlareSolverr.URL)
	payload := flareSolverrRequest{
		Cmd: "request.get",
		URL: targetURL,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := fsClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("calling FlareSolverr failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("FlareSolverr returned status: %d", resp.StatusCode)
	}

	var result flareSolverrResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", err
	}

	if result.Status != "ok" {
		return nil, "", fmt.Errorf("FlareSolverr error: %s", result.Message)
	}

	return result.Solution.Cookies, result.Solution.UserAgent, nil
}

func fetchHTMLWithSession(ctx context.Context, targetURL, referer string, cookies []flareSolverrCookie, userAgent string) (string, error) {
	client := &http.Client{
		Timeout: 35 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", err
	}

	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	} else {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "es-419,es;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	for _, c := range cookies {
		if c.Name == "" {
			continue
		}
		req.AddCookie(&http.Cookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
			Path:   c.Path,
		})
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	strBody := string(body)
	if strings.Contains(strBody, "cf-browser-verification") || strings.Contains(strBody, "challenge-running") {
		return "", fmt.Errorf("cloudflare challenge present in response")
	}

	return strBody, nil
}

// SolveAndFetchHTML queries FlareSolverr / Byparr to bypass Cloudflare and return the page HTML directly.
func SolveAndFetchHTML(ctx context.Context, cfg *config.Config, targetURL string, referer string) (string, error) {
	if cfg == nil || !cfg.FlareSolverr.Enabled {
		return "", fmt.Errorf("FlareSolverr is not enabled")
	}

	domain := extractDomain(targetURL)
	if domain != "" {
		if cookies, ua, ok := getSession(domain); ok && len(cookies) > 0 {
			log.Printf("[FLARESOLVERR] Using cached session for %s HTML", domain)
			html, err := fetchHTMLWithSession(ctx, targetURL, referer, cookies, ua)
			if err == nil && len(html) > 0 {
				return html, nil
			}
			log.Printf("[FLARESOLVERR] Cached session failed for %s HTML (%v), invalidating...", domain, err)
			invalidateSession(domain)
		}
	}

	select {
	case flareSolverrSem <- struct{}{}:
		defer func() { <-flareSolverrSem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	fsClient := &http.Client{
		Timeout: getFlareSolverrTimeout(cfg.FlareSolverr.Timeout),
	}

	apiURL := getFlareSolverrAPIURL(cfg.FlareSolverr.URL)
	payload := flareSolverrRequest{
		Cmd: "request.get",
		URL: targetURL,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := fsClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling FlareSolverr failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("FlareSolverr returned status: %d", resp.StatusCode)
	}

	var result flareSolverrResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding FlareSolverr response: %w", err)
	}

	if result.Status != "ok" {
		return "", fmt.Errorf("FlareSolverr error: %s", result.Message)
	}

	// Cache session from successful solution
	if domain != "" && len(result.Solution.Cookies) > 0 {
		setSession(domain, result.Solution.Cookies, result.Solution.UserAgent, defaultSessionTTL)
		log.Printf("[FLARESOLVERR] Cached session for %s from SolveAndFetchHTML (%d cookies)", domain, len(result.Solution.Cookies))
	}

	if result.Solution.Response != "" {
		return result.Solution.Response, nil
	}

	// Fallback: If response HTML was omitted, retry with resolved cookies and User-Agent
	if len(result.Solution.Cookies) > 0 {
		html, err := fetchHTMLWithSession(ctx, targetURL, referer, result.Solution.Cookies, result.Solution.UserAgent)
		if err == nil && len(html) > 0 {
			return html, nil
		}
	}

	return "", fmt.Errorf("no HTML content returned by FlareSolverr")
}

// ExtractJSONFromHTML extracts raw JSON from an HTML response (e.g. if Chrome wrapped JSON in <pre> tags).
func ExtractJSONFromHTML(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return trimmed
	}
	if preIdx := strings.Index(trimmed, "<pre"); preIdx != -1 {
		closeIdx := strings.Index(trimmed[preIdx:], ">")
		if closeIdx != -1 {
			sub := trimmed[preIdx+closeIdx+1:]
			if endIdx := strings.Index(sub, "</pre>"); endIdx != -1 {
				inner := strings.TrimSpace(sub[:endIdx])
				if strings.HasPrefix(inner, "{") || strings.HasPrefix(inner, "[") {
					return inner
				}
			}
		}
	}
	return trimmed
}
