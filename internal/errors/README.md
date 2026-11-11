# Domain Errors (`internal/errors`)

## Overview
The `internal/errors` package establishes a strongly typed, allocation-conscious domain error hierarchy for Tsuruyomi. It provides classification kinds, machine-readable codes, human-readable messages, and root-cause wrapping that integrates seamlessly with Go standard library `errors.Is` and `errors.As` without third-party dependencies.

## Interface
- **Types**:
  - `[errors.go:Kind]`: Semantic classification enum (`KindInternal`, `KindNotFound`, `KindInvalidInput`, `KindUnauthorized`, `KindForbidden`, `KindConflict`, `KindUpstreamFailed`, `KindServiceUnavailable`).
  - `[errors.go:AppError]`: Core domain error struct carrying `Kind`, `Code`, `Message`, and wrapped `Err`.
- **Methods**:
  - `[errors.go:AppError.Error]`: Returns formatted error string `[Code] Message: Err` or `[Code] Message`.
  - `[errors.go:AppError.Unwrap]`: Returns inner cause `Err` for `errors.Unwrap` compatibility.
  - `[errors.go:AppError.Is]`: Implements custom equivalence matching for `errors.Is`.
- **Constructors**:
  - `[errors.go:New]`: Explicit constructor taking `Kind`, code, message, and root cause error.
  - `[errors.go:Internal]`: Helper creating `KindInternal` error.
  - `[errors.go:NotFound]`: Helper creating `KindNotFound` error.
  - `[errors.go:InvalidInput]`: Helper creating `KindInvalidInput` error.
  - `[errors.go:Unauthorized]`: Helper creating `KindUnauthorized` error.
  - `[errors.go:Forbidden]`: Helper creating `KindForbidden` error.
  - `[errors.go:Conflict]`: Helper creating `KindConflict` error.
  - `[errors.go:UpstreamFailed]`: Helper creating `KindUpstreamFailed` error.
  - `[errors.go:ServiceUnavailable]`: Helper creating `KindServiceUnavailable` error.

## Invariants
- `[errors.go:AppError.Is]`: Matches target error if target is `*AppError` with identical `Kind` and target `Code` is either empty or matches exactly.
- `[errors.go:Kind.String]`: Returns non-empty string identifier for every defined `Kind` constant; defaults to `"Unknown"`.

## Side Effects
- Pure in-memory value allocations with zero external I/O or system state modifications.

## Verification
- Run: `go test ./internal/errors/...`
