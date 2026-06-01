#!/usr/bin/env bash
set -euo pipefail

MODE="run"
CONFIG_FILE="/etc/fblink-backup/backup.env"

case "${1:-}" in
  --validate-config)
    MODE="validate"
    CONFIG_FILE="${2:-$CONFIG_FILE}"
    ;;
  --help|-h)
    cat <<USAGE
Usage:
  fblink-backup [CONFIG_FILE]
  fblink-backup --validate-config [CONFIG_FILE]
USAGE
    exit 0
    ;;
  "")
    ;;
  *)
    CONFIG_FILE="$1"
    ;;
esac

if [[ ! -r "$CONFIG_FILE" ]]; then
  echo "backup config is not readable: $CONFIG_FILE" >&2
  exit 2
fi

set -a
# shellcheck source=/dev/null
. "$CONFIG_FILE"
set +a

: "${RESTIC_REPOSITORY:?RESTIC_REPOSITORY is required}"
: "${RESTIC_PASSWORD_FILE:?RESTIC_PASSWORD_FILE is required}"
: "${BACKEND_CONTAINER:=vpn-backend}"
: "${BACKEND_DB_PATH:=/app/data/vpn.db}"
: "${BACKEND_DOWNLOADS_PATH:=/app/data/downloads}"
: "${COMPOSE_DIR:=/opt/fblink/vpn-backend}"
: "${LOCK_FILE:=/var/lock/fblink-backup.lock}"
: "${WORK_ROOT:=/var/tmp/fblink-backup}"
: "${BACKUP_RECEIVER_HOST:=srv.frakebit.com}"

validate_private_file() {
  local name="$1"
  local path="$2"
  local mode
  if [[ ! -r "$path" ]]; then
    echo "$name is not readable: $path" >&2
    exit 2
  fi

  case "$(uname -s 2>/dev/null || true)" in
    MINGW*|MSYS*|CYGWIN*) return ;;
  esac

  if mode="$(stat -c '%a' "$path" 2>/dev/null)"; then
    mode=$((8#$mode))
    if (( mode & 077 )); then
      echo "$name must not be group/world readable: $path" >&2
      exit 2
    fi
  fi
}

validate_backup_config() {
  local authority expected_prefix
  if [[ "$RESTIC_REPOSITORY" =~ ^rest:https?://([^/]+) ]]; then
    authority="${BASH_REMATCH[1]}"
    if [[ "$authority" == *"@"* ]]; then
      echo "RESTIC_REPOSITORY must not contain credentials; use RESTIC_REST_USERNAME and RESTIC_REST_PASSWORD_FILE" >&2
      exit 2
    fi
  fi

  expected_prefix="rest:https://${BACKUP_RECEIVER_HOST}/"
  if [[ "$RESTIC_REPOSITORY" != "$expected_prefix"* ]]; then
    echo "RESTIC_REPOSITORY must use ${expected_prefix}" >&2
    exit 2
  fi

  if [[ -z "${RESTIC_REST_USERNAME:-}" ]]; then
    echo "RESTIC_REST_USERNAME is required for the backup receiver" >&2
    exit 2
  fi
  if [[ -z "${RESTIC_REST_PASSWORD_FILE:-}" ]]; then
    echo "RESTIC_REST_PASSWORD_FILE is required for the backup receiver" >&2
    exit 2
  fi

  validate_private_file RESTIC_PASSWORD_FILE "$RESTIC_PASSWORD_FILE"
  validate_private_file RESTIC_REST_PASSWORD_FILE "$RESTIC_REST_PASSWORD_FILE"
}

validate_backup_config

if [[ "$MODE" == "validate" ]]; then
  echo "backup config ok: $RESTIC_REPOSITORY"
  exit 0
fi

if [[ -n "${RESTIC_REST_PASSWORD_FILE:-}" ]]; then
  export RESTIC_REST_PASSWORD
  RESTIC_REST_PASSWORD="$(tr -d '\r\n' < "$RESTIC_REST_PASSWORD_FILE")"
fi

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 2
  fi
}

require_cmd docker
require_cmd flock
require_cmd restic
require_cmd sqlite3

umask 077
mkdir -p "$(dirname "$LOCK_FILE")" "$WORK_ROOT"
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  echo "another fblink backup is already running" >&2
  exit 1
fi

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
WORK_DIR="$(mktemp -d "$WORK_ROOT/$RUN_ID.XXXXXX")"
SNAPSHOT_DIR="$WORK_DIR/snapshot"
mkdir -p "$SNAPSHOT_DIR/backend-data" "$SNAPSHOT_DIR/config"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

echo "creating consistent SQLite snapshot"
docker exec "$BACKEND_CONTAINER" sh -c "sqlite3 '$BACKEND_DB_PATH' \".backup '/tmp/fblink-vpn.db'\""
docker cp "$BACKEND_CONTAINER:/tmp/fblink-vpn.db" "$SNAPSHOT_DIR/backend-data/vpn.db"
docker exec "$BACKEND_CONTAINER" rm -f /tmp/fblink-vpn.db

DB_CHECK="$(sqlite3 "$SNAPSHOT_DIR/backend-data/vpn.db" "PRAGMA integrity_check;")"
if [[ "$DB_CHECK" != "ok" ]]; then
  echo "SQLite integrity check failed: $DB_CHECK" >&2
  exit 1
fi

if docker exec "$BACKEND_CONTAINER" test -d "$BACKEND_DOWNLOADS_PATH"; then
  echo "copying backend downloads"
  docker cp "$BACKEND_CONTAINER:$BACKEND_DOWNLOADS_PATH" "$SNAPSHOT_DIR/backend-data/downloads"
fi

if [[ -d "$COMPOSE_DIR" ]]; then
  echo "copying compose configuration"
  for name in .env docker-compose.yml Dockerfile nginx.conf; do
    if [[ -e "$COMPOSE_DIR/$name" ]]; then
      cp -a "$COMPOSE_DIR/$name" "$SNAPSHOT_DIR/config/"
    fi
  done
fi

if [[ -n "${CADDYFILE_PATH:-}" && -e "$CADDYFILE_PATH" ]]; then
  cp -a "$CADDYFILE_PATH" "$SNAPSHOT_DIR/config/Caddyfile"
fi

if [[ -n "${EXTRA_PATHS:-}" ]]; then
  mkdir -p "$SNAPSHOT_DIR/extra"
  for path in $EXTRA_PATHS; do
    if [[ -e "$path" ]]; then
      cp -a "$path" "$SNAPSHOT_DIR/extra/"
    fi
  done
fi

cat > "$SNAPSHOT_DIR/manifest.txt" <<MANIFEST
created_utc=$RUN_ID
source_host=$(hostname -f 2>/dev/null || hostname)
backend_container=$BACKEND_CONTAINER
backend_db_path=$BACKEND_DB_PATH
compose_dir=$COMPOSE_DIR
MANIFEST

if ! restic snapshots >/dev/null 2>&1; then
  echo "initializing restic repository"
  restic init
fi

echo "sending encrypted restic backup"
(
  cd "$WORK_DIR"
  restic backup snapshot --tag fblink-vpn --tag daily
)

echo "checking restic repository metadata"
restic check

echo "backup complete: $RUN_ID"
