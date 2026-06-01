#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_SCRIPT="$ROOT_DIR/source/fblink-backup.sh"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

write_secret() {
  local path="$1"
  printf '%s\n' "$2" > "$path"
  chmod 600 "$path"
}

write_base_config() {
  local path="$1"
  local repo="$2"
  cat > "$path" <<CONFIG
RESTIC_REPOSITORY=$repo
RESTIC_PASSWORD_FILE=$TMP_DIR/restic-password
RESTIC_REST_USERNAME=fblink-vds
RESTIC_REST_PASSWORD_FILE=$TMP_DIR/rest-server-password
BACKUP_RECEIVER_HOST=srv.frakebit.com
CONFIG
}

assert_success() {
  local name="$1"
  shift
  if ! output="$("$@" 2>&1)"; then
    printf 'not ok - %s\n%s\n' "$name" "$output" >&2
    exit 1
  fi
  printf 'ok - %s\n' "$name"
}

assert_failure_contains() {
  local name="$1"
  local expected="$2"
  shift 2
  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e
  if [[ "$status" -eq 0 || "$output" != *"$expected"* ]]; then
    printf 'not ok - %s\nstatus=%s\nexpected=%s\noutput=%s\n' "$name" "$status" "$expected" "$output" >&2
    exit 1
  fi
  printf 'ok - %s\n' "$name"
}

write_secret "$TMP_DIR/restic-password" "repo-pass"
write_secret "$TMP_DIR/rest-server-password" "receiver-pass"

valid_config="$TMP_DIR/valid.env"
write_base_config "$valid_config" "rest:https://srv.frakebit.com/fblink"
assert_success "accepts srv.frakebit.com over HTTPS" bash "$BACKUP_SCRIPT" --validate-config "$valid_config"

http_config="$TMP_DIR/http.env"
write_base_config "$http_config" "rest:http://srv.frakebit.com/fblink"
assert_failure_contains "rejects insecure HTTP receiver" "must use rest:https://srv.frakebit.com/" bash "$BACKUP_SCRIPT" --validate-config "$http_config"

wrong_host_config="$TMP_DIR/wrong-host.env"
write_base_config "$wrong_host_config" "rest:https://backup.example.com/fblink"
assert_failure_contains "rejects unexpected receiver host" "must use rest:https://srv.frakebit.com/" bash "$BACKUP_SCRIPT" --validate-config "$wrong_host_config"

url_secret_config="$TMP_DIR/url-secret.env"
write_base_config "$url_secret_config" "rest:https://fblink-vds:receiver-pass@srv.frakebit.com/fblink"
assert_failure_contains "rejects credentials in repository URL" "must not contain credentials" bash "$BACKUP_SCRIPT" --validate-config "$url_secret_config"

missing_password_config="$TMP_DIR/missing-password.env"
write_base_config "$missing_password_config" "rest:https://srv.frakebit.com/fblink"
rm -f "$TMP_DIR/rest-server-password"
assert_failure_contains "requires receiver password file" "RESTIC_REST_PASSWORD_FILE is not readable" bash "$BACKUP_SCRIPT" --validate-config "$missing_password_config"
