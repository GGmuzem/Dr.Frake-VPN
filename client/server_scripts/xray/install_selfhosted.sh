#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CONTAINER_NAME="${CONTAINER_NAME:-amnezia-xray}"
IMAGE_NAME="${IMAGE_NAME:-amnezia-xray}"
CONFIG_DIR="${CONFIG_DIR:-/opt/amnezia/xray}"
XRAY_SERVER_PORT="${XRAY_SERVER_PORT:-443}"
XRAY_SITE_NAME="${XRAY_SITE_NAME:-}"
XRAY_XHTTP_PATH="${XRAY_XHTTP_PATH:-/assets/7d91f0e4}"
XRAY_RELEASE="${XRAY_RELEASE:-v25.8.3}"
XRAY_SHORT_IDS_COUNT="${XRAY_SHORT_IDS_COUNT:-8}"
PUBLIC_HOST="${PUBLIC_HOST:-}"
REBUILD_IMAGE=0
FORCE_REGENERATE=0

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --port <port>            Public/server port for VLESS XHTTP/TLS (default: ${XRAY_SERVER_PORT})
  --sni <host>             Required TLS SNI / origin host
  --xhttp-path <path>      XHTTP path (default: ${XRAY_XHTTP_PATH})
  --container <name>       Docker container name (default: ${CONTAINER_NAME})
  --image <name>           Docker image tag (default: ${IMAGE_NAME})
  --config-dir <path>      Host directory for server.json and keys (default: ${CONFIG_DIR})
  --public-host <host>     Public IP or hostname shown in the summary
  --rebuild-image          Force docker build even if image already exists
  --force-regenerate       Rotate UUID/keys/short ID and rewrite config
  --help                   Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)
      XRAY_SERVER_PORT="$2"
      shift 2
      ;;
    --sni)
      XRAY_SITE_NAME="$2"
      shift 2
      ;;
    --xhttp-path)
      XRAY_XHTTP_PATH="$2"
      shift 2
      ;;
    --container)
      CONTAINER_NAME="$2"
      shift 2
      ;;
    --image)
      IMAGE_NAME="$2"
      shift 2
      ;;
    --config-dir)
      CONFIG_DIR="$2"
      shift 2
      ;;
    --public-host)
      PUBLIC_HOST="$2"
      shift 2
      ;;
    --rebuild-image)
      REBUILD_IMAGE=1
      shift
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

if ! [[ "$XRAY_SERVER_PORT" =~ ^[0-9]+$ ]]; then
  echo "XRAY_SERVER_PORT must be numeric" >&2
  exit 1
fi
if [[ -z "$XRAY_SITE_NAME" ]]; then
  echo "--sni is required for self-hosted XRay bootstrap" >&2
  exit 1
fi
if [[ "$XRAY_XHTTP_PATH" != /* ]]; then
  XRAY_XHTTP_PATH="/${XRAY_XHTTP_PATH}"
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

docker_root() {
  if [[ -n "$SUDO" ]]; then
    "$SUDO" docker "$@"
  else
    docker "$@"
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

read_root_file() {
  local path="$1"
  if [[ -n "$SUDO" ]]; then
    "$SUDO" cat "$path"
  else
    cat "$path"
  fi
}

log() {
  printf '[selfhosted-xray] %s\n' "$*"
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Required command not found: $cmd" >&2
    exit 1
  fi
}

require_cmd docker
require_cmd openssl
require_cmd awk
require_cmd sed
require_cmd grep
require_cmd install
require_cmd mktemp

run_root mkdir -p "$CONFIG_DIR"

if [[ "$REBUILD_IMAGE" -eq 1 ]] || [[ -z "$(docker_root images -q "$IMAGE_NAME" 2>/dev/null)" ]]; then
  log "Building docker image ${IMAGE_NAME} from ${SCRIPT_DIR}"
  docker_root build --pull --build-arg "XRAY_RELEASE=${XRAY_RELEASE}" -t "$IMAGE_NAME" "$SCRIPT_DIR"
else
  log "Reusing existing docker image ${IMAGE_NAME}"
fi

generate_uuid() {
  docker_root run --rm --entrypoint xray "$IMAGE_NAME" uuid | tr -d '\r'
}

generate_keypair() {
  docker_root run --rm --entrypoint xray "$IMAGE_NAME" x25519 | tr -d '\r'
}

if [[ "$FORCE_REGENERATE" -eq 1 ]] || ! run_root test -s "$CONFIG_DIR/xray_uuid.key"; then
  XRAY_CLIENT_ID="$(generate_uuid)"
  write_root_file "$CONFIG_DIR/xray_uuid.key" 600 "$XRAY_CLIENT_ID"
else
  XRAY_CLIENT_ID="$(read_root_file "$CONFIG_DIR/xray_uuid.key" | tr -d '\r\n')"
fi

generate_mldsa65() {
  docker_root run --rm --entrypoint xray "$IMAGE_NAME" mldsa65 | tr -d '\r'
}

XRAY_SHORT_IDS_FILE="$CONFIG_DIR/xray_short_ids.txt"
if [[ "$FORCE_REGENERATE" -eq 1 ]] || ! run_root test -s "$XRAY_SHORT_IDS_FILE"; then
  tmp_short_ids="$(mktemp)"
  : > "$tmp_short_ids"
  for _ in $(seq 1 "$XRAY_SHORT_IDS_COUNT"); do
    openssl rand -hex 8 >> "$tmp_short_ids"
  done
  run_root install -m 600 "$tmp_short_ids" "$XRAY_SHORT_IDS_FILE"
  rm -f "$tmp_short_ids"
fi
XRAY_SHORT_ID="$(read_root_file "$XRAY_SHORT_IDS_FILE" | head -n1 | tr -d '\r\n')"
write_root_file "$CONFIG_DIR/xray_short_id.key" 600 "$XRAY_SHORT_ID"
XRAY_SHORT_IDS_JSON_ARRAY="$(read_root_file "$XRAY_SHORT_IDS_FILE" | awk 'BEGIN{first=1; printf "["} {gsub(/[\r\n ]/,""); if(length($0)){if(first==0) printf ","; printf "\"%s\"", $0; first=0}} END{print "]"}')"

if [[ "$FORCE_REGENERATE" -eq 1 ]] || ! run_root test -s "$CONFIG_DIR/xray_private.key" || ! run_root test -s "$CONFIG_DIR/xray_public.key"; then
  KEYPAIR="$(generate_keypair)"
  XRAY_PRIVATE_KEY="$(printf '%s\n' "$KEYPAIR" | awk -F': ' '/Private key:/ {print $2}')"
  XRAY_PUBLIC_KEY="$(printf '%s\n' "$KEYPAIR" | awk -F': ' '/Public key:/ {print $2}')"
  write_root_file "$CONFIG_DIR/xray_private.key" 600 "$XRAY_PRIVATE_KEY"
  write_root_file "$CONFIG_DIR/xray_public.key" 600 "$XRAY_PUBLIC_KEY"
else
  XRAY_PRIVATE_KEY="$(read_root_file "$CONFIG_DIR/xray_private.key" | tr -d '\r\n')"
  XRAY_PUBLIC_KEY="$(read_root_file "$CONFIG_DIR/xray_public.key" | tr -d '\r\n')"
fi

if [[ "$FORCE_REGENERATE" -eq 1 ]] || ! run_root test -s "$CONFIG_DIR/xray_mldsa65_seed.key" || ! run_root test -s "$CONFIG_DIR/xray_mldsa65_verify.key"; then
  MLDSA65_PAIR="$(generate_mldsa65)"
  XRAY_MLDSA65_SEED="$(printf '%s\n' "$MLDSA65_PAIR" | awk -F': ' '/Seed:/ {print $2}')"
  XRAY_MLDSA65_VERIFY="$(printf '%s\n' "$MLDSA65_PAIR" | awk -F': ' '/Verify:/ {print $2}')"
  write_root_file "$CONFIG_DIR/xray_mldsa65_seed.key" 600 "$XRAY_MLDSA65_SEED"
  write_root_file "$CONFIG_DIR/xray_mldsa65_verify.key" 600 "$XRAY_MLDSA65_VERIFY"
else
  XRAY_MLDSA65_SEED="$(read_root_file "$CONFIG_DIR/xray_mldsa65_seed.key" | tr -d '\r\n')"
  XRAY_MLDSA65_VERIFY="$(read_root_file "$CONFIG_DIR/xray_mldsa65_verify.key" | tr -d '\r\n')"
fi

if [[ "$FORCE_REGENERATE" -eq 1 ]] || ! run_root test -s "$CONFIG_DIR/xray_tls.crt" || ! run_root test -s "$CONFIG_DIR/xray_tls.key"; then
  tmp_cert="$(mktemp)"
  tmp_key="$(mktemp)"
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$tmp_key" \
    -out "$tmp_cert" \
    -subj "/CN=${XRAY_SITE_NAME}" \
    -days 3650 >/dev/null 2>&1
  run_root install -m 600 "$tmp_key" "$CONFIG_DIR/xray_tls.key"
  run_root install -m 644 "$tmp_cert" "$CONFIG_DIR/xray_tls.crt"
  rm -f "$tmp_cert" "$tmp_key"
fi

SERVER_JSON="$(cat <<EOF
{
  "log": {
    "loglevel": "warning"
  },
  "inbounds": [
    {
      "listen": "0.0.0.0",
      "port": ${XRAY_SERVER_PORT},
      "protocol": "vless",
      "settings": {
        "clients": [
          {
            "id": "${XRAY_CLIENT_ID}"
          }
        ],
        "decryption": "none"
      },
      "streamSettings": {
        "network": "xhttp",
        "security": "tls",
        "xhttpSettings": {
          "path": "${XRAY_XHTTP_PATH}",
          "mode": "packet-up"
        },
        "tlsSettings": {
          "serverName": "${XRAY_SITE_NAME}",
          "alpn": [
            "h2",
            "http/1.1"
          ],
          "certificates": [
            {
              "certificateFile": "/opt/amnezia/xray/xray_tls.crt",
              "keyFile": "/opt/amnezia/xray/xray_tls.key"
            }
          ]
        }
      },
      "sniffing": {
        "enabled": true,
        "destOverride": [
          "http",
          "tls",
          "quic"
        ]
      }
    }
  ],
  "outbounds": [
    {
      "tag": "direct",
      "protocol": "freedom"
    },
    {
      "tag": "block",
      "protocol": "blackhole"
    }
  ],
  "routing": {
    "rules": [
      {
        "type": "field",
        "ip": [
          "geoip:private"
        ],
        "outboundTag": "block"
      },
      {
        "type": "field",
        "protocol": [
          "bittorrent"
        ],
        "outboundTag": "block"
      }
    ]
  }
}
EOF
)"

write_root_file "$CONFIG_DIR/server.json" 600 "$SERVER_JSON"

if ! docker_root network ls --format '{{.Name}}' | grep -qx 'amnezia-dns-net'; then
  log "Creating optional docker network amnezia-dns-net"
  docker_root network create --driver bridge --subnet=172.29.172.0/24 --opt com.docker.network.bridge.name=amn0 amnezia-dns-net >/dev/null
fi

log "Recreating container ${CONTAINER_NAME}"
docker_root rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker_root run -d \
  --privileged \
  --log-driver none \
  --restart always \
  --cap-add=NET_ADMIN \
  -p "${XRAY_SERVER_PORT}:${XRAY_SERVER_PORT}/tcp" \
  -v "${CONFIG_DIR}:/opt/amnezia/xray" \
  --name "$CONTAINER_NAME" \
  "$IMAGE_NAME" >/dev/null

docker_root network connect amnezia-dns-net "$CONTAINER_NAME" >/dev/null 2>&1 || true

if command -v iptables >/dev/null 2>&1; then
  run_root iptables -C INPUT -p tcp --dport "$XRAY_SERVER_PORT" -j ACCEPT >/dev/null 2>&1 || \
    run_root iptables -I INPUT 1 -p tcp --dport "$XRAY_SERVER_PORT" -j ACCEPT
fi

if command -v ip6tables >/dev/null 2>&1; then
  run_root ip6tables -C INPUT -p tcp --dport "$XRAY_SERVER_PORT" -j ACCEPT >/dev/null 2>&1 || \
    run_root ip6tables -I INPUT 1 -p tcp --dport "$XRAY_SERVER_PORT" -j ACCEPT
fi

if command -v ufw >/dev/null 2>&1; then
  run_root ufw allow "${XRAY_SERVER_PORT}/tcp" >/dev/null 2>&1 || true
fi

if command -v firewall-cmd >/dev/null 2>&1; then
  run_root firewall-cmd --add-port="${XRAY_SERVER_PORT}/tcp" --permanent >/dev/null 2>&1 || true
  run_root firewall-cmd --reload >/dev/null 2>&1 || true
fi

sleep 3

if ! docker_root exec "$CONTAINER_NAME" sh -lc "nc -z 127.0.0.1 ${XRAY_SERVER_PORT}" >/dev/null 2>&1; then
  echo "Container started, but XRay did not open port ${XRAY_SERVER_PORT}" >&2
  docker_root exec "$CONTAINER_NAME" sh -lc "cat /tmp/xray.log 2>/dev/null || true" >&2
  exit 1
fi

if [[ -z "$PUBLIC_HOST" ]] && command -v curl >/dev/null 2>&1; then
  PUBLIC_HOST="$(curl -4fsSL https://api.ipify.org 2>/dev/null || true)"
fi

cat <<EOF

amnezia-xray is ready.

Container:   ${CONTAINER_NAME}
Image:       ${IMAGE_NAME}
Config dir:  ${CONFIG_DIR}
Port:        ${XRAY_SERVER_PORT}
SNI:         ${XRAY_SITE_NAME}
Transport:   VLESS + TLS + XHTTP packet-up
XHTTP path:  ${XRAY_XHTTP_PATH}
Address:     ${PUBLIC_HOST:-<set your server IP or hostname>}
UUID:        ${XRAY_CLIENT_ID}
Short ID:    ${XRAY_SHORT_ID}
Short IDs:   ${XRAY_SHORT_IDS_JSON_ARRAY}
Public key:  ${XRAY_PUBLIC_KEY}
ML-DSA-65:   ${XRAY_MLDSA65_VERIFY}

Quick checks:
  docker ps --format 'table {{.Names}}\t{{.Ports}}\t{{.Status}}'
  docker exec ${CONTAINER_NAME} sh -lc 'nc -z 127.0.0.1 ${XRAY_SERVER_PORT} && echo XRay is listening'
  Test-NetConnection ${PUBLIC_HOST:-<server-ip>} -Port ${XRAY_SERVER_PORT}

Notes:
  - If Test-NetConnection is still false, open ${XRAY_SERVER_PORT}/tcp in the provider firewall.
  - Backend VIP auto-discovery expects:
      ${CONFIG_DIR}/server.json
      ${CONFIG_DIR}/xray_uuid.key
      ${CONFIG_DIR}/xray_short_id.key
      ${CONFIG_DIR}/xray_short_ids.txt
      ${CONFIG_DIR}/xray_public.key
      ${CONFIG_DIR}/xray_private.key
      ${CONFIG_DIR}/xray_tls.crt
      ${CONFIG_DIR}/xray_tls.key
      ${CONFIG_DIR}/xray_mldsa65_seed.key
      ${CONFIG_DIR}/xray_mldsa65_verify.key
EOF
