# Internal Subsystems (`internal`)

## Overview
The `internal` directory contains the core application architecture, business logic, persistence services, and protocol handlers for Tsuruyomi. It isolates private implementation packages from external consumers, structuring the server into specialized domain packages.

## Subdirectories & Boundaries
- [Authentication & Security](./auth/README.md): HTTP Basic auth header validation, HMAC-SHA256 session tokens, JWTs, and public route access control.
- [Configuration Management](./config/README.md): Hierarchical YAML and environment variable configuration management via Viper.
- [Database Persistence](./database/README.md): PostgreSQL connection pooling, embedded schema bootstrapping, category management, and native source catalog seeding.
- [Domain Errors](./errors/README.md): Strongly typed domain error taxonomy, semantic kinds, and standard error wrapping.
- [Domain Models](./models/README.md): Data transfer objects (DTOs), enumerations, and JSON/GraphQL serializable entities.
- [HTTP Router & Middleware](./router/README.md): Central Fiber application assembly, security headers, CORS, authentication middleware, and route dispatching.
- [Background Services](./service/README.md): Asynchronous background jobs, Tachiyomi Protobuf backups, download queue worker, library self-healing, and update scheduling.
- [Native Sources & Scrapers](./sources/README.md): High-speed native Go manga scrapers, FlareSolverr Cloudflare bypass, and next chapter release prediction.
- [Common Utilities](./util/README.md): Disk space monitoring, cache cleanup, image resizing, MIME detection, and CBZ ZIP archive indexing.
- [WebSocket Hub](./websocket/README.md): In-memory connection manager and broadcast hub with slow-client eviction for real-time client events.
