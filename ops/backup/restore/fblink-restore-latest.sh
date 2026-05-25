#!/usr/bin/env bash
set -euo pipefail

CONFIG_FILE="${FBLINK_BACKUP_CONFIG:-/etc/fblink-backup/backup.env}"
TARGET_DIR="${1:-/tmp/fblink-restore-$(date -u +%Y%m%dT%H%M%SZ)}"

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

if [[ -n "${RESTIC_REST_PASSWORD_FILE:-}" ]]; then
  export RESTIC_REST_PASSWORD
  RESTIC_REST_PASSWORD="$(tr -d '\r\n' < "$RESTIC_REST_PASSWORD_FILE")"
fi

mkdir -p "$TARGET_DIR"
restic restore latest --target "$TARGET_DIR" --tag fblink-vpn

DB_PATH="$(find "$TARGET_DIR" -path '*/snapshot/backend-data/vpn.db' -type f | head -n 1)"
if [[ -z "$DB_PATH" ]]; then
  echo "restored SQLite database was not found under $TARGET_DIR" >&2
  exit 1
fi

if command -v sqlite3 >/dev/null 2>&1; then
  echo "running SQLite integrity check"
  sqlite3 "$DB_PATH" "PRAGMA integrity_check;"
else
  echo "sqlite3 is not installed; skipped integrity check"
fi

echo "restore complete: $TARGET_DIR"
