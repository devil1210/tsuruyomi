package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	SessionCookieName = "tsuruyomi_session"
	SessionDuration   = 30 * 24 * time.Hour // 30 days
)

// CheckBasicAuth validates an HTTP Basic authorization header using constant-time comparison.
func CheckBasicAuth(authHeader, expectedUser, expectedPass string) bool {
	if expectedUser == "" || expectedPass == "" {
		return false
	}
	if !strings.HasPrefix(authHeader, "Basic ") {
		return false
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(authHeader[6:]))
	if err != nil {
		return false
	}
	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 {
		return false
	}

	userMatch := subtle.ConstantTimeCompare([]byte(pair[0]), []byte(expectedUser)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(pair[1]), []byte(expectedPass)) == 1
	return userMatch && passMatch
}

// CreateSessionToken creates a signed HMAC-SHA256 session token.
func CreateSessionToken(username, password string) string {
	now := time.Now().Unix()
	payload := fmt.Sprintf("%s:%d", username, now)

	h := hmac.New(sha256.New, []byte(password))
	h.Write([]byte(payload))
	sig := hex.EncodeToString(h.Sum(nil))

	fullToken := fmt.Sprintf("%s:%s", payload, sig)
	return base64.URLEncoding.EncodeToString([]byte(fullToken))
}

// ValidateSessionToken verifies the authenticity, timestamp, and signature of a session token.
func ValidateSessionToken(token, expectedUsername, expectedPassword string) bool {
	if token == "" || expectedUsername == "" || expectedPassword == "" {
		return false
	}

	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return false
	}

	parts := strings.Split(string(raw), ":")
	if len(parts) != 3 {
		return false
	}

	username, tsStr, sig := parts[0], parts[1], parts[2]

	// Verify username match in constant time
	if subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) != 1 {
		return false
	}

	// Verify timestamp
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}

	tokenTime := time.Unix(ts, 0)
	now := time.Now()

	// Token expired or in the future
	if now.Sub(tokenTime) > SessionDuration || tokenTime.After(now.Add(1*time.Minute)) {
		return false
	}

	// Verify signature in constant time
	h := hmac.New(sha256.New, []byte(expectedPassword))
	h.Write([]byte(fmt.Sprintf("%s:%d", username, ts)))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(sig), []byte(expectedSig)) == 1
}

// IsPublicAsset determines if an incoming request path should be accessible without authentication.
func IsPublicAsset(path string) bool {
	// Exact public paths
	if path == "/login.html" || path == "/logout" || path == "/robots.txt" || path == "/favicon.ico" || path == "/favicon.svg" {
		return true
	}

	// Manifests, icons, and metadata
	if strings.HasSuffix(path, "manifest.json") ||
		strings.HasSuffix(path, "site.webmanifest") ||
		strings.HasPrefix(path, "/apple-touch-icon") ||
		strings.HasPrefix(path, "/web-app-manifest") ||
		strings.HasPrefix(path, "/favicon-") {
		return true
	}

	// Compiled WebUI frontend bundles (CSS, JS, Fonts)
	if strings.HasPrefix(path, "/assets/") {
		return true
	}

	// Service Worker files
	if path == "/sw.js" || path == "/registerSW.js" || strings.HasPrefix(path, "/workbox-") {
		return true
	}

	return false
}
