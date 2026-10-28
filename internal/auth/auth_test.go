package auth

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestCheckBasicAuth(t *testing.T) {
	user := "admin"
	pass := "secretPassword123"

	// Valid header
	validHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", user, pass)))
	if !CheckBasicAuth(validHeader, user, pass) {
		t.Errorf("Expected valid basic auth to succeed")
	}

	// Invalid password
	wrongPassHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", user, "wrong")))
	if CheckBasicAuth(wrongPassHeader, user, pass) {
		t.Errorf("Expected wrong password to fail")
	}

	// Invalid username
	wrongUserHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "other", pass)))
	if CheckBasicAuth(wrongUserHeader, user, pass) {
		t.Errorf("Expected wrong user to fail")
	}

	// Malformed header
	if CheckBasicAuth("Bearer token", user, pass) {
		t.Errorf("Expected non-Basic auth to fail")
	}
	if CheckBasicAuth("Basic invalidBase64!!", user, pass) {
		t.Errorf("Expected invalid base64 to fail")
	}

	// Empty config credentials
	if CheckBasicAuth(validHeader, "", "") {
		t.Errorf("Expected empty config credentials to fail")
	}
}

func TestSessionToken(t *testing.T) {
	user := "manga_reader"
	pass := "superSecretKey99"

	token := CreateSessionToken(user, pass)
	if token == "" {
		t.Fatalf("Expected non-empty session token")
	}

	// Valid token validation
	if !ValidateSessionToken(token, user, pass) {
		t.Errorf("Expected token to be valid")
	}

	// Wrong password (server secret changed)
	if ValidateSessionToken(token, user, "differentPassword") {
		t.Errorf("Expected validation with wrong password to fail")
	}

	// Wrong user
	if ValidateSessionToken(token, "attacker", pass) {
		t.Errorf("Expected validation with wrong username to fail")
	}

	// Tampered token
	tampered := token[:len(token)-4] + "AAAA"
	if ValidateSessionToken(tampered, user, pass) {
		t.Errorf("Expected tampered token to fail")
	}
}

func TestIsPublicAsset(t *testing.T) {
	publicPaths := []string{
		"/login.html",
		"/logout",
		"/favicon.ico",
		"/favicon.svg",
		"/site.webmanifest",
		"/manifest.json",
		"/apple-touch-icon.png",
		"/assets/index-Dk2.js",
		"/assets/main.css",
		"/sw.js",
		"/registerSW.js",
		"/workbox-c494150e.js",
	}

	for _, p := range publicPaths {
		if !IsPublicAsset(p) {
			t.Errorf("Expected %s to be recognized as public asset", p)
		}
	}

	protectedPaths := []string{
		"/",
		"/library",
		"/api/graphql",
		"/api/v1/meta",
		"/api/v1/source/list",
		"/api/v1/backup/export",
		"/opds/v1.2/library/series",
	}

	for _, p := range protectedPaths {
		if IsPublicAsset(p) {
			t.Errorf("Expected %s to be protected (not public)", p)
		}
	}
}
