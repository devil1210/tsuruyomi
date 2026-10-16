package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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

// DownloadWithFlareSolverr performs an HTTP GET request to retrieve a remote resource.
func DownloadWithFlareSolverr(ctx context.Context, cfg *config.Config, targetURL string) ([]byte, string, error) {
	return DownloadWithFlareSolverrAndReferer(ctx, cfg, targetURL, "")
}

// DownloadWithFlareSolverrAndReferer performs an HTTP GET request with an optional Referer header.
// If it encounters a 403/503 (Cloudflare protection) and FlareSolverr is enabled in config,
// it queries the FlareSolverr service to bypass the challenge and retries the request using the resolved cookies/UA.
func DownloadWithFlareSolverrAndReferer(ctx context.Context, cfg *config.Config, targetURL string, referer string) ([]byte, string, error) {
	client := &http.Client{
		Timeout: 35 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, "", err
	}

	// Standard clean browser header
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("initial request failed: %w", err)
	}
	defer resp.Body.Close()

	// If HTTP 403 or 503 is returned, it might be a Cloudflare check
	if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable) && cfg.FlareSolverr.Enabled {
		// Attempt bypass via FlareSolverr
		cookies, userAgent, solveErr := solveCloudflareChallenge(ctx, cfg, targetURL)
		if solveErr == nil && len(cookies) > 0 {
			// Retry request with solved session
			retryReq, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
			if err != nil {
				return nil, "", err
			}

			retryReq.Header.Set("User-Agent", userAgent)
			if referer != "" {
				retryReq.Header.Set("Referer", referer)
			}
			for _, cookie := range cookies {
				retryReq.AddCookie(&http.Cookie{
					Name:   cookie.Name,
					Value:  cookie.Value,
					Domain: cookie.Domain,
					Path:   cookie.Path,
				})
			}

			retryResp, err := client.Do(retryReq)
			if err == nil {
				defer retryResp.Body.Close()
				if retryResp.StatusCode == http.StatusOK {
					data, err := io.ReadAll(retryResp.Body)
					if err != nil {
						return nil, "", err
					}
					return data, retryResp.Header.Get("Content-Type"), nil
				}
				return nil, "", fmt.Errorf("retry failed with status: %d", retryResp.StatusCode)
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("request returned status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return data, resp.Header.Get("Content-Type"), nil
}

func solveCloudflareChallenge(ctx context.Context, cfg *config.Config, targetURL string) ([]flareSolverrCookie, string, error) {
	fsClient := &http.Client{
		Timeout: time.Duration(cfg.FlareSolverr.Timeout) * time.Second,
	}

	apiURL := strings.TrimSuffix(cfg.FlareSolverr.URL, "/") + "/v1"
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

// SolveAndFetchHTML queries FlareSolverr / Byparr to bypass Cloudflare and return the page HTML directly.
func SolveAndFetchHTML(ctx context.Context, cfg *config.Config, targetURL string, referer string) (string, error) {
	if cfg == nil || !cfg.FlareSolverr.Enabled {
		return "", fmt.Errorf("FlareSolverr is not enabled")
	}

	fsClient := &http.Client{
		Timeout: time.Duration(cfg.FlareSolverr.Timeout) * time.Second,
	}

	apiURL := strings.TrimSuffix(cfg.FlareSolverr.URL, "/") + "/v1"
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

	if result.Solution.Response != "" {
		return result.Solution.Response, nil
	}

	// Fallback: If response HTML was omitted, retry with resolved cookies and User-Agent
	if len(result.Solution.Cookies) > 0 {
		retryReq, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if err != nil {
			return "", err
		}
		if result.Solution.UserAgent != "" {
			retryReq.Header.Set("User-Agent", result.Solution.UserAgent)
		}
		if referer != "" {
			retryReq.Header.Set("Referer", referer)
		}
		for _, cookie := range result.Solution.Cookies {
			retryReq.AddCookie(&http.Cookie{
				Name:   cookie.Name,
				Value:  cookie.Value,
				Domain: cookie.Domain,
				Path:   cookie.Path,
			})
		}
		retryResp, err := fsClient.Do(retryReq)
		if err == nil {
			defer retryResp.Body.Close()
			if retryResp.StatusCode == http.StatusOK {
				body, err := io.ReadAll(retryResp.Body)
				if err == nil {
					return string(body), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no HTML content returned by FlareSolverr")
}

