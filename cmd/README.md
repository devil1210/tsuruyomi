# Command Line Executables (`cmd`)

## Overview
The `cmd` directory contains the application entrypoint packages and binary targets for Tsuruyomi. It groups the primary server daemon and utility CLI tools into dedicated command packages.

## Subdirectories & Boundaries
- [Migration CLI](./migrate/README.md): Standalone CLI tool to migrate library manga records from TuMangaOnline or custom IDs to alternative sources with dry-run support.
- [Server Daemon](./server/README.md): Primary background service daemon hosting the HTTP Fiber web server, OPDS feeds, schedulers, and database persistence layers.
