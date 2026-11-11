# Operational & Maintenance Scripts (`scripts`)

## Overview
The `scripts` package provides operational shell automation utilities for administrative auditing, data integrity verification, and self-healing tasks in Tsuruyomi.

## Interface
- `[tsuruyomi-heal.sh:--audit-only]`: CLI flag passed as first argument (`$1`) to perform a dry-run audit and terminate without triggering repair routines.
- `[tsuruyomi-heal.sh:log]`: Internal helper logging timestamped status lines to standard output and conditionally appending to `LOG_FILE`.
- `[tsuruyomi-heal.sh:SERVER_URL]`: Base server URL variable (default: `http://127.0.0.1:7654`).
- `[tsuruyomi-heal.sh:AUTH_USER]`: HTTP Basic authentication username variable (default: `devil`).
- `[tsuruyomi-heal.sh:AUTH_PASS]`: HTTP Basic authentication password variable.
- `[tsuruyomi-heal.sh:LOG_FILE]`: Destination log file path variable (default: `/var/log/tsuruyomi-healer.log`).

## Invariants
- `[tsuruyomi-heal.sh:set -e]`: Script execution terminates immediately upon any unhandled non-zero command return code.
- `[tsuruyomi-heal.sh:AUDIT_JSON]`: If `$AUDIT_JSON` response is empty due to API unreachable errors, the script logs an error and exits with code 1.
- `[tsuruyomi-heal.sh:--audit-only]`: When `$1` equals `'--audit-only'`, the script exits with status 0 immediately after printing the audit summary.
- `[tsuruyomi-heal.sh:ZERO_DATES]`: If `$ZERO_DATES` equals 0 and `$EMPTY_NAMES` equals 0, the script logs that all sources are healthy and terminates with code 0 without executing a heal pass.

## Side Effects
- `[tsuruyomi-heal.sh:curl]`: Executes HTTP GET with HTTP Basic Auth to `$SERVER_URL/api/v1/system/audit` to inspect database health metrics.
- `[tsuruyomi-heal.sh:curl]`: Executes HTTP POST with HTTP Basic Auth to `$SERVER_URL/api/v1/system/heal` triggering database repairs and source resynchronization.
- `[tsuruyomi-heal.sh:LOG_FILE]`: Appends log entries to `/var/log/tsuruyomi-healer.log` if the destination path has write permissions.
- `[tsuruyomi-heal.sh:echo]`: Emits status reports, metric values, and execution summaries to standard output.

## Verification
- Run: `bash scripts/tsuruyomi-heal.sh --audit-only`
