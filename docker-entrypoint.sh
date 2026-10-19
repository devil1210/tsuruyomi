#!/bin/sh
set -e

CONFIG_FILE="${CONFIG_PATH:-/config/config.yaml}"

mkdir -p /config /data

DBUSER="${DBUSER:-tsuruyomi}"
DBPASS="${DBPASS:-tsuruyomi_pass}"
DBHOST="${DBHOST:-postgres}"
DBPORT="${DBPORT:-5432}"
DBNAME="${DBNAME:-tsuruyomi}"
DEFAULT_DSN="postgres://${DBUSER}:${DBPASS}@${DBHOST}:${DBPORT}/${DBNAME}?sslmode=disable"

# If config.yaml does not exist in the persistent volume, create a default one
if [ ! -f "$CONFIG_FILE" ]; then
    echo "[TSURUYOMI] Initializing default configuration at $CONFIG_FILE..."
    cat <<EOF > "$CONFIG_FILE"
server:
  host: "${SERVER_HOST:-0.0.0.0}"
  port: ${SERVER_PORT:-7654}
  subpath: "${SERVER_SUBPATH:-}"

database:
  dsn: "${DATABASE_DSN:-$DEFAULT_DSN}"
  max_conns: 20
  min_conns: 2

data:
  root_dir: "${DATA_ROOT_DIR:-/data}"

auth:
  mode: "${AUTH_MODE:-none}"
  username: "${AUTH_USERNAME:-}"
  password: "${AUTH_PASSWORD:-}"

extensions:
  repo_url: "${EXTENSIONS_REPO_URL:-}"

logging:
  level: "${LOG_LEVEL:-info}"

sidecar:
  enabled: ${SIDECAR_ENABLED:-false}
  base_url: "${SIDECAR_BASE_URL:-}"

flaresolverr:
  enabled: ${FLARESOLVERR_ENABLED:-false}
  url: "${FLARESOLVERR_URL:-http://flaresolverr:8191}"
  timeout: 90
EOF
fi

# Ensure webUI is accessible inside data directory if data.root_dir is /data
if [ ! -d "/data/webUI" ]; then
    echo "[TSURUYOMI] Linking WebUI assets into /data/webUI..."
    ln -s /app/data/webUI /data/webUI
fi

echo "[TSURUYOMI] Starting Tsuruyomi with config: $CONFIG_FILE..."
exec /usr/local/bin/tsuruyomi -config "$CONFIG_FILE"
