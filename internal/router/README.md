# HTTP Router & Routing Middleware (`internal/router`)

## Overview
The `internal/router` package configures and provisions the central Fiber HTTP application router for Tsuruyomi. It registers security and logging middleware, enforces authentication rules (supporting disabled auth, HTTP Basic auth, session cookies, JWT bearer tokens, and modern SPA login mode), provisions route groups (REST v1, OPDS 1.2, OPDS 2.0, WebSockets, WebUI static asset serving, and SPA fallback), and delegates extension/source proxying to an optional JVM sidecar.

## Interface
- `[router.go:Setup]`: `func Setup(cfg *config.Config) *fiber.App` instantiating and configuring the complete HTTP Fiber application.
- `[error_handler.go:CentralErrorHandler]`: Centralized Fiber error handler translating domain `AppError` and `fiber.Error` types into protocol-compliant responses across REST v1, GraphQL, and OPDS feeds.
- [Route Handlers Contract](./handlers/README.md): Detailed contract for REST endpoints, GraphQL resolvers, OPDS feeds, and WebSocket managers.

## Invariants
- `[router.go:Setup]`: Configures Fiber's native `requestid` middleware using header `X-Request-ID` and injects contextual `slog.Logger` into the user request context.
- `[router.go:Setup]`: Configures Fiber's `recover` middleware with structured stack trace reporting routed to `slog.Error`.
- `[error_handler.go:CentralErrorHandler]`: Translates domain `AppError` kinds to HTTP status codes (`NotFound` -> 404, `InvalidInput` -> 400, `Unauthorized` -> 401, `Forbidden` -> 403, `Conflict` -> 409, `UpstreamFailed` -> 502, `ServiceUnavailable` -> 503). Formats `/api/graphql` errors as HTTP 200 with GraphQL error extensions; formats OPDS feeds with raw HTTP status and text; formats REST endpoints as JSON `{error, code}`.
- `[router.go:Setup]`: Fiber App `BodyLimit` is constrained to 64MB (`64 * 1024 * 1024`).
- `[router.go:Setup]`: Fiber App `ReadTimeout` and `WriteTimeout` are set to 0 to prevent timing out long-lived WebSocket connections.
- `[router.go:Setup]`: Route prefix defaults to `'/'` if `cfg.Server.Subpath` is empty.
- `[router.go:Setup]`: Authentication check is bypassed if `cfg.Auth.Mode` is empty or `'none'`.
- `[router.go:Setup]`: HTTP `OPTIONS` pre-flight requests bypass authentication and return HTTP 204 No Content.
- `[router.go:Setup]`: Requests matching `auth.IsPublicAsset(path)` always bypass authentication.
- `[router.go:Setup]`: Bearer tokens must validate against `auth.ValidateJWT` or `auth.ValidateSessionToken`; other Authorization headers validate via `auth.CheckBasicAuth`.
- `[router.go:Setup]`: Session cookie `auth.SessionCookieName` must validate against `auth.ValidateJWT` or `auth.ValidateSessionToken`.
- `[router.go:Setup]`: Under modern auth mode (`'simple_login'` or `'ui_login'`), unauthenticated `/api/graphql` and `/graphql` pass through to GraphQL handler, unauthenticated `/api/` and `/opds` return 401 JSON without `WWW-Authenticate` header, and browser requests pass through to SPA `index.html`.
- `[router.go:Setup]`: Under basic auth mode, unauthenticated API, OPDS, and GraphQL requests receive 401 JSON with `WWW-Authenticate: Basic realm="Suwayomi"` header.
- `[router.go:Setup]`: Under basic auth mode, unauthenticated browser requests receive 401 string with `WWW-Authenticate`; other modes redirect to `<subpath>/login?redirect=<path>`.
- `[router.go:Setup]`: Requests with path containing `'/icon'` are strictly excluded from being forwarded to the sidecar.
- `[router.go:Setup]`: When `cfg.Sidecar.Enabled` is true, non-icon extension and source requests are forwarded to `cfg.Sidecar.BaseURL`.
- `[router.go:Setup]`: SPA fallback catch-all (`'/*'`) passes through if path starts with `/api/`, `/opds/`, `/graphql`, ends with `/update-server`, or contains `'.'`; otherwise serves `index.html`.

## Side Effects
- `[router.go:Setup]`: Registers panic recovery middleware capturing unhandled panics and writing structured stack traces to `slog.Error`.
- `[router.go:Setup]`: Registers structured request logging middleware emitting contextual `slog` events containing `request_id`, method, path, status, and latency.
- `[router.go:Setup]`: Sets security response headers (`X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN`, `X-XSS-Protection: 1; mode=block`, `Referrer-Policy: strict-origin-when-cross-origin`) on all responses.
- `[router.go:Setup]`: Sets CORS headers allowing all origins, standard request headers, and common HTTP methods.
- `[router.go:Setup]`: Mutates Fiber context locals by setting `c.Locals("config", cfg)` and `c.Locals("user", cfg.Auth.Username)`.
- `[router.go:Setup]`: Reads `index.html` from `<rootDir>/webUI/index.html` and sets no-cache headers for SPA routes.
- `[router.go:Setup]`: Serves static assets with gzip compression from `<rootDir>/webUI`.
- `[router.go:Setup]`: Performs reverse proxy forwarding to external sidecar service at `cfg.Sidecar.BaseURL` when enabled.

## Verification
- Run: `go test ./...`
