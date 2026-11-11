# Authentication & Session Security (`internal/auth`)

## Overview
The `internal/auth` package provides authentication mechanisms and token validation utilities for Tsuruyomi. It handles HTTP Basic authentication verification, HMAC-SHA256 session token generation and lifecycle verification (with expiration and future clock-skew tolerance), HMAC-SHA256 signed JSON Web Tokens (HS256), and path-based whitelisting for public assets, PWA manifests, and static WebUI endpoints.

## Interface
- `[auth.go:SessionCookieName]`: `const SessionCookieName = "tsuruyomi_session"`
- `[auth.go:SessionDuration]`: `const SessionDuration = 30 * 24 * time.Hour`
- `[auth.go:CheckBasicAuth]`: `func CheckBasicAuth(authHeader, expectedUser, expectedPass string) bool`
- `[auth.go:CreateSessionToken]`: `func CreateSessionToken(username, password string) string`
- `[auth.go:CreateJWT]`: `func CreateJWT(username, tokenType string, duration time.Duration, secret string) (string, error)`
- `[auth.go:ValidateJWT]`: `func ValidateJWT(tokenStr, expectedUsername, secret string) bool`
- `[auth.go:ValidateSessionToken]`: `func ValidateSessionToken(token, expectedUsername, expectedPassword string) bool`
- `[auth.go:IsPublicAsset]`: `func IsPublicAsset(path string) bool`

## Invariants
- `[auth.go:CheckBasicAuth]`: Returns `false` if `expectedUser` or `expectedPass` is empty, header is not prefixed with `'Basic '`, base64 decoding fails, or credentials format is not `'username:password'`; uses `subtle.ConstantTimeCompare` for credential validation.
- `[auth.go:CreateJWT]`: Generates HS256 JWT header fixed to `{"alg":"HS256","typ":"JWT"}` with payload claims `sub` (username), `iat` (current UNIX timestamp), `exp` (current UNIX timestamp + duration in seconds), and `type`.
- `[auth.go:ValidateJWT]`: Returns `false` if `tokenStr`, `expectedUsername`, or `secret` is empty, token lacks exactly three dot-separated parts, HMAC-SHA256 signature verification fails via `subtle.ConstantTimeCompare` (evaluating both raw and padded base64 URL encoding), payload JSON unmarshaling fails, `sub` does not match `expectedUsername` in constant time, or `claims.Exp <= current UNIX timestamp`.
- `[auth.go:ValidateSessionToken]`: Returns `false` if `token`, `expectedUsername`, or `expectedPassword` is empty.
- `[auth.go:ValidateSessionToken]`: If `token` contains exactly two dots, validation delegates to `ValidateJWT(token, expectedUsername, expectedPassword)`.
- `[auth.go:ValidateSessionToken]`: Session token must be base64 URL-encoded containing three colon-separated fields (`username:timestamp:signature`); returns `false` if `username` does not match `expectedUsername` in constant time, timestamp parsing fails, token age exceeds `SessionDuration` (30 days), timestamp is more than 1 minute in the future, or HMAC-SHA256 signature does not match in constant time.
- `[auth.go:IsPublicAsset]`: Returns `true` only for exact public web routes (`/login`, `/login.html`, `/logout`, `/robots.txt`, `/favicon.ico`, `/favicon.svg`, `/sw.js`, `/registerSW.js`), manifest/icon prefixes, `/assets/*` paths, `/workbox-*` files, and uncredentialed image endpoints (`/api/v1/manga/*/thumbnail`, `/api/v1/extension/icon/*`, `/api/v1/source/*/icon`); returns `false` for all other paths.

## Side Effects
- `[auth.go]`: None (pure computational validation and token generation functions without disk, network, or state mutations).

## Verification
- Run: `go test ./...`
