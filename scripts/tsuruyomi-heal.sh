#!/bin/sh
# ==============================================================================
# Tsuruyomi Autonomous Source Auditor & Self-Healer
# Detects anomalies across all sources (zero upload dates, empty names,
# broken chapter numbers) and fixes them automatically.
# ==============================================================================

set -e

SERVER_URL="http://127.0.0.1:7654"
AUTH_USER="devil"
AUTH_PASS="128710"
LOG_FILE="/var/log/tsuruyomi-healer.log"

log() {
    TIMESTAMP=$(date "+%Y-%m-%d %H:%M:%S")
    echo "[$TIMESTAMP] $1"
    if [ -w "$(dirname "$LOG_FILE")" ]; then
        echo "[$TIMESTAMP] $1" >> "$LOG_FILE"
    fi
}

echo "========================================================"
echo "   TSURUYOMI AUTONOMOUS AUDITOR & SELF-HEALING ENGINE   "
echo "========================================================"

log "Triggering automated source integrity audit..."

# 1. Fetch pre-heal audit
AUDIT_JSON=$(curl -s -u "$AUTH_USER:$AUTH_PASS" "$SERVER_URL/api/v1/system/audit" 2>/dev/null || true)

if [ -z "$AUDIT_JSON" ]; then
    log "Error: Unable to connect to Tsuruyomi API at $SERVER_URL"
    exit 1
fi

ZERO_DATES=$(echo "$AUDIT_JSON" | grep -o '"zeroDateChapters":[0-9]*' | head -n 1 | cut -d':' -f2)
EMPTY_NAMES=$(echo "$AUDIT_JSON" | grep -o '"emptyChapterNames":[0-9]*' | head -n 1 | cut -d':' -f2)
TOTAL_CH=$(echo "$AUDIT_JSON" | grep -o '"totalChapters":[0-9]*' | head -n 1 | cut -d':' -f2)

log "Audit Status: Total chapters=$TOTAL_CH, Zero-date chapters=$ZERO_DATES, Empty names=$EMPTY_NAMES"

if [ "$1" = "--audit-only" ]; then
    echo "Audit Complete (dry-run)."
    exit 0
fi

if [ "$ZERO_DATES" -eq 0 ] && [ "$EMPTY_NAMES" -eq 0 ]; then
    log "All sources and chapters are 100% healthy. No healing needed."
    echo "========================================================"
    echo " Status: ALL SOURCES AND CHAPTERS ARE HEALTHY"
    echo "========================================================"
    exit 0
fi

log "Anomalies detected. Executing self-healing pass..."

# 2. Trigger automated heal
HEAL_RES=$(curl -s -X POST -u "$AUTH_USER:$AUTH_PASS" "$SERVER_URL/api/v1/system/heal" 2>/dev/null || true)

DATES_FIXED=$(echo "$HEAL_RES" | grep -o '"datesRepaired":[0-9]*' | head -n 1 | cut -d':' -f2)
NAMES_FIXED=$(echo "$HEAL_RES" | grep -o '"namesRepaired":[0-9]*' | head -n 1 | cut -d':' -f2)
NATIVE_RESYNCED=$(echo "$HEAL_RES" | grep -o '"nativeMangasResynced":[0-9]*' | head -n 1 | cut -d':' -f2)
DURATION_MS=$(echo "$HEAL_RES" | grep -o '"durationMs":[0-9]*' | head -n 1 | cut -d':' -f2)

log "Self-healing completed in ${DURATION_MS}ms:"
log "  - Native mangas resynced: ${NATIVE_RESYNCED}"
log "  - Upload dates repaired: ${DATES_FIXED}"
log "  - Empty names repaired:   ${NAMES_FIXED}"

echo "========================================================"
echo " RESULTS:"
echo "   Native Mangas Resynced: ${NATIVE_RESYNCED:-0}"
echo "   Upload Dates Repaired:  ${DATES_FIXED:-0}"
echo "   Chapter Names Repaired: ${NAMES_FIXED:-0}"
echo "   Duration:               ${DURATION_MS:-0}ms"
echo "========================================================"

exit 0
