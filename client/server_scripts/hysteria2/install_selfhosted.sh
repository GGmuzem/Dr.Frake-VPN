#!/usr/bin/env bash

set -euo pipefail

HYSTERIA_PORT="${HYSTERIA_PORT:-443}"
HYSTERIA_SNI="${HYSTERIA_SNI:-}"
HYSTERIA_PASSWORD="${HYSTERIA_PASSWORD:-}"
HYSTERIA_OBFS_PASSWORD="${HYSTERIA_OBFS_PASSWORD:-}"
HYSTERIA_MASQUERADE_URL="${HYSTERIA_MASQUERADE_URL:-https://www.microsoft.com}"
CONFIG_DIR="${CONFIG_DIR:-/etc/hysteria}"
CERT_FILE="${CERT_FILE:-${CONFIG_DIR}/server.crt}"
KEY_FILE="${KEY_FILE:-${CONFIG_DIR}/server.key}"
FORCE_REGENERATE=0

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --port <port>             UDP port for Hysteria2 (default: ${HYSTERIA_PORT})
  --sni <host>              TLS SNI / certificate CN
  --password <password>     Hysteria2 auth password
  --obfs-password <value>   Optional salamander obfuscation password
  --masquerade-url <url>    HTTP/3 masquerade proxy target
  --config-dir <path>       Config directory (default: ${CONFIG_DIR})
  --force-regenerate        Rotate generated password, obfs password and cert
  --help                    Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)
      HYSTERIA_PORT="$2"
      shift 2
      ;;
    --sni)
      HYSTERIA_SNI="$2"
      shift 2
      ;;
    --password)
      HYSTERIA_PASSWORD="$2"
      shift 2
      ;;
    --obfs-password)
      HYSTERIA_OBFS_PASSWORD="$2"
      shift 2
      ;;
    --masquerade-url)
      HYSTERIA_MASQUERADE_URL="$2"
      shift 2
      ;;
    --config-dir)
      CONFIG_DIR="$2"
      CERT_FILE="${CONFIG_DIR}/server.crt"
      KEY_FILE="${CONFIG_DIR}/server.key"
      shift 2
      ;;
    --force-regenerate)
      FORCE_REGENERATE=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if ! [[ "$HYSTERIA_PORT" =~ ^[0-9]+$ ]]; then
  echo "HYSTERIA_PORT must be numeric" >&2
  exit 1
fi
if [[ -z "$HYSTERIA_SNI" ]]; then
  echo "--sni is required" >&2
  exit 1
fi

if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
  SUDO=""
else
  SUDO="sudo"
fi

run_root() {
  if [[ -n "$SUDO" ]]; then
    "$SUDO" "$@"
  else
    "$@"
  fi
}

read_root_file() {
  local path="$1"
  if [[ -n "$SUDO" ]]; then
    "$SUDO" cat "$path"
  else
    cat "$path"
  fi
}

write_root_file() {
  local path="$1"
  local mode="$2"
  local content="$3"
  local tmp_file
  tmp_file="$(mktemp)"
  printf '%s' "$content" > "$tmp_file"
  run_root install -m "$mode" "$tmp_file" "$path"
  rm -f "$tmp_file"
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Required command not found: $cmd" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd openssl
require_cmd install
require_cmd mktemp

run_root mkdir -p "$CONFIG_DIR"

if ! command -v hysteria >/dev/null 2>&1; then
  curl -fsSL https://get.hy2.sh/ | run_root bash
fi

if [[ "$FORCE_REGENERATE" -eq 1 ]] || [[ -z "$HYSTERIA_PASSWORD" ]]; then
  if ! run_root test -s "$CONFIG_DIR/password.key" || [[ "$FORCE_REGENERATE" -eq 1 ]]; then
    HYSTERIA_PASSWORD="$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-32)"
    write_root_file "$CONFIG_DIR/password.key" 600 "$HYSTERIA_PASSWORD"
  else
    HYSTERIA_PASSWORD="$(read_root_file "$CONFIG_DIR/password.key" | tr -d '\r\n')"
  fi
fi

if [[ "$FORCE_REGENERATE" -eq 1 ]] || [[ -z "$HYSTERIA_OBFS_PASSWORD" ]]; then
  if ! run_root test -s "$CONFIG_DIR/obfs_password.key" || [[ "$FORCE_REGENERATE" -eq 1 ]]; then
    HYSTERIA_OBFS_PASSWORD="$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-32)"
    write_root_file "$CONFIG_DIR/obfs_password.key" 600 "$HYSTERIA_OBFS_PASSWORD"
  else
    HYSTERIA_OBFS_PASSWORD="$(read_root_file "$CONFIG_DIR/obfs_password.key" | tr -d '\r\n')"
  fi
fi

if [[ "$FORCE_REGENERATE" -eq 1 ]] || ! run_root test -s "$CERT_FILE" || ! run_root test -s "$KEY_FILE"; then
  tmp_cert="$(mktemp)"
  tmp_key="$(mktemp)"
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$tmp_key" \
    -out "$tmp_cert" \
    -subj "/CN=${HYSTERIA_SNI}" \
    -days 3650 >/dev/null 2>&1
  run_root install -m 600 "$tmp_key" "$KEY_FILE"
  run_root install -m 644 "$tmp_cert" "$CERT_FILE"
  rm -f "$tmp_cert" "$tmp_key"
fi

CONFIG_BODY="$(cat <<EOF
listen: :${HYSTERIA_PORT}

tls:
  cert: ${CERT_FILE}
  key: ${KEY_FILE}

auth:
  type: password
  password: ${HYSTERIA_PASSWORD}

obfs:
  type: salamander
  salamander:
    password: ${HYSTERIA_OBFS_PASSWORD}

masquerade:
  type: proxy
  proxy:
    url: ${HYSTERIA_MASQUERADE_URL}
    rewriteHost: true
EOF
)"
write_root_file "$CONFIG_DIR/config.yaml" 600 "$CONFIG_BODY"

if command -v iptables >/dev/null 2>&1; then
  run_root iptables -C INPUT -p udp --dport "$HYSTERIA_PORT" -j ACCEPT >/dev/null 2>&1 || \
    run_root iptables -I INPUT 1 -p udp --dport "$HYSTERIA_PORT" -j ACCEPT
fi
if command -v ip6tables >/dev/null 2>&1; then
  run_root ip6tables -C INPUT -p udp --dport "$HYSTERIA_PORT" -j ACCEPT >/dev/null 2>&1 || \
    run_root ip6tables -I INPUT 1 -p udp --dport "$HYSTERIA_PORT" -j ACCEPT
fi
if command -v ufw >/dev/null 2>&1; then
  run_root ufw allow "${HYSTERIA_PORT}/udp" >/dev/null 2>&1 || true
fi
if command -v firewall-cmd >/dev/null 2>&1; then
  run_root firewall-cmd --add-port="${HYSTERIA_PORT}/udp" --permanent >/dev/null 2>&1 || true
  run_root firewall-cmd --reload >/dev/null 2>&1 || true
fi

run_root systemctl enable hysteria-server.service >/dev/null 2>&1 || true
run_root systemctl restart hysteria-server.service

cat <<EOF
hysteria2 is ready.

port=${HYSTERIA_PORT}
sni=${HYSTERIA_SNI}
password=${HYSTERIA_PASSWORD}
obfs_password=${HYSTERIA_OBFS_PASSWORD}
insecure=true

Admin fields:
  Hysteria2 for VIP: enabled
  Hysteria2 UDP port: ${HYSTERIA_PORT}
  Hysteria2 password: ${HYSTERIA_PASSWORD}
  Hysteria2 SNI: ${HYSTERIA_SNI}
  Hysteria2 obfs password: ${HYSTERIA_OBFS_PASSWORD}
  Hysteria2 insecure TLS: enabled

Checks:
  systemctl status hysteria-server.service
  journalctl --no-pager -e -u hysteria-server.service
EOF
