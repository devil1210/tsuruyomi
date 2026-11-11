# Design Documents Sitemap

Index of technical design documents and architectural RFCs for Tsuruyomi.

| Design Document | Date | Author | Status | Components Touched | Description |
| :--- | :--- | :--- | :--- | :--- | :--- |
| [Resilience, Error Standardization, and Structured Diagnostics Framework](./resilience_and_diagnostics_framework.md) | 2026-11-11 | devil1210 | Implemented | `internal/errors`, `internal/util`, `internal/router`, `internal/service`, `cmd/server` | Zero-dependency resilience framework featuring SafeGo panic recovery for workers, typed domain error models, and standard library `log/slog` structured diagnostics with credential masking. |
