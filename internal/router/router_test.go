package router

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	if respPostLogin.StatusCode != fiber.StatusSeeOther && respPostLogin.StatusCode != fiber.StatusFound {
		t.Errorf("Expected 303 See Other redirect, got %d", respPostLogin.StatusCode)
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

func TestTsumiruGraphQLCompatibility(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Mode: "none",
		},
	}
	app := Setup(cfg)

	// 1. Tsumiru legacy user fields detection probe: { __type(name: "ChapterType") { fields { name } } }
	reqProbe := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(`{"query":"{ __type(name: \"ChapterType\") { fields { name } } }"}`))
	reqProbe.Header.Set("Content-Type", "application/json")
	respProbe, err := app.Test(reqProbe)
	if err != nil {
		t.Fatalf("Probe request failed: %v", err)
	}
	bodyProbe := make([]byte, 2048)
	n, _ := respProbe.Body.Read(bodyProbe)
	respStr := string(bodyProbe[:n])
	if !strings.Contains(respStr, `"name":"user"`) {
		t.Errorf("Expected probe response to contain field 'user', got: %s", respStr)
	}

	// 2. WebUIFlavor introspection: __type(name: "WebUIFlavor") { enumValues { name } }
	reqEnum := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(`{"query":"{ __type(name: \"WebUIFlavor\") { enumValues { name } } }"}`))
	reqEnum.Header.Set("Content-Type", "application/json")
	respEnum, err := app.Test(reqEnum)
	if err != nil {
		t.Fatalf("Enum request failed: %v", err)
	}
	bodyEnum := make([]byte, 2048)
	n, _ = respEnum.Body.Read(bodyEnum)
	respStr = string(bodyEnum[:n])
	if !strings.Contains(respStr, `"name":"PREVIEW"`) {
		t.Errorf("Expected enum response to contain 'PREVIEW', got: %s", respStr)
	}

	// 3. CurrentAccount query
	reqAccount := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(`{"query":"query CurrentAccount { user { id username roles permissions } }"}`))
	reqAccount.Header.Set("Content-Type", "application/json")
	respAccount, err := app.Test(reqAccount)
	if err != nil {
		t.Fatalf("Account request failed: %v", err)
	}
	bodyAccount := make([]byte, 2048)
	n, _ = respAccount.Body.Read(bodyAccount)
	respStr = string(bodyAccount[:n])
	if !strings.Contains(respStr, `"username":"devil"`) {
		t.Errorf("Expected account response to contain user 'devil', got: %s", respStr)
	}

	// 4. GetMangaTrackRecords (Tsumiru recommendations dependency)
	reqTracks := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(`{
		"operationName": "GetMangaTrackRecords",
		"query": "query GetMangaTrackRecords($mangaId: Int!) { manga(id: $mangaId) { id trackRecords { nodes { id trackerId remoteId } totalCount } } }",
		"variables": { "mangaId": 2408 }
	}`))
	reqTracks.Header.Set("Content-Type", "application/json")
	respTracks, err := app.Test(reqTracks)
	if err != nil {
		t.Fatalf("GetMangaTrackRecords failed: %v", err)
	}
	bodyTracks := make([]byte, 2048)
	n, _ = respTracks.Body.Read(bodyTracks)
	respStr = string(bodyTracks[:n])
	if !strings.Contains(respStr, `"trackRecords"`) || !strings.Contains(respStr, `"manga"`) {
		t.Errorf("Expected response to contain manga and trackRecords, got: %s", respStr)
	}

	// 5. Server Database Settings DSN parsing
	cfg.Database.DSN = "postgres://customuser:custompass@192.168.1.109:5432/tsuruyomi?sslmode=disable"
	reqSettings := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(`{
		"query": "query DatabaseSettings { settings { databaseUrl databaseUsername databaseType } }"
	}`))
	reqSettings.Header.Set("Content-Type", "application/json")
	respSettings, err := app.Test(reqSettings)
	if err != nil {
		t.Fatalf("DatabaseSettings request failed: %v", err)
	}
	bodySettings := make([]byte, 2048)
	n, _ = respSettings.Body.Read(bodySettings)
	respStr = string(bodySettings[:n])
	if !strings.Contains(respStr, `"databaseUsername":"customuser"`) || !strings.Contains(respStr, `"databaseUrl":"postgresql://192.168.1.109:5432/tsuruyomi"`) {
		t.Errorf("Expected dynamic database settings in response, got: %s", respStr)
	}
}

func TestSimpleLoginAuthIntegration(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Subpath: "",
		},
		Data: config.DataConfig{
			RootDir: "./data",
		},
		Auth: config.AuthConfig{
			Mode:     "simple_login",
			Username: "devil",
			Password: "mypassword123",
		},
	}

	app := Setup(cfg)

	// 1. Unauthenticated request to REST endpoint in simple_login mode should return 401 WITHOUT WWW-Authenticate
	reqAPI := httptest.NewRequest("GET", "/api/v1/meta", nil)
	respAPI, err := app.Test(reqAPI)
	if err != nil {
		t.Fatalf("Failed to execute REST request: %v", err)
	}
	if respAPI.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for /api/v1/meta without auth, got %d", respAPI.StatusCode)
	}
	wwwAuth := respAPI.Header.Get("WWW-Authenticate")
	if wwwAuth != "" {
		t.Errorf("Expected NO WWW-Authenticate header in simple_login mode, got: %s", wwwAuth)
	}

	// 2. Unauthenticated GraphQL query in simple_login mode should return 200 OK with UnauthorizedException
	reqGQL := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(`{"query":"query GET_ABOUT { aboutServer { name } }"}`))
	reqGQL.Header.Set("Content-Type", "application/json")
	respGQL, err := app.Test(reqGQL)
	if err != nil {
		t.Fatalf("Failed to execute GQL query: %v", err)
	}
	if respGQL.StatusCode != fiber.StatusOK {
		t.Errorf("Expected 200 OK for GQL query with auth error, got %d", respGQL.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(respGQL.Body)
	if !strings.Contains(string(bodyBytes), "suwayomi.tachidesk.server.user.UnauthorizedException") {
		t.Errorf("Expected UnauthorizedException in GQL response, got: %s", string(bodyBytes))
	}

	// 3. Invalid credentials in USER_LOGIN mutation should fail
	invalidLoginJSON := `{"query":"mutation USER_LOGIN($username: String!, $password: String!) { login(input: { username: $username, password: $password }) { accessToken refreshToken } }","variables":{"username":"devil","password":"wrongpassword"}}`
	reqInvalidLogin := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(invalidLoginJSON))
	reqInvalidLogin.Header.Set("Content-Type", "application/json")
	respInvalidLogin, err := app.Test(reqInvalidLogin)
	if err != nil {
		t.Fatalf("Failed to execute invalid login: %v", err)
	}
	bodyInvalidBytes, _ := io.ReadAll(respInvalidLogin.Body)
	if !strings.Contains(string(bodyInvalidBytes), "Invalid username or password") {
		t.Errorf("Expected 'Invalid username or password' error, got: %s", string(bodyInvalidBytes))
	}

	// 4. Valid credentials in USER_LOGIN mutation should return accessToken, refreshToken, and set cookie
	validLoginJSON := `{"query":"mutation USER_LOGIN($username: String!, $password: String!) { login(input: { username: $username, password: $password }) { accessToken refreshToken } }","variables":{"username":"devil","password":"mypassword123"}}`
	reqValidLogin := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(validLoginJSON))
	reqValidLogin.Header.Set("Content-Type", "application/json")
	respValidLogin, err := app.Test(reqValidLogin)
	if err != nil {
		t.Fatalf("Failed to execute valid login: %v", err)
	}
	if respValidLogin.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 OK for valid login, got %d", respValidLogin.StatusCode)
	}
	bodyValidBytes, _ := io.ReadAll(respValidLogin.Body)

	var loginResp struct {
		Data struct {
			Login struct {
				AccessToken  string `json:"accessToken"`
				RefreshToken string `json:"refreshToken"`
			} `json:"login"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyValidBytes, &loginResp); err != nil {
		t.Fatalf("Failed to parse login response JSON: %v, raw: %s", err, string(bodyValidBytes))
	}
	if loginResp.Data.Login.AccessToken == "" || loginResp.Data.Login.RefreshToken == "" {
		t.Fatalf("Expected non-empty accessToken and refreshToken, got: %s", string(bodyValidBytes))
	}

	// Verify session cookie was set
	setCookie := respValidLogin.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, auth.SessionCookieName) {
		t.Errorf("Expected Set-Cookie header with session cookie, got: %s", setCookie)
	}

	// 5. Query GraphQL using Bearer <accessToken> should succeed
	reqAuthGQL := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(`{"query":"query GET_ABOUT { aboutServer { name } }"}`))
	reqAuthGQL.Header.Set("Content-Type", "application/json")
	reqAuthGQL.Header.Set("Authorization", "Bearer "+loginResp.Data.Login.AccessToken)
	respAuthGQL, err := app.Test(reqAuthGQL)
	if err != nil {
		t.Fatalf("Failed to execute authenticated GQL: %v", err)
	}
	bodyAuthBytes, _ := io.ReadAll(respAuthGQL.Body)
	if !strings.Contains(string(bodyAuthBytes), `"name":"Tsuruyomi"`) {
		t.Errorf("Expected 'name':'Tsuruyomi' in authenticated GQL response, got: %s", string(bodyAuthBytes))
	}

	// 6. Refresh token mutation should issue a new accessToken
	refreshJSON := fmt.Sprintf(`{"query":"mutation REFRESH_ACCESS_TOKEN($refreshToken: String!) { refreshToken(input: { refreshToken: $refreshToken }) { accessToken } }","variables":{"refreshToken":%q}}`, loginResp.Data.Login.RefreshToken)
	reqRefresh := httptest.NewRequest("POST", "/api/graphql", strings.NewReader(refreshJSON))
	reqRefresh.Header.Set("Content-Type", "application/json")
	respRefresh, err := app.Test(reqRefresh)
	if err != nil {
		t.Fatalf("Failed to execute refresh mutation: %v", err)
	}
	bodyRefreshBytes, _ := io.ReadAll(respRefresh.Body)
	var refreshResp struct {
		Data struct {
			RefreshToken struct {
				AccessToken string `json:"accessToken"`
			} `json:"refreshToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyRefreshBytes, &refreshResp); err != nil {
		t.Fatalf("Failed to parse refresh response JSON: %v, raw: %s", err, string(bodyRefreshBytes))
	}
	if refreshResp.Data.RefreshToken.AccessToken == "" {
		t.Errorf("Expected new accessToken in refresh response, got: %s", string(bodyRefreshBytes))
	}

	// 7. External clients using HTTP Basic Auth header still succeed (backwards compatibility)
	reqBasicAuth := httptest.NewRequest("GET", "/api/v1/meta", nil)
	cred := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cfg.Auth.Username, cfg.Auth.Password)))
	reqBasicAuth.Header.Set("Authorization", "Basic "+cred)
	respBasicAuth, err := app.Test(reqBasicAuth)
	if err != nil {
		t.Fatalf("Failed to execute basic auth in simple_login mode: %v", err)
	}
	if respBasicAuth.StatusCode == fiber.StatusUnauthorized {
		t.Errorf("Expected HTTP Basic Auth to still be accepted in simple_login mode, got 401")
	}
}
