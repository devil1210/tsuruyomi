package router

import (
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"tsuruyomi/internal/auth"
	"tsuruyomi/internal/config"
)

func TestAuthMiddlewareIntegration(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Subpath: "",
		},
		Data: config.DataConfig{
			RootDir: "./data",
		},
		Auth: config.AuthConfig{
			Mode:     "basic",
			Username: "reader",
			Password: "mangaPassword123",
		},
	}

	app := Setup(cfg)

	// 1. Unauthenticated request to protected API endpoint should return 401
	req := httptest.NewRequest("GET", "/api/v1/meta", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for /api/v1/meta without auth, got %d", resp.StatusCode)
	}
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if !strings.Contains(wwwAuth, "Basic realm=") {
		t.Errorf("Expected WWW-Authenticate header, got: %s", wwwAuth)
	}

	// 2. Unauthenticated request to /api/graphql MUST return 401
	reqGQL := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(`{"query":"{ __typename }"}`))
	reqGQL.Header.Set("Content-Type", "application/json")
	respGQL, err := app.Test(reqGQL)
	if err != nil {
		t.Fatalf("Failed to execute GQL request: %v", err)
	}
	if respGQL.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("CRITICAL: Expected 401 Unauthorized for /api/graphql without auth, got %d", respGQL.StatusCode)
	}

	// 3. Public asset (/favicon.svg) should NOT return 401
	reqFav := httptest.NewRequest("GET", "/favicon.svg", nil)
	respFav, err := app.Test(reqFav)
	if err != nil {
		t.Fatalf("Failed to execute favicon request: %v", err)
	}
	if respFav.StatusCode == fiber.StatusUnauthorized {
		t.Errorf("Expected /favicon.svg to not be blocked by 401")
	}

	// 4. Login page /login.html should return 200 OK
	reqLogin := httptest.NewRequest("GET", "/login.html", nil)
	respLogin, err := app.Test(reqLogin)
	if err != nil {
		t.Fatalf("Failed to execute login page request: %v", err)
	}
	if respLogin.StatusCode != fiber.StatusOK {
		t.Errorf("Expected 200 OK for /login.html, got %d", respLogin.StatusCode)
	}

	// 5. Valid HTTP Basic Auth on /api/v1/meta should succeed
	reqAuth := httptest.NewRequest("GET", "/api/v1/meta", nil)
	cred := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cfg.Auth.Username, cfg.Auth.Password)))
	reqAuth.Header.Set("Authorization", "Basic "+cred)
	respAuth, err := app.Test(reqAuth)
	if err != nil {
		t.Fatalf("Failed to execute basic auth request: %v", err)
	}
	if respAuth.StatusCode == fiber.StatusUnauthorized {
		t.Errorf("Expected valid basic auth to succeed, got 401")
	}

	// 6. Security headers check
	secHeader := respAuth.Header.Get("X-Content-Type-Options")
	if secHeader != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options: nosniff, got: %s", secHeader)
	}
	frameHeader := respAuth.Header.Get("X-Frame-Options")
	if frameHeader != "SAMEORIGIN" {
		t.Errorf("Expected X-Frame-Options: SAMEORIGIN, got: %s", frameHeader)
	}

	// 7. Form POST Login with correct credentials sets cookie and redirects
	form := url.Values{}
	form.Set("user", cfg.Auth.Username)
	form.Set("pass", cfg.Auth.Password)
	reqPostLogin := httptest.NewRequest("POST", "/login.html?redirect=/library", strings.NewReader(form.Encode()))
	reqPostLogin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	respPostLogin, err := app.Test(reqPostLogin)
	if err != nil {
		t.Fatalf("Failed to execute POST login: %v", err)
	}
	if respPostLogin.StatusCode != fiber.StatusFound {
		t.Errorf("Expected 302 Found redirect, got %d", respPostLogin.StatusCode)
	}
	setCookie := respPostLogin.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, auth.SessionCookieName) {
		t.Errorf("Expected Set-Cookie with %s, got: %s", auth.SessionCookieName, setCookie)
	}

	// 8. Session Cookie authentication on /api/graphql succeeds
	token := auth.CreateSessionToken(cfg.Auth.Username, cfg.Auth.Password)
	reqCookieGQL := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(`{"query":"{ __typename }"}`))
	reqCookieGQL.Header.Set("Content-Type", "application/json")
	reqCookieGQL.Header.Set("Cookie", fmt.Sprintf("%s=%s", auth.SessionCookieName, token))
	respCookieGQL, err := app.Test(reqCookieGQL)
	if err != nil {
		t.Fatalf("Failed to execute cookie GQL: %v", err)
	}
	if respCookieGQL.StatusCode == fiber.StatusUnauthorized {
		t.Errorf("Expected session cookie auth to succeed on /api/graphql, got 401")
	}
}
