<p align="center">
  <img src="docs/assets/tsuruyomi-icon.png" width="160" height="160" alt="Tsuruyomi Logo">
</p>

# Tsuruyomi Blueprint

<p align="center">
  <b>Ultra-fast, lightweight native Go backend replacement for Suwayomi-Server (Tachidesk) with OPDS 1.2 & 2.0</b>
</p>

<p align="center">
  <a href="https://github.com/devil1210/tsuruyomi/releases"><img src="https://img.shields.io/badge/version-v1.0.1-rose?style=flat-square" alt="Version"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go" alt="Go"></a>
  <a href="https://github.com/devil1210/tsuruyomi/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-GPL--3.0-blue?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/OPDS-1.2%20%2B%202.0%20(PSE)-emerald?style=flat-square" alt="OPDS 1.2 + 2.0">
  <img src="https://img.shields.io/badge/RAM-%3C%2045MB-crimson?style=flat-square" alt="RAM < 45MB">
  <img src="https://img.shields.io/badge/Architecture-EGM%20Certified-purple?style=flat-square" alt="EGM Certified">
</p>

---

## System Overview

Tsuruyomi (鶴読み) is an ultra-fast, rock-solid native Go replacement for the Java/JVM-based Suwayomi-Server (Tachidesk). Built with [Fiber](https://gofiber.io/) and [pgx](https://github.com/jackc/pgx), it eliminates heavy JVM resource consumption, slow garbage collection spikes, and complex container orchestration.

Tsuruyomi provides complete protocol parity with Suwayomi (`/api/v1/...` REST and `/api/graphql` compatible with Tachidesk-WebUI and Tsumiru), high-speed OPDS 1.2 and OPDS 2.0 catalogs with page streaming (PSE) for e-readers, native Go scrapers backed by a smart fallback failover engine and FlareSolverr Cloudflare bypass, Tachiyomi `.tachibk` Protobuf backup synchronization, an autonomous self-healing engine, and an enterprise resilience framework featuring panic isolation (`SafeGo`), strongly typed domain errors, and structured telemetry (`log/slog`) with automated credential masking.

---

## Local Interface & Invariants

### Local Interface
- `[docker-entrypoint.sh:CONFIG_FILE]`: Environment variable `CONFIG_PATH` determining configuration path (default: `/config/config.yaml`).
- `[docker-entrypoint.sh:DEFAULT_DSN]`: Synthesized PostgreSQL connection string constructed from `DBUSER`, `DBPASS`, `DBHOST`, `DBPORT`, and `DBNAME`.

### Local Invariants
- `[docker-entrypoint.sh:set -e]`: Container initialization halts immediately upon command failure.
- `[docker-entrypoint.sh:config.yaml]`: If `$CONFIG_FILE` does not exist on startup, synthesizes a default YAML template configured from environment variables.
- `[docker-entrypoint.sh:webUI]`: If `/data/webUI` is absent, creates a symbolic link from `/app/data/webUI` to `/data/webUI`.
- `[docker-entrypoint.sh:exec]`: Replaces container shell process by executing `/usr/local/bin/tsuruyomi -config "$CONFIG_FILE"`.

### Local Side Effects
- `[docker-entrypoint.sh:mkdir]`: Creates persistent directories `/config` and `/data`.
- `[docker-entrypoint.sh:cat]`: Writes default YAML configuration to `$CONFIG_FILE`.
- `[docker-entrypoint.sh:ln]`: Creates filesystem symlink `/data/webUI`.

---

## Subdirectories & System Boundaries

- [Command Line Executables (cmd)](./cmd/README.md): Primary server daemon (`cmd/server`) and manga migration utility (`cmd/migrate`).
- [Deployment Configurations (deploy)](./deploy/README.md): Official ProxMenux OCI container templates (`deploy/proxmenux`) and multi-LXC stack definitions.
- [Internal Subsystems (internal)](./internal/README.md): Core private architecture including authentication, configuration, database persistence, domain errors, domain models, HTTP routing and handlers, background services, native sources, common utilities, and WebSocket managers.
- [Maintenance Scripts (scripts)](./scripts/README.md): Administrative automation scripts (`tsuruyomi-heal.sh`) for source integrity audits and repairs.
- [Architectural Design RFCs (docs/designs)](./docs/designs/SITEMAP.md): Approved technical specifications and architectural decision records.

---

## Global Invariants

- **Database Relational Contract**: All persistence relies on PostgreSQL via `pgx/v5`, enforcing schema ownership under `suwayomi.*` (`category`, `manga`, `chapter`, `page`, `mangauser`, `chapteruser`, `serversetting`, `extension`, `source`).
- **Sequential Chronological Chapter Ordering**: Chapter ordering strictly maintains 1-based chronological ordering (`oldest = 1`, `newest = total`), enforced through `EnsureCleanChapterSourceOrders`.
- **Authentication Bypass & Protection**: Authentication behavior is dictated by `config.Auth.Mode` (`none`, `basic`, `simple_login`, `ui_login`). Whitelisted public assets (`auth.IsPublicAsset`), login endpoints, and pre-flight HTTP `OPTIONS` bypass authentication across all routes.
- **Concurrency Serialization**: Chapter CBZ packaging is serialized per chapter ID using `inFlightDownloads` `sync.Map`; library update sweeps are bounded to a single active runner via `um.running`; self-healing routines are serialized via `healerMu`.
- **Panic Resilience & Safe Concurrency**: All asynchronous tasks (download queues, reconcilers, updater passes, healer audits, restore tasks) are strictly dispatched through `util.SafeGo`, intercepting runtime panics, capturing `debug.Stack()` traces, and isolating failures to single worker items without crashing the daemon process.
- **Download Reconciliation & Storage Parity**: Chapter download flags (`is_downloaded`) are verified against physical `.cbz` files on disk via `service.ReconcileDownloads`, ensuring database state never desynchronizes from filesystem reality.
- **Diagnostic Credential Redaction**: All structured log attributes and error messages passed to `log/slog` automatically redact sensitive parameters (`password`, `secret`, `token`, `cookie`, `authorization`) and regex-sanitize PostgreSQL connection DSNs (`postgres://user:pass@host`).
- **HTTP Transport Bounds**: Fiber application enforces a 64MB request body ceiling (`BodyLimit = 64 * 1024 * 1024`) and sets read/write timeouts to 0 to support indefinite WebSocket streaming connections.

---

## Global Side Effects

- **Database Mutations**: Executes SQL transactions mutating `suwayomi.*` tables during source scraping, reading progress tracking, category assignments, and backup restorations.
- **Filesystem Storage**: Stores persistent assets under `cfg.Data.RootDir`: downloaded chapter archives (`.cbz`), temporary reader page caches (`cache/pages/`), cover thumbnails (`downloads/thumbnails/`), dynamic JS extensions, and automated `.tachibk` Protobuf backups.
- **Network Invocations**: Dispatches outbound HTTP requests to scrape external manga providers, connects to FlareSolverr for Cloudflare anti-bot clearance, and establishes optional SOCKS5 proxy tunnels.
- **Real-Time Client Broadcasting**: Emits state transitions over WebSockets to connected browsers and applications for download queue tracking, updater progress, and restore status.

---

## System Verification

```bash
# Execute full test suite with clean cache
go test -count=1 ./...

# Verify zero compilation warnings or errors
go build ./...
```

---

## ⚡ Performance Benchmarks

| Metric | Suwayomi-Server (Java JVM) | **Tsuruyomi (Go)** | Improvement |
| :--- | :--- | :--- | :--- |
| **Idle Memory (RAM)** | ~1,200 MB – 1,800 MB | **~10 MB – 45 MB** | **> 95% less RAM** |
| **Cold Startup Time** | ~22 – 30 seconds | **~45 milliseconds** | **~500x faster** |
| **Library Query (1,500+ series)** | ~1,200 ms (or crashes with SQL 500) | **~25 ms** | **~48x faster** |
| **Backup Restoration (1,000+ manga)** | ~1 – 3 minutes | **~0.10 seconds** | **~1,000x faster** |
| **Source Scraping Concurrency** | JVM thread pool overhead & contention | **Go Singleflight + lightweight goroutines** | **Zero duplicate calls** |
| **Runtime Dependency** | OpenJDK 17/21 JRE (~300MB) | **Zero (Static Native Binary ~26MB)** | **Self-contained** |
| **OPDS Stability** | ❌ SQL GROUP BY bug in Exposed ORM | **✅ Native SQL, zero crashes** | **Rock-solid** |

---

## 🌟 Key Features & Subsystems

### 1. Autonomous Healer Engine
- **Automated Source & Chapter Auditing**: Continuous background auditing and repair engine (`internal/service/healer.go`) that inspects the library for broken chapter links, missing upload timestamps (`date_upload`), corrupted chapter titles, and missing or non-sequential chapter numbers.
- **Background Scheduler & CLI Tool**: Executes an automatic audit pass on startup and runs every 6 hours in the background. Administrators can trigger audits and repairs via REST (`GET /api/v1/healer/audit`, `POST /api/v1/healer/heal`) or automated cron tasks using the standalone [`scripts/tsuruyomi-heal.sh`](./scripts/README.md) script.
- **Multilingual Date & Number Normalization**: Parses complex and localized date formats across sources, including Spanish abbreviated months (`ene`, `feb`, `dic`), relative time strings, and custom source upload representations.

### 2. Next-Gen Reader & UX (Ported from Tsumiru)
- **Smart Orientation Controller**: Intelligent auto-rotation system that dynamically detects manga format vs webtoons:
  - **Manga (Landscape)**: Automatically triggers dual-page side-by-side spread view when device orientation or viewport switches to landscape.
  - **Webtoons & Manhwa**: Actively guards vertical format against accidental rotation or splits, preserving an authentic continuous webtoon scroll experience.
- **Seamless Continuous Horizontal Mode**: Eliminates distracting vertical seams, gaps, and separator lines between consecutive double-page images.
- **Smart Chapter Navigation & Hotkeys**:
  - `N` / `P` keyboard hotkeys to instantly navigate to the Next / Previous chapter; ArrowUp / ArrowDown for smooth reading steps.
  - Auto-hides header and navigation controls upon scrolling down in webtoon mode.
  - Detects missing chapter gaps across unrecognized releases and warns readers of skipped story arcs.
- **Next Chapter Prediction Engine**: Statistical analysis engine calculating the historical release cadence of ongoing series to forecast the probable release date of upcoming chapters.
- **Clean Category Tabs**: Dynamically filters out empty categories in library tabs for an uncluttered reading dashboard.

### 3. Resilience & Production Hardening
- **Panic-Safe Goroutine Dispatcher (`SafeGo`)**: All background workers (download queues, reconcilers, updaters, healer passes, scheduled backups) run encapsulated inside [`internal/util/safego.go`](./internal/util/README.md). Any unexpected panic is intercepted, formatted with complete runtime stack traces (`debug.Stack()`), and emitted via `slog.Error` without crashing the daemon.
- **Strongly Typed Domain Errors (`internal/errors`)**: Pure Go 1.24 error taxonomy defining semantic categories (`KindNotFound`, `KindInvalidInput`, `KindInternal`, `KindUnauthorized`, `KindForbidden`, `KindConflict`, `KindUpstreamFailed`, `KindServiceUnavailable`) with native standard library `errors.Is` and `errors.As` wrapping support.
- **Centralized Error Routing (`CentralErrorHandler`)**: Centrally converts domain error types to standard HTTP REST JSON (`{ "error": "...", "code": "..." }`), GraphQL 200 envelopes with structured error extensions, and plain-text OPDS responses.
- **Structured High-Performance Telemetry (`log/slog`)**: Configurable console or JSON output (`format: "text" | "json"`) with buffered asynchronous writing (250ms periodic flushes) and automated regex-based redaction of database DSN passwords (`postgres://...`) and Bearer tokens.

### 4. High-Performance Source Engine
- **Native Go Scrapers**: High-speed scrapers for top providers (MangaDex, LectorManga, Madara engines, MangaThemesia, ManhwaWeb, RavenManga, LegionScans, Submanhwa, and more) operating directly in compiled Go without JVM overhead.
- **Smart Fallback & Mirror Failover**: Automatic failover engine routing image downloads to secondary CDNs or mirrors when primary endpoints return 403, 404, or rate limits, complete with specialized referrer injection (e.g. Submanhwa CDN referrers).
- **Singleflight Concurrency & Session Reuse**: Utilizes Go's `singleflight.Group` to deduplicate concurrent Cloudflare challenge requests per domain, eliminating redundant FlareSolverr requests and preventing database lock contention.
- **Cloudflare & Anti-Bot Bypass**: Native integration with [FlareSolverr](https://github.com/FlareSolverr/FlareSolverr) with persistent session extraction, cookie management, and automatic invalidation upon challenge rotation.
- **Built-in Goja JavaScript Runtime**: Dynamic JS engine loading and executing community extension manifests without external dependencies, with optional JVM sidecar proxying for legacy extensions.

### 5. Robust Storage & Backup Suite
- **Automated Download Reconciler**: Background scanner ([`internal/service/downloads_reconciler.go`](./internal/service/README.md)) that automatically verifies filesystem `.cbz` archives against database records on startup, restoring orphaned downloads and purging phantom download flags.
- **Ephemeral Tokenized Storage**: Thread-safe ephemeral storage (`GlobalTempFileStorage`) generating short-lived, cryptographically secure UUID tokens for exported `.tachibk` files with automated hourly TTL garbage collection (Suwayomi PR #2509).
- **Sub-Second Tachiyomi Protobuf Backups**: 100% binary compatibility with Tachiyomi / Mihon / Suwayomi `.tachibk` Protobuf format. Deserializes and restores extensive libraries (1,000+ manga) in under **0.10 seconds** compared to 1–3 minutes on JVM.
- **Automated Scheduled Backups**: Configurable wall-clock daily backups with retention rotation.
- **Live WebSocket Progress**: Broadcasts real-time progress percentages over WebSockets during async backup restorations.

### 6. Modern Authentication (`SIMPLE_LOGIN`)
- **JWT & HMAC-SHA256 Session Management**: Modern authentication flow using signed JWT tokens and HTTP-only session cookies.
- **Desktop & Mobile Redirect Protection**: Eliminates protocol-relative redirect loops and SPA page reload bugs across browsers and desktop clients.
- **Clean API Integration**: Suppresses native browser basic-auth credential dialogs on unauthenticated API requests, cleanly returning standard 401 JSON envelopes for modern web applications.

### 7. Webview Reverse Proxy & Diagnostics
- **Server-Side Diagnostic Portal**: Built-in reverse proxy accessible via `/api/v1/webview` and `/api/v1/webview#<url>`.
- **Source Inspection**: Allows developers and administrators to inspect Cloudflare challenge pages, verify session cookies, and debug dynamic webtoon scrapers directly through the server without client-side CORS restrictions.

---

## 📱 Compatible Clients & E-Readers

| Client / Reader | Platform | Connection Mode | Key Capabilities |
| :--- | :--- | :--- | :--- |
| **Tachidesk WebUI** | Web Browser / PWA | Native HTTP / GraphQL | Complete management, reading, settings, and updates. |
| **Tsumiru** | Desktop (Win / Mac / Linux) | GraphQL / REST | Modern native desktop client with high-speed caching. |
| **KOReader** | Kobo, Kindle, Android, Linux | OPDS 1.2 / 2.0 + PSE | Page streaming (PSE), direct CBZ downloads, reading sync. |
| **Moon+ Reader** | Android | OPDS 1.2 Catalog | Direct CBZ acquisition, rich catalog navigation. |
| **Chunky Comic Reader** | iOS / iPadOS | OPDS 1.2 Catalog | Cover previews, OPDS acquisition feeds. |
| **Panels** | iOS / iPadOS | OPDS 1.2 Catalog | Cloud sync and OPDS import. |
| **Mihon / Tachiyomi** | Android | Tachidesk Extension | Native mobile app sync and remote reading. |

---

## 🔧 Configuration Reference

```yaml
# Tsuruyomi Server Configuration (config.yaml)

server:
  # IP address to bind to ("0.0.0.0" for all interfaces, "127.0.0.1" for local only)
  host: "0.0.0.0"
  # Port to listen on (default: 7654)
  port: 7654
  # Subpath prefix (e.g. "/tsuruyomi") — leave empty for root
  subpath: ""

database:
  # PostgreSQL Connection DSN
  dsn: "postgres://tsuruyomi:password@localhost:5432/tsuruyomi?sslmode=disable"
  # Connection pool sizing
  max_conns: 20
  min_conns: 2

data:
  # Root directory for extensions, thumbnails, chapter caches, and WebUI
  root_dir: "./data"

auth:
  # Authentication mode: "none", "basic", or "simple_login"
  mode: "none"
  username: "admin"
  password: "changeme"

extensions:
  # Mihon/Tachiyomi extension repository index
  repo_url: ""

logging:
  # Log level: "debug", "info", "warn", "error"
  level: "info"
  # Log output format: "text" (human readable console) or "json" (structured observability)
  format: "text"

sidecar:
  # Optional JVM Sidecar proxy fallback for unported Java-only extensions
  enabled: false
  base_url: "http://127.0.0.1:4568"

flaresolverr:
  # FlareSolverr integration for bypassing Cloudflare Turnstile / anti-bot challenges
  enabled: false
  url: "http://127.0.0.1:8191"
  timeout: 90
```

---

## 🚀 Quick Start

### 1. Requirements
- **Go 1.24+** (if compiling from source)
- **PostgreSQL 14+** (or existing Suwayomi-Server database)

### 2. Build Binary
```bash
# Clone the repository
git clone https://github.com/devil1210/tsuruyomi.git
cd tsuruyomi

# Build for current host platform
go build -o tsuruyomi ./cmd/server

# Cross-compile for Linux AMD64 (Production / LXC)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o tsuruyomi ./cmd/server
```

### 3. Initialize & Run
```bash
# Copy and adjust configuration
cp config.example.yaml config.yaml

# Run Tsuruyomi daemon
./tsuruyomi -config config.yaml
```

Once running, access:
- **Web Interface**: `http://localhost:7654`
- **OPDS 1.2 Catalog**: `http://localhost:7654/opds` or `http://localhost:7654/api/opds/v1.2`
- **OPDS 2.0 Catalog**: `http://localhost:7654/api/opds/v2.0`
- **Server Diagnostics**: `http://localhost:7654/api/v1/settings/about`
- **Healer Status**: `http://localhost:7654/api/v1/healer/audit`

---

## 🐳 Deployment (Docker & Proxmox LXC)

### Docker Compose
```yaml
services:
  tsuruyomi:
    image: ghcr.io/devil1210/tsuruyomi:latest
    container_name: tsuruyomi
    restart: unless-stopped
    ports:
      - "7654:7654"
    volumes:
      - ./config:/config
      - ./data:/data
    environment:
      SERVER_PORT: 7654
      DATABASE_DSN: postgres://tsuruyomi:secretpassword@postgres:5432/tsuruyomi?sslmode=disable
      TZ: America/Santiago
      FLARESOLVERR_ENABLED: true
      FLARESOLVERR_URL: http://flaresolverr:8191
      LOGGING_FORMAT: text
    depends_on:
      - postgres

  postgres:
    image: postgres:16-alpine
    container_name: tsuruyomi-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: tsuruyomi
      POSTGRES_USER: tsuruyomi
      POSTGRES_PASSWORD: secretpassword
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

### Official ProxMenux LXC Template
Tsuruyomi includes official validated ProxMenux OCI container templates in [`deploy/proxmenux/`](./deploy/proxmenux/README.md):
- **Standalone Mode (`tsuruyomi.json`)**: Deploys a dedicated unprivileged Alpine LXC container connecting to an existing external database (e.g. `postgres-core`).
- **Complete Stack Mode (`tsuruyomi-stack.json`)**: Provisions a coordinated dual-container LXC stack with dedicated PostgreSQL persistence.
- **Update Workflow**: Updates are fully automated via ProxMenux OCI manager, replacing the rootfs from `ghcr.io/devil1210/tsuruyomi:latest` while preserving managed data volumes (`/config`, `/data`).

---

## 📖 OPDS Reader Configuration

To connect e-readers such as **KOReader** or **Moon+ Reader**:

1. Open your reader's OPDS / Net Library catalog section.
2. Add a new catalog:
   - **Name**: `Tsuruyomi`
   - **URL**: `http://<your-server-ip>:7654/opds` (or `http://<your-server-ip>:7654/api/opds/v1.2`)
   - **Username / Password**: *(Leave blank unless Basic Auth is enabled in `config.yaml`)*
3. Save and open the catalog. Enjoy full access to your manga library, categories, search, reading progress tracking, and direct `.cbz` acquisition.

---

## 📄 License

This project is licensed under the **GNU General Public License v3.0 (GPL-3.0)**. See the [LICENSE](LICENSE) file for details.

---

## 🤝 Acknowledgments

- [Suwayomi-Server](https://github.com/Suwayomi/Suwayomi-Server) / Tachidesk for the original specification and GraphQL schema.
- [Tsumiru](https://github.com/Secual/Tsumiru) for innovative client UX paradigms, webtoon protection, and smart orientation logic.
- [Mihon](https://github.com/mihonapp/mihon) & Tachiyomi community for the manga extension ecosystem.
