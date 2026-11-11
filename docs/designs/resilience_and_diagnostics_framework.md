# Resilience, Error Standardization, and Structured Diagnostics Framework

*   **Author:** devil1210
*   **Status:** Implemented
*   **Date:** 2026-11-11

--------------------------------------------------------------------------------

## Overview

Tsuruyomi is a Go implementation of the Suwayomi-Server (Tachidesk) API. While primary HTTP routing, database persistence, and scrapers are operational, production deployment requires systematic fault isolation, standardized domain error handling, and structured contextual logging using the Go 1.24 standard library (`log/slog`). This design establishes a resilience and diagnostics framework: task-level panic recovery for background workers, centralized Fiber error handling, a typed domain error taxonomy, and structured logging with automated credential redaction.

## Problem Statement

1. **Unprotected Background Goroutines**: Fiber's standard recovery middleware only intercepts panics occurring within the immediate HTTP request goroutine. Background services (such as chapter downloads, automated library updates, periodic self-healing, and database backups) execute on detached goroutines. Any unhandled panic in these workers terminates the entire OS process.
2. **Inconsistent Error Handling**: Handlers and services throughout `internal/router` and `internal/service` construct disparate error strings using `fmt.Errorf` or `errors.New` without standardized error codes or classification. This prevents reliable programmatic classification of transient external errors (such as network scraping timeouts or Cloudflare challenge blocks) and causes inconsistent API error formatting.
3. **Unstructured Production Logging**: Current logging relies on unstructured standard library `log.Printf` calls with ad-hoc formatting. Debugging production failures requires manual string parsing, lacks correlated identifiers (`request_id`, `manga_id`, `source_id`), and provides no structured redaction for sensitive credentials in query parameters, error strings, or auth headers.

## Goals & Non-Goals

### Goals

*   **Isolate Worker Failures**: Recover from recoverable runtime panics in background worker goroutines and HTTP request handlers to prevent daemon process termination.
*   **Standardize Domain Errors**: Define a cohesive, strongly typed domain error hierarchy in `internal/errors` that classifies errors by kind (e.g., `NotFound`, `Conflict`, `UpstreamFailed`, `Unauthorized`, `InvalidInput`) while preserving original root causes.
*   **Standardized Protocol Mapping**: Centrally serialize domain errors to their respective client protocol requirements via Fiber's error handler: JSON error payloads for REST v1, standard error extensions for GraphQL, and HTTP status codes / XML messages for OPDS 1.2 and OPDS 2.0.
*   **Contextual Structured Logging**: Replace unstructured `log.Printf` with standard library `log/slog`, attaching contextual key-value attributes (`request_id`, `source_id`, `manga_id`, `latency_ms`) while enforcing automatic redaction of credentials, cookies, and tokens across keys and values.
*   **Zero Third-Party Dependencies**: Rely exclusively on Go standard library packages (`log/slog`, `context`, `errors`) to prevent third-party dependency expansion and minimize binary footprint.

### Non-Goals

*   **Distributed Tracing / OpenTelemetry Exporter**: This design does not implement OpenTelemetry, Jaeger, or external gRPC telemetry collection agents. All telemetry remains self-contained within stdout/stderr and file logs.
*   **API Breaking Changes**: This design will not alter any external REST endpoints, GraphQL schemas, or OPDS feed structures. All error responses must remain backward-compatible with Tachidesk-WebUI, Mihon, and e-readers.
*   **Automated Retry Loops for All Scrapers**: While error types will distinguish transient errors from permanent errors, redesigning scraper retry policies is out of scope.

## Risks & Mitigations

1. **Unrecoverable Go Runtime Aborts**:
   * *Risk*: Fatal Go runtime panics (such as recursive stack overflows, out-of-memory aborts, or concurrent map read/write violations) cannot be captured by `recover()` and will terminate the process.
   * *Mitigation*: The design does not claim total crash immunity. To prevent concurrent map faults, shared state structures must enforce mutex synchronization or immutability.
2. **Mutex and Database Transaction Poisoning**:
   * *Risk*: A panic occurring while holding a `sync.Mutex` or within an active SQL transaction will leave the lock held or transaction open, causing subsequent worker deadlocks.
   * *Mitigation*: Workers managed by `SafeGo` must strictly defer cleanup primitives (`defer mu.Unlock()`, `defer tx.Rollback()`) before acquiring locks or starting transactions.
3. **Stdout Buffer Saturation & Backpressure**:
   * *Risk*: High-frequency logging (e.g., streaming hundreds of pages via OPDS) directly to unbuffered `os.Stdout` causes blocking `write(2)` syscall contention across goroutines.
   * *Mitigation*: Wrap `os.Stdout` in a synchronized, buffered writer with periodic background flushes.
4. **Credential Leakage in Log Values**:
   * *Risk*: Sensitive data (PostgreSQL passwords, session tokens, API keys) often appears in error strings (e.g., `pq: password authentication failed for dsn ...`) or request URLs, bypassing key-name-only filters.
   * *Mitigation*: Implement value-aware sanitization that strips credentials from URI/DSN patterns (`postgres://user:pass@host` -> `postgres://user:[REDACTED]@host`) and redacts `Authorization: Bearer` values.

## Open Questions

*   **Question 1 (Resolved)**: Should background worker panics trigger an automatic worker restart?
    *   *Resolution*: Schedulers remain active by placing `SafeGo` panic isolation *inside* the job iteration loop (wrapping each task run), avoiding outer loop death without requiring fragile supervisor restart loops. Individual failed download items are marked as `models.DownloadStateError` in memory.
*   **Question 2 (Resolved)**: How should `request_id` correlation be handled?
    *   *Resolution*: The server inspects incoming `X-Request-ID` headers. If present and matching alphanumeric constraints (`^[a-zA-Z0-9_\-]{1,64}$`), it is preserved; otherwise, a fresh UUIDv4 is generated.

## Critical User Journeys (CUJs)

### CUJ 1: Background Worker Panic Isolation and Recovery
*   **Given**: A background chapter downloader task is processing a malformed image buffer or encountering a nil pointer during CBZ archiving.
*   **When**: The worker goroutine triggers a runtime panic.
*   **Then**: The worker recovery interceptor captures the panic, formats a structured error log with the stack trace, marks the chapter download item as `models.DownloadStateError` in the in-memory queue, and keeps the server daemon and concurrent downloads running without interruption.

### CUJ 2: Contextual Triage of Upstream Scraping Failure
*   **Given**: A client requests chapter pages for a manga whose upstream provider changed its HTML structure or activated a new Cloudflare challenge.
*   **When**: The scraper fails to resolve page image URLs.
*   **Then**: The handler logs a structured `slog.Error` containing `request_id`, `manga_id`, `source_id`, and typed error category `KindUpstreamFailed`, while returning an HTTP 502 or GraphQL error to the client without leaking internal database DSNs or stack traces.

### CUJ 3: OPDS E-Reader Navigation with Client Connection Drop
*   **Given**: An e-reader (e.g., KOReader) opens a paginated OPDS PSE (Page Streaming Extension) stream and disconnects mid-transfer.
*   **When**: The HTTP connection context is canceled.
*   **Then**: Context cancellation propagates through `internal/router` to `internal/util/pages.go`, terminating file I/O and zip indexing upon checking `ctx.Done()`, releasing file descriptors without leaking goroutines.

## Proposed Architecture

### Architectural Overview

The framework introduces four interconnected subsystems implemented purely within the Go 1.24 standard library:
1. **Safe Goroutine Dispatcher (`internal/util/safego.go`)**: A goroutine wrapper that intercepts panics in background workers, logs structured traces, and triggers failure callbacks.
2. **Domain Error Model (`internal/errors`)**: A strongly typed error package implementing categorization, root-cause wrapping, contextual attributes, and `errors.Is` compatibility.
3. **Structured Contextual Logging (`internal/util/logger.go`)**: An `slog` logging configuration supporting text/JSON output, contextual correlation (`request_id`), and value-aware credential masking.
4. **Centralized Fiber Error Handling (`internal/router/error_handler.go`)**: Centralized HTTP status and JSON/XML mapping configured via `fiber.Config{ ErrorHandler: ... }`.

```mermaid
flowchart TD
    subgraph Client Requests
        ClientReq[Incoming Client Request] --> FiberApp[Fiber HTTP Engine]
    end

    subgraph HTTP Pipeline
        FiberApp --> ReqIDMiddleware[Fiber native requestid.New]
        ReqIDMiddleware --> RecoverMiddleware[Fiber native recover.New with slog.Error]
        RecoverMiddleware --> RouteHandlers[Domain Route Handlers]
        RouteHandlers -- Returns error --> CentralErrorHandler[fiber.Config.ErrorHandler]
        CentralErrorHandler --> MapProtocol[Map to REST JSON / GraphQL / OPDS Status]
    end

    subgraph Background Services
        Schedulers[Periodic Schedulers] --> SchedulerLoop[Ticker Loop]
        SchedulerLoop --> TaskIteration[Individual Task Iteration]
        TaskIteration --> SafeGo[SafeGo Dispatcher]
        DownloadWorker[Chapter Download Queue] --> SafeGo
        SafeGo --> WorkerPanicGuard{Panic Intercepted?}
        WorkerPanicGuard -- Yes --> LogWorkerPanic[slog.Error Stack Trace]
        LogWorkerPanic --> FailCallback[Record Error State in Queue / Task]
        WorkerPanicGuard -- No --> WorkerExecution[Execute Service Logic]
    end

    subgraph Structured Logging & SRE
        ReqIDMiddleware --> SlogLogger[slog.Logger with Context Attributes]
        RecoverMiddleware --> SlogLogger
        CentralErrorHandler --> SlogLogger
        LogWorkerPanic --> SlogLogger
        WorkerExecution --> SlogLogger
        SlogLogger --> RedactionFilter[slog.ReplaceAttr Secret Redactor]
        RedactionFilter --> StdoutBuffer[Buffered os.Stdout Stream]
    end
```

### 1. Safe Goroutine Dispatcher (`internal/util/safego.go`)

To eliminate daemon crashes from unhandled panics outside HTTP Fiber handlers, all detached asynchronous tasks must execute through `SafeGo`. Persistent scheduler loops place `SafeGo` *inside* the ticker loop:

```go
package util

import (
    "context"
    "log/slog"
    "runtime/debug"
)

// PanicHandler defines an optional failure hook invoked when a goroutine recovers from a panic.
type PanicHandler func(ctx context.Context, workerName string, panicVal any, stackTrace []byte)

// SafeGo spawns an asynchronous task with deferred panic isolation and structured telemetry.
func SafeGo(ctx context.Context, workerName string, fn func(ctx context.Context), onPanic PanicHandler) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                stack := debug.Stack()
                slog.Error("Unhandled panic intercepted in background worker",
                    slog.String("worker", workerName),
                    slog.Any("panic", r),
                    slog.String("stack", string(stack)),
                )
                if onPanic != nil {
                    defer func() {
                        if rHook := recover(); rHook != nil {
                            slog.Error("Panic handler hook itself panicked",
                                slog.String("worker", workerName),
                                slog.Any("panic", rHook),
                            )
                        }
                    }()
                    onPanic(ctx, workerName, r, stack)
                }
            }
        }()
        fn(ctx)
    }()
}
```

### 2. Domain Error Subsystem (`internal/errors`)

The error package provides classification codes and contextual fields while adhering to `errors.Is` and `errors.As`:

```go
package errors

import (
    "fmt"
)

type Kind uint8

const (
    KindInternal Kind = iota
    KindNotFound
    KindInvalidInput
    KindUnauthorized
    KindForbidden
    KindConflict
    KindUpstreamFailed
    KindServiceUnavailable
)

type AppError struct {
    Kind    Kind   `json:"kind"`
    Code    string `json:"code"`
    Message string `json:"message"`
    Err     error  `json:"-"`
}

func (e *AppError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
    }
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
    return e.Err
}

// Is implements custom equivalence matching for errors.Is.
func (e *AppError) Is(target error) bool {
    t, ok := target.(*AppError)
    if !ok {
        return false
    }
    return e.Kind == t.Kind && (t.Code == "" || e.Code == t.Code)
}

// Constructors
func NotFound(code, msg string, err error) *AppError {
    return &AppError{Kind: KindNotFound, Code: code, Message: msg, Err: err}
}

func UpstreamFailed(code, msg string, err error) *AppError {
    return &AppError{Kind: KindUpstreamFailed, Code: code, Message: msg, Err: err}
}

func InvalidInput(code, msg string, err error) *AppError {
    return &AppError{Kind: KindInvalidInput, Code: code, Message: msg, Err: err}
}
```

### 3. Protocol Error Adapters & Centralized Error Handler

Rather than writing manual adapter calls in every route handler, Fiber is configured with a centralized error handler (`fiber.Config{ ErrorHandler: CentralErrorHandler }`):

```go
package router

import (
    "errors"
    "net/http"
    "strings"

    "github.com/gofiber/fiber/v2"
    apperrors "tsuruyomi/internal/errors"
)

func CentralErrorHandler(c *fiber.Ctx, err error) error {
    var appErr *apperrors.AppError
    status := http.StatusInternalServerError
    code := "INTERNAL_ERROR"
    message := "An internal server error occurred"

    if errors.As(err, &appErr) {
        message = appErr.Message
        code = appErr.Code
        switch appErr.Kind {
        case apperrors.KindNotFound: status = http.StatusNotFound
        case apperrors.KindInvalidInput: status = http.StatusBadRequest
        case apperrors.KindUnauthorized: status = http.StatusUnauthorized
        case apperrors.KindForbidden: status = http.StatusForbidden
        case apperrors.KindConflict: status = http.StatusConflict
        case apperrors.KindUpstreamFailed: status = http.StatusBadGateway
        case apperrors.KindServiceUnavailable: status = http.StatusServiceUnavailable
        }
    } else {
        var fiberErr *fiber.Error
        if errors.As(err, &fiberErr) {
            status = fiberErr.Code
            message = fiberErr.Message
        }
    }

    path := c.Path()
    if strings.HasPrefix(path, "/api/graphql") {
        return c.Status(http.StatusOK).JSON(fiber.Map{
            "errors": []fiber.Map{{"message": message, "extensions": fiber.Map{"code": code}}},
        })
    }
    if strings.HasPrefix(path, "/opds") || strings.HasPrefix(path, "/api/opds") {
        return c.Status(status).SendString(message)
    }
    return c.Status(status).JSON(fiber.Map{
        "error": message,
        "code": code,
    })
}
```

### 4. Structured Logging & Sensitive Masking (`internal/util/logger.go`)

Initializes `log/slog` driven by `config.LoggingConfig`, supporting value-aware sanitization and context retrieval:

```go
package util

import (
    "bufio"
    "context"
    "log/slog"
    "os"
    "regexp"
    "strings"

    "tsuruyomi/internal/config"
)

type ctxKey string
const ContextLoggerKey ctxKey = "ctx_logger"

var (
    dsnRegex   = regexp.MustCompile(`(postgres(?:ql)?://[^:]+:)([^@]+)(@)`)
    tokenRegex = regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9_\-\.]+)`)
)

func InitLogger(cfg config.LoggingConfig) *slog.Logger {
    var logLevel slog.Level
    switch strings.ToLower(cfg.Level) {
    case "debug": logLevel = slog.LevelDebug
    case "warn":  logLevel = slog.LevelWarn
    case "error": logLevel = slog.LevelError
    default:      logLevel = slog.LevelInfo
    }

    opts := &slog.HandlerOptions{
        Level: logLevel,
        ReplaceAttr: redactSensitiveAttrs,
    }

    writer := bufio.NewWriterSize(os.Stdout, 32*1024)
    var handler slog.Handler
    if strings.ToLower(cfg.Format) == "json" {
        handler = slog.NewJSONHandler(writer, opts)
    } else {
        handler = slog.NewTextHandler(writer, opts)
    }

    logger := slog.New(handler)
    slog.SetDefault(logger)
    return logger
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
    if ctx == nil {
        return slog.Default()
    }
    if logger, ok := ctx.Value(ContextLoggerKey).(*slog.Logger); ok {
        return logger
    }
    return slog.Default()
}

func redactSensitiveAttrs(groups []string, a slog.Attr) slog.Attr {
    key := strings.ToLower(a.Key)
    if strings.Contains(key, "password") ||
       strings.Contains(key, "secret") ||
       strings.Contains(key, "cookie") ||
       strings.Contains(key, "authorization") {
        return slog.String(a.Key, "[REDACTED]")
    }
    if a.Value.Kind() == slog.KindString {
        str := a.Value.String()
        if dsnRegex.MatchString(str) {
            str = dsnRegex.ReplaceAllString(str, "${1}[REDACTED]${3}")
        }
        if tokenRegex.MatchString(str) {
            str = tokenRegex.ReplaceAllString(str, "${1}[REDACTED]")
        }
        return slog.String(a.Key, str)
    }
    return a
}
```

### 5. Context Propagation & Async Detachment

To avoid goroutine leaks while ensuring background tasks survive client request completion:
- HTTP requests utilize Fiber's `c.UserContext()` injected with `request_id` via Fiber's native `requestid.New()`.
- If an HTTP handler dispatches a background task (e.g., triggering a backup or download), it must detach the context via `context.WithoutCancel(c.UserContext())` so client disconnects do not terminate asynchronous work.
- Downstream streaming endpoints (e.g., OPDS PSE page retrieval) cooperatively check `ctx.Done()` to stop disk I/O when clients disconnect.

## Alternatives Considered

### Alternative A: Distributed Tracing via External Agents (OpenTelemetry SDK / Jaeger)

*   **Pros**: Full distributed trace propagation across services, out-of-the-box Grafana dashboards, standard metrics export.
*   **Cons**: Introduces multiple external dependencies, inflates memory footprint by 30 MB – 80 MB, and adds substantial configuration complexity for single-node homelab deployments.
*   **Reason for Rejection**: Tsuruyomi is architected to operate within a 35 MB – 50 MB RAM budget on lightweight hardware (Raspberry Pi, LXC). Importing external OpenTelemetry dependencies violates the target memory budget.

### Alternative B: Third-Party Structured Logging Frameworks (`uber-go/zap` or `rs/zerolog`)

*   **Pros**: High-throughput JSON logging benchmarks, rich ecosystem of specialized log encoders.
*   **Cons**: Introduces third-party dependency chains, potential versioning conflicts, and redundant functionality given Go's native standard library `log/slog`.
*   **Reason for Rejection**: Go 1.21+ `log/slog` provides standard library structured logging without external dependencies, eliminating third-party module dependencies and preventing binary growth.

### Alternative C: Crash-Only Process Restarts (Relying on Systemd/Docker Container Restarts)

*   **Pros**: Zero in-application recovery code needed; simplifies runtime assumptions by resetting process memory on panic.
*   **Cons**: A single corrupt image file in an asynchronous chapter download causes recurring crash loops that terminate the web server process, breaking active reader sessions for all users.
*   **Reason for Rejection**: Background worker panics must not impact HTTP server availability. Isolating background failures in memory prevents non-fatal worker panics from disrupting running HTTP handlers.

## Detailed Implementation Plan

The implementation proceeds in four sequential phases: foundational primitives, runtime infrastructure, handler & service migration, and documentation alignment.

### Phase 1: Foundational Primitives (Domain Errors & Safe Goroutines)

1.  **`internal/errors/errors.go`** (NEW):
    *   *Rationale*: Implements domain error type `AppError`, `Kind` enum definitions, `Is(target error) bool`, and standard constructors (`NotFound`, `InvalidInput`, `UpstreamFailed`, `Conflict`). Enables typed error classification without third-party libraries.
2.  **`internal/errors/errors_test.go`** (NEW):
    *   *Rationale*: Unit tests validating `errors.Is`, `errors.As`, unwrapping behavior, and formatted string output.
3.  **`internal/util/safego.go`** (NEW):
    *   *Rationale*: Implements `SafeGo` goroutine dispatcher with deferred panic isolation, stack trace capture via `runtime/debug.Stack()`, and structured telemetry reporting via `slog.Error`.
4.  **`internal/util/safego_test.go`** (NEW):
    *   *Rationale*: Unit tests confirming that intentional worker panics are captured without terminating the test binary and that failure hook callbacks are invoked upon panic.

### Phase 2: Telemetry & HTTP Middleware Upgrades

5.  **`internal/util/logger.go`** (NEW):
    *   *Rationale*: Implements `InitLogger(cfg config.LoggingConfig)` and `LoggerFromContext(ctx)`, supporting `slog.TextHandler` or `slog.JSONHandler` with value-aware credential masking (`redactSensitiveAttrs`).
6.  **`internal/util/logger_test.go`** (NEW):
    *   *Rationale*: Unit tests verifying redaction of database DSNs, bearer tokens, passwords, and authorization headers.
7.  **`internal/config/config.go`** (MODIFIED):
    *   *Rationale*: Adds `Format` field (`"text"` or `"json"`) to `LoggingConfig` with `"text"` default, updates `config.Load()` with Viper default `v.SetDefault("logging.format", "text")`, and updates `Save()` serialization template.
8.  **`internal/router/router.go`** (MODIFIED):
    *   *Rationale*: Replaces custom middleware with Fiber's native `requestid.New()`, updates `recover.New()` with custom `StackTraceHandler` writing to `slog.Error`, configures `fiber.Config{ ErrorHandler: CentralErrorHandler }`, and replaces legacy `logger.New()` to prevent duplicate request logs.
9.  **`internal/router/error_handler.go`** (NEW):
    *   *Rationale*: Implements `CentralErrorHandler` mapping `AppError` and `fiber.Error` instances to REST JSON, GraphQL 200 envelopes with error extensions, and OPDS status responses.
10. **`internal/router/error_handler_test.go`** (NEW):
    *   *Rationale*: Unit tests verifying HTTP status code and response payload formatting across REST, GraphQL, and OPDS paths.

### Phase 3: Service & Handler Integration

11. **`cmd/server/main.go`** (MODIFIED):
    *   *Rationale*: Initializes `util.InitLogger(cfg.Logging)` early in `main()` and wraps detached background initialization goroutines (including `service.ReconcileDownloads`) in `util.SafeGo`.
12. **`internal/service/download_manager.go`** (MODIFIED):
    *   *Rationale*: Wraps chapter download processing inside `dm.workerLoop` with `util.SafeGo` capturing the active chapter item via closure, setting `dm.queue[currIdx].State = models.DownloadStateError` upon panic, and broadcasting status over WebSockets.
13. **`internal/service/update_manager.go`** (MODIFIED):
    *   *Rationale*: Wraps individual manga update iterations inside the scheduler with `util.SafeGo` so a single scraper failure does not terminate the updater loop.
14. **`internal/service/healer.go`** (MODIFIED):
    *   *Rationale*: Wraps individual 6-hour audit/heal sweeps inside the ticker loop with `util.SafeGo`.
15. **`internal/service/backup_service.go`** (MODIFIED):
    *   *Rationale*: Wraps async backup restore tasks and scheduled backup executions with `util.SafeGo`.
16. **`internal/router/handlers/category.go`** (MODIFIED):
    *   *Rationale*: Pilot migration returning `internal/errors` constructors (`NotFound`, `InvalidInput`) directly to Fiber's centralized error handler.
17. **`internal/router/handlers/manga.go`** (MODIFIED):
    *   *Rationale*: Pilot migration returning typed domain errors for manga lookup and chapter page streaming.
18. **`internal/router/handlers/graphql.go`** (MODIFIED):
    *   *Rationale*: Updates error returns to integrate with GraphQL error extension formatting.

### Phase 4: EGM Contract & Documentation Alignment

19. **`internal/errors/README.md`** (NEW):
    *   *Rationale*: Elephant-and-Goldfish Model (EGM) Component Package contract documenting `AppError`, `Kind`, constructors, and verification commands (`go test ./...`).
20. **`internal/util/README.md`** (MODIFIED):
    *   *Rationale*: Documents new `SafeGo` functions and structured `slog` logger interfaces in the package contract.
21. **`internal/router/README.md`** (MODIFIED):
    *   *Rationale*: Documents native `requestid` correlation middleware and centralized error handling behavior in the package contract.
22. **`internal/README.md`** (MODIFIED):
    *   *Rationale*: Updates intermediate blueprint to link to the new `internal/errors/README.md` package contract.

## Rollback Plan

1. **Configuration Backward Compatibility**:
   * If a user downgrades the binary to an earlier release while retaining a `config.yaml` containing `logging.format`, Viper ignores unrecognized keys without failing startup.
2. **Failure Recovery Triggers**:
   * If log ingestion fails due to unexpected formatting, setting `logging.format: "text"` in `config.yaml` or `SUWAYOMI_LOGGING_FORMAT=text` immediately restores traditional plain-text console output without code changes.
   * If client integrations fail due to centralized error handling, handlers can temporarily fall back to explicit `c.Status().SendString()` returns.
3. **Rollback Verification Steps**:
   * To safely roll back changes:
     ```bash
     git revert HEAD
     go build -o tsuruyomi ./cmd/server
     go test ./...
     ```
   * Confirm the restored binary boots cleanly and executes standard test suites.
