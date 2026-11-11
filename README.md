<p align="center">
  <img src="docs/assets/tsuruyomi-icon.png" width="160" height="160" alt="Tsuruyomi Logo">
</p>

# Tsuruyomi Blueprint

<p align="center">
  <b>Ultra-fast, lightweight native Go backend replacement for Suwayomi-Server (Tachidesk) with OPDS 1.2</b>
</p>

<p align="center">
  <a href="https://github.com/devil1210/tsuruyomi/releases"><img src="https://img.shields.io/badge/version-v1.0.0-rose?style=flat-square" alt="Version"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go" alt="Go"></a>
  <a href="https://github.com/devil1210/tsuruyomi/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-GPL--3.0-blue?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/OPDS-1.2%20%2B%20PSE-emerald?style=flat-square" alt="OPDS 1.2">
  <img src="https://img.shields.io/badge/RAM-~45MB-crimson?style=flat-square" alt="RAM ~45MB">
</p>

---

## System Overview
Tsuruyomi is an ultra-fast, lightweight native Go replacement for the Java/JVM-based Suwayomi-Server (Tachidesk). Built with [Fiber](https://gofiber.io/) and [pgx](https://github.com/jackc/pgx), it provides complete API parity with Suwayomi (`/api/v1/...` REST and `/api/graphql`), native OPDS 1.2 and OPDS 2.0 catalogs with page streaming (PSE) for e-readers, native Go scrapers with FlareSolverr Cloudflare bypass, Tachiyomi `.tachibk` Protobuf backup synchronization, in-memory WebSocket hubs for real-time progress events, and automated library maintenance schedulers.

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
- [Internal Subsystems (internal)](./internal/README.md): Core private architecture including authentication, configuration, database persistence, domain models, HTTP routing and handlers, background services, native sources, and WebSocket managers.
- [Maintenance Scripts (scripts)](./scripts/README.md): Administrative automation scripts (`tsuruyomi-heal.sh`) for source integrity audits and repairs.

---

## Global Invariants
- **Database Relational Contract**: All persistence relies on PostgreSQL via `pgx/v5`, enforcing schema ownership under `suwayomi.*` (`category`, `manga`, `chapter`, `page`, `mangauser`, `chapteruser`, `serversetting`, `extension`, `source`).
- **Sequential Chronological Chapter Ordering**: Chapter ordering strictly maintains 1-based chronological ordering (`oldest = 1`, `newest = total`), enforced through `EnsureCleanChapterSourceOrders`.
- **Authentication Bypass & Protection**: Authentication behavior is dictated by `config.Auth.Mode` (`none`, `basic`, `simple_login`, `ui_login`). Whitelisted public assets (`auth.IsPublicAsset`), login endpoints, and pre-flight HTTP `OPTIONS` bypass authentication across all routes.
- **Concurrency Serialization**: Chapter CBZ packaging is serialized per chapter ID using `inFlightDownloads` `sync.Map`; library update sweeps are bounded to a single active runner via `um.running`; self-healing routines are serialized via `healerMu`.
- **HTTP Transport Bounds**: Fiber application enforces a 64MB request body ceiling (`BodyLimit = 64 * 1024 * 1024`) and sets read/write timeouts to 0 to support indefinite WebSocket streaming connections.

---

## Global Side Effects
- **Database Mutations**: Executes SQL transactions mutating `suwayomi.*` tables during source scraping, reading progress tracking, category assignments, and backup restorations.
- **Filesystem Storage**: Stores persistent assets under `cfg.Data.RootDir`: downloaded chapter archives (`.cbz`), temporary reader page caches (`cache/pages/`), cover thumbnails (`downloads/thumbnails/`), dynamic JS extensions, and automated `.tachibk` Protobuf backups.
- **Network Invocations**: Dispatches outbound HTTP requests to scrape external manga providers, connects to FlareSolverr for Cloudflare anti-bot clearance, and establishes optional SOCKS5 proxy tunnels.
- **Real-Time Client Broadcasting**: Emits state transitions over WebSockets to connected browsers and applications for download queue tracking, updater progress, and restore status.

---

## System Verification
- Run: `go test ./...`

---

## ⚡ Performance Benchmarks

| Metric | Suwayomi-Server (Java JVM) | **Tsuruyomi (Go)** | Improvement |
| :--- | :--- | :--- | :--- |
| **Idle Memory (RAM)** | ~1,200 MB – 1,800 MB | **~35 MB – 50 MB** | **> 95% less RAM** |
| **Cold Startup Time** | ~22 – 30 seconds | **~45 milliseconds** | **~500x faster** |
| **Library Query (1,500+ series)** | ~1,200 ms (or crashes with SQL 500) | **~25 ms** | **~48x faster** |
| **Runtime Dependency** | OpenJDK 17/21 JRE (~300MB) | **Zero (Static Native Binary)** | **Self-contained** |
| **OPDS Stability** | ❌ SQL GROUP BY bug in Exposed ORM | **✅ Native SQL, zero crashes** | **Rock-solid** |

---

## 🌟 Key Features

- **Blazingly Fast & Lightweight**: Written in Go with [Fiber](https://gofiber.io/) and [pgx](https://github.com/jackc/pgx), using under 50 MB of RAM. Perfect for Raspberry Pi, LXC containers, NAS devices, and low-spec VPS.
- **Native OPDS 1.2 Server**:
  - Full navigation feeds: All Series, Categories, Sources, Genres, Publication Statuses, Recent Updates, Reading History.
  - **OPDS Page Streaming (PSE)**: Stream individual chapter pages without downloading the entire CBZ archive.
  - **Open-Access CBZ Acquisition**: Download chapters as `.cbz` directly to e-readers.
  - **Automatic Read Status**: Sync reading progress via `?markAsRead=true` or OPDS-PSE headers.
  - OpenSearch XML discovery description.
- **Full API Compatibility**:
  - `/api/v1/...` REST API (Manga, Chapters, Categories, Downloads, Updates, Backups).
  - `/api/graphql` endpoint for complete compatibility with Tachidesk-WebUI and external clients.
  - Real-time WebSockets for download queue progress and library batch updates.
- **Native Go Scrapers & JavaScript Engine**:
  - Direct high-speed scrapers for popular sources (MangaDex, LectorManga, Madara engines, MangaThemesia).
  - Built-in Goja JavaScript runtime to load community extensions without a JVM.
  - Optional JVM sidecar fallback proxy for unmigrated legacy extensions.
- **Cloudflare Bypass**: Native integration with [FlareSolverr](https://github.com/FlareSolverr/FlareSolverr) to solve Cloudflare Turnstile / anti-bot challenges seamlessly.
- **Bundled WebUI**: Includes the complete, responsive Tachidesk-WebUI preconfigured with the Tsuruyomi Crimson palette (`#E11D48`) and Origami Crane launcher icons.

---

## 📱 Compatible Clients & E-Readers

| Client / Reader | Platform | Connection Mode |
| :--- | :--- | :--- |
| **Tachidesk WebUI** | Web Browser / PWA | Native HTTP / GraphQL |
| **Moon+ Reader** | Android | OPDS Catalog (`http://<ip>:7654/opds`) |
| **KOReader** | Kobo, Kindle, Android, Linux | OPDS Catalog (`http://<ip>:7654/opds`) |
| **Chunky Comic Reader** | iOS / iPadOS | OPDS Catalog (`http://<ip>:7654/opds`) |
| **Panels** | iOS / iPadOS | OPDS Catalog (`http://<ip>:7654/opds`) |
| **Mihon / Tachiyomi** | Android | Tachidesk Extension |

---

## 🚀 Quick Start

### 1. Download or Build from Source

#### Requirements
- Go 1.24+ (if compiling from source)
- PostgreSQL 14+ (or existing Suwayomi database)

#### Build Binary
```bash
# Clone the repository
git clone https://github.com/devil1210/tsuruyomi.git
cd tsuruyomi

# Build for current OS
go build -o tsuruyomi ./cmd/server

# Cross-compile for Linux AMD64
GOOS=linux GOARCH=amd64 go build -o tsuruyomi ./cmd/server
```

### 2. Configuration

Copy `config.example.yaml` to `config.yaml` and configure your database connection:

```bash
cp config.example.yaml config.yaml
```

```yaml
server:
  host: "0.0.0.0"
  port: 7654
  subpath: ""

database:
  dsn: "postgres://username:password@localhost:5432/suwayomi?sslmode=disable"
  max_conns: 20
  min_conns: 2

data:
  root_dir: "./data"

auth:
  mode: "none" # "none", "basic", or "simple_login"

flaresolverr:
  enabled: false
  url: "http://127.0.0.1:8191"
  timeout: 90
```

### 3. Run

```bash
./tsuruyomi -config config.yaml
```

Once running, access:
- **Web Interface**: `http://localhost:7654`
- **OPDS Catalog**: `http://localhost:7654/opds` or `http://localhost:7654/api/opds/v1.2`
- **API Status**: `http://localhost:7654/api/v1/settings/about`

---

## 🐳 Docker Deployment

### Docker Compose

```yaml
version: '3.8'

services:
  tsuruyomi:
    image: tsuruyomi:latest
    container_name: tsuruyomi
    restart: unless-stopped
    ports:
      - "7654:7654"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./data:/app/data
    environment:
      - SUWAYOMI_SERVER_PORT=7654
    depends_on:
      - postgres

  postgres:
    image: postgres:16-alpine
    container_name: tsuruyomi-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: suwayomi
      POSTGRES_USER: tsuruyomi
      POSTGRES_PASSWORD: secretpassword
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

---

## 📖 OPDS Reader Configuration

To connect e-readers such as **KOReader** or **Moon+ Reader**:

1. Open your reader's OPDS / Net Library section.
2. Add a new catalog:
   - **Name**: `Tsuruyomi`
   - **URL**: `http://<your-server-ip>:7654/opds` (or `http://<your-server-ip>:7654/api/opds/v1.2`)
   - **Username / Password**: *(Leave blank unless Basic Auth is enabled in `config.yaml`)*
3. Save and open the catalog. You will have full access to your manga library, categories, search, reading history, and direct CBZ downloading.

---

## 📄 License

This project is licensed under the **GNU General Public License v3.0 (GPL-3.0)**. See the [LICENSE](LICENSE) file for details.

---

## 🤝 Acknowledgments

- [Suwayomi-Server](https://github.com/Suwayomi/Suwayomi-Server) / Tachidesk for the original specification and GraphQL schema.
- [Mihon](https://github.com/mihonapp/mihon) & Tachiyomi community for the extension ecosystem.
