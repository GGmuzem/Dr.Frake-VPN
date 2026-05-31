package handlers

import (
	"fmt"
	"strings"
	"vpn-backend/internal/models"
)

const (
	defaultSelfHostedXrayPort      = 443
	defaultSelfHostedXrayConfigDir = "/opt/amnezia/xray"
	defaultSelfHostedXrayRelease   = "v26.3.27"
	defaultSelfHostedXrayShortIDs  = 8
	defaultSelfHostedXHTTPPath     = "/video-stream"
)

type selfHostedXrayBootstrapOptions struct {
	ContainerName   string
	ImageName       string
	ConfigDir       string
	Port            int
	SNI             string
	Network         string
	Flow            string
	GrpcServiceName string
	GrpcAuthority   string
	GrpcMultiMode   bool
	RebuildImage    bool
	ForceRegenerate bool
}

type xrayBootstrapResult struct {
	Ran     bool   `json:"ran"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

const selfHostedXrayDockerfile = `FROM alpine:3.15
LABEL maintainer="AmneziaVPN"

RUN apk add --no-cache curl unzip bash openssl netcat-openbsd dumb-init rng-tools xz
RUN apk --update upgrade --no-cache

RUN mkdir -p /opt/amnezia
COPY start.sh /opt/amnezia/start.sh
RUN chmod a+x /opt/amnezia/start.sh

RUN mkdir -p /opt/amnezia/xray

RUN curl -fsSL "https://github.com/XTLS/Xray-core/releases/download/v26.3.27/Xray-linux-64.zip" -o /root/xray.zip && \
  unzip /root/xray.zip -d /usr/bin/ && \
  chmod a+x /usr/bin/xray && \
  rm -f /root/xray.zip

# Tune network
RUN echo -e " \n\
  fs.file-max = 51200 \n\
  \n\
  net.core.rmem_max = 67108864 \n\
  net.core.wmem_max = 67108864 \n\
  net.core.netdev_max_backlog = 250000 \n\
  net.core.somaxconn = 4096 \n\
  net.core.default_qdisc=fq \n\
  \n\
  net.ipv4.tcp_syncookies = 1 \n\
  net.ipv4.tcp_tw_reuse = 1 \n\
  net.ipv4.tcp_tw_recycle = 0 \n\
  net.ipv4.tcp_fin_timeout = 30 \n\
  net.ipv4.tcp_keepalive_time = 1200 \n\
  net.ipv4.ip_local_port_range = 10000 65000 \n\
  net.ipv4.tcp_max_syn_backlog = 8192 \n\
  net.ipv4.tcp_max_tw_buckets = 5000 \n\
  net.ipv4.tcp_fastopen = 3 \n\
  net.ipv4.tcp_mem = 25600 51200 102400 \n\
  net.ipv4.tcp_rmem = 4096 87380 67108864 \n\
  net.ipv4.tcp_wmem = 4096 65536 67108864 \n\
  net.ipv4.tcp_mtu_probing = 1 \n\
  net.ipv4.tcp_congestion_control = bbr \n\
  " | sed -e 's/^\s\+//g' | tee -a /etc/sysctl.conf && \
  mkdir -p /etc/security && \
  echo -e " \n\
  * soft nofile 51200 \n\
  * hard nofile 51200 \n\
  " | sed -e 's/^\s\+//g' | tee -a /etc/security/limits.conf

ENV TZ=Asia/Shanghai

ENTRYPOINT [ "dumb-init", "/opt/amnezia/start.sh" ]
`

const selfHostedXrayStartScript = `#!/bin/bash

echo "Container startup"

XRAY_PORT="$XRAY_SERVER_PORT"
if [ -z "$XRAY_PORT" ] && [ -f /opt/amnezia/xray/server.json ]; then
  XRAY_PORT=$(sed -n 's/.*"port":[[:space:]]*\([0-9]\+\).*/\1/p' /opt/amnezia/xray/server.json | head -n 1)
fi

ensure_iptables_rule() {
  local tool="$1"
  shift
  if command -v "$tool" >/dev/null 2>&1; then
    "$tool" -C INPUT "$@" >/dev/null 2>&1 || "$tool" -A INPUT "$@"
  fi
}

ensure_iptables_rule iptables -i lo -j ACCEPT
ensure_iptables_rule iptables -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
ensure_iptables_rule iptables -p icmp -j ACCEPT
ensure_iptables_rule iptables -p tcp --dport 80 -j ACCEPT
ensure_iptables_rule iptables -p tcp --dport 443 -j ACCEPT
if [ -n "$XRAY_PORT" ]; then ensure_iptables_rule iptables -p tcp --dport "$XRAY_PORT" -j ACCEPT; fi
command -v iptables >/dev/null 2>&1 && iptables -P INPUT DROP

ensure_iptables_rule ip6tables -i lo -j ACCEPT
ensure_iptables_rule ip6tables -m state --state RELATED,ESTABLISHED -j ACCEPT
ensure_iptables_rule ip6tables -p ipv6-icmp -j ACCEPT
if [ -n "$XRAY_PORT" ]; then ensure_iptables_rule ip6tables -p tcp --dport "$XRAY_PORT" -j ACCEPT; fi
command -v ip6tables >/dev/null 2>&1 && ip6tables -P INPUT DROP

killall -KILL xray 2>/dev/null || true

if [ -f /opt/amnezia/xray/server.json ]; then
  exec xray -config /opt/amnezia/xray/server.json
fi

exec tail -f /dev/null
`

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func selfHostedXrayClientFlowLine(opts selfHostedXrayBootstrapOptions) string {
	if strings.TrimSpace(opts.Flow) == "" || opts.Network == "xhttp" {
		return `"id": "${XRAY_CLIENT_ID}"`
	}
	return `"id": "${XRAY_CLIENT_ID}",
            "flow": "${XRAY_CLIENT_FLOW}"`
}

func selfHostedXrayTransportSettings(opts selfHostedXrayBootstrapOptions) string {
	if opts.Network == "xhttp" {
		return `"network": "xhttp",
        "security": "reality",
        "xhttpSettings": {
          "path": "${XRAY_GRPC_SERVICE_NAME}",
          "host": "${XRAY_SITE_NAME}",
          "mode": "auto",
          "xPaddingBytes": "100-1000",
          "scMaxEachPostBytes": 1000000
        },
        "realitySettings": {
          "show": false,
          "dest": "${XRAY_SITE_NAME}:443",
          "xver": 0,
          "serverNames": [
            "${XRAY_SITE_NAME}"
          ],
          "privateKey": "${XRAY_PRIVATE_KEY}",
          "shortIds": ${XRAY_SHORT_IDS_JSON_ARRAY}
        }`
	}
	if opts.Network == xrayNetworkGRPC {
		return `"network": "grpc",
        "security": "reality",
        "grpcSettings": {
          "serviceName": "${XRAY_GRPC_SERVICE_NAME}",
          "authority": "${XRAY_GRPC_AUTHORITY}",
          "multiMode": ${XRAY_GRPC_MULTI_MODE}
        }`
	}
	return `"network": "tcp",
        "security": "reality",
        "tcpSettings": {
          "acceptProxyProtocol": false,
          "header": {
            "type": "none"
          }
        }`
}

func buildSelfHostedXrayBootstrapCommand(server *models.VPNServer, template *models.VLESSServerTemplate, opts selfHostedXrayBootstrapOptions) string {
	return fmt.Sprintf(`set -eu
CONTAINER_NAME=%s
IMAGE_NAME=%s
CONFIG_DIR=%s
XRAY_SERVER_PORT=%d
XRAY_SITE_NAME=%s
XRAY_CLIENT_FLOW=%s
XRAY_GRPC_SERVICE_NAME=%s
XRAY_GRPC_AUTHORITY=%s
XRAY_GRPC_MULTI_MODE=%s
FORCE_REGENERATE=%d
REBUILD_IMAGE=%d
BOOTSTRAP_DIR='/opt/amnezia/xray-bootstrap'

mkdir -p "$CONFIG_DIR" "$BOOTSTRAP_DIR"

cat > "$BOOTSTRAP_DIR/Dockerfile" <<'FBLINK_DOCKERFILE'
%s
FBLINK_DOCKERFILE

cat > "$BOOTSTRAP_DIR/start.sh" <<'FBLINK_START'
%s
FBLINK_START

chmod +x "$BOOTSTRAP_DIR/start.sh"

if [ "$REBUILD_IMAGE" = "1" ] || ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
  docker build --pull -t "$IMAGE_NAME" "$BOOTSTRAP_DIR"
fi

if [ "$FORCE_REGENERATE" = "1" ] || [ ! -s "$CONFIG_DIR/xray_uuid.key" ]; then
  XRAY_CLIENT_ID="$(docker run --rm --entrypoint xray "$IMAGE_NAME" uuid | tr -d '\r')"
  printf '%%s' "$XRAY_CLIENT_ID" > "$CONFIG_DIR/xray_uuid.key"
else
  XRAY_CLIENT_ID="$(tr -d '\r\n' < "$CONFIG_DIR/xray_uuid.key")"
fi

XRAY_SHORT_IDS_FILE="$CONFIG_DIR/xray_short_ids.txt"
if [ "$FORCE_REGENERATE" = "1" ] || [ ! -s "$XRAY_SHORT_IDS_FILE" ]; then
  : > "$XRAY_SHORT_IDS_FILE"
  for _ in $(seq 1 %d); do
    openssl rand -hex 8 >> "$XRAY_SHORT_IDS_FILE"
  done
fi
# Single-id key kept for backward compatibility: first entry of the list
head -n1 "$XRAY_SHORT_IDS_FILE" | tr -d '\r\n' > "$CONFIG_DIR/xray_short_id.key"
XRAY_SHORT_ID="$(tr -d '\r\n' < "$CONFIG_DIR/xray_short_id.key")"
XRAY_SHORT_IDS_JSON_ARRAY="$(awk 'BEGIN{first=1; printf "["} {gsub(/[\r\n ]/,""); if(length($0)){if(first==0) printf ","; printf "\"%%s\"", $0; first=0}} END{print "]"}' "$XRAY_SHORT_IDS_FILE")"

generate_reality_keypair() {
  KEYPAIR="$(docker run --rm --entrypoint xray "$IMAGE_NAME" x25519 | tr -d '\r')"
  XRAY_PRIVATE_KEY="$(printf '%%s\n' "$KEYPAIR" | awk -F': *' 'tolower($1) ~ /private/ {print $2; exit}')"
  XRAY_PUBLIC_KEY="$(printf '%%s\n' "$KEYPAIR" | awk -F': *' 'tolower($1) ~ /public/ {print $2; exit}')"
  if [ -z "$XRAY_PRIVATE_KEY" ] || [ -z "$XRAY_PUBLIC_KEY" ]; then
    printf 'failed to parse xray x25519 output:\n%%s\n' "$KEYPAIR" >&2
    exit 1
  fi
  printf '%%s' "$XRAY_PRIVATE_KEY" > "$CONFIG_DIR/xray_private.key"
  printf '%%s' "$XRAY_PUBLIC_KEY" > "$CONFIG_DIR/xray_public.key"
}

if [ "$FORCE_REGENERATE" = "1" ] || [ ! -s "$CONFIG_DIR/xray_private.key" ] || [ ! -s "$CONFIG_DIR/xray_public.key" ]; then
  generate_reality_keypair
else
  XRAY_PRIVATE_KEY="$(tr -d '\r\n' < "$CONFIG_DIR/xray_private.key")"
  XRAY_PUBLIC_KEY="$(tr -d '\r\n' < "$CONFIG_DIR/xray_public.key")"
  if [ -z "$XRAY_PRIVATE_KEY" ] || [ -z "$XRAY_PUBLIC_KEY" ]; then
    generate_reality_keypair
  fi
fi

XRAY_TLS_CERT_FILE="/opt/amnezia/xray/xray_tls.crt"
XRAY_TLS_KEY_FILE="/opt/amnezia/xray/xray_tls.key"
if [ "$FORCE_REGENERATE" = "1" ] || [ ! -s "$CONFIG_DIR/xray_tls.crt" ] || [ ! -s "$CONFIG_DIR/xray_tls.key" ]; then
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$CONFIG_DIR/xray_tls.key" \
    -out "$CONFIG_DIR/xray_tls.crt" \
    -subj "/CN=${XRAY_SITE_NAME}" \
    -days 3650 >/dev/null 2>&1
fi

cat > "$CONFIG_DIR/server.json" <<EOF
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
        "security": "reality",
        "xhttpSettings": {
          "path": "${XRAY_GRPC_SERVICE_NAME}",
          "host": "${XRAY_SITE_NAME}",
          "mode": "auto",
          "xPaddingBytes": "100-1000",
          "scMaxEachPostBytes": 1000000
        },
        "realitySettings": {
          "show": false,
          "dest": "${XRAY_SITE_NAME}:443",
          "xver": 0,
          "serverNames": [
            "${XRAY_SITE_NAME}"
          ],
          "privateKey": "${XRAY_PRIVATE_KEY}",
          "shortIds": ${XRAY_SHORT_IDS_JSON_ARRAY}
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

if ! docker network ls --format '{{.Name}}' | grep -qx 'amnezia-dns-net'; then
  docker network create --driver bridge --subnet=172.29.172.0/24 --opt com.docker.network.bridge.name=amn0 amnezia-dns-net >/dev/null
fi

docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d \
  --privileged \
  --restart always \
  --cap-add=NET_ADMIN \
  -p "${XRAY_SERVER_PORT}:${XRAY_SERVER_PORT}/tcp" \
  -v "${CONFIG_DIR}:/opt/amnezia/xray" \
  --name "$CONTAINER_NAME" \
  --entrypoint /bin/sh \
  "$IMAGE_NAME" \
  -lc 'exec xray -config /opt/amnezia/xray/server.json' >/dev/null

docker network connect amnezia-dns-net "$CONTAINER_NAME" >/dev/null 2>&1 || true

if command -v iptables >/dev/null 2>&1; then
  iptables -C INPUT -p tcp --dport "$XRAY_SERVER_PORT" -j ACCEPT >/dev/null 2>&1 || iptables -I INPUT 1 -p tcp --dport "$XRAY_SERVER_PORT" -j ACCEPT
fi
if command -v ip6tables >/dev/null 2>&1; then
  ip6tables -C INPUT -p tcp --dport "$XRAY_SERVER_PORT" -j ACCEPT >/dev/null 2>&1 || ip6tables -I INPUT 1 -p tcp --dport "$XRAY_SERVER_PORT" -j ACCEPT
fi
if command -v ufw >/dev/null 2>&1; then
  ufw allow "$XRAY_SERVER_PORT/tcp" >/dev/null 2>&1 || true
fi
if command -v firewall-cmd >/dev/null 2>&1; then
  firewall-cmd --zone=public --add-port="$XRAY_SERVER_PORT/tcp" >/dev/null 2>&1 || true
  firewall-cmd --permanent --zone=public --add-port="$XRAY_SERVER_PORT/tcp" >/dev/null 2>&1 || true
  firewall-cmd --reload >/dev/null 2>&1 || true
fi

READY=0
for _ in 1 2 3 4 5 6 7 8 9 10; do
  if docker exec "$CONTAINER_NAME" sh -lc "nc -z 127.0.0.1 ${XRAY_SERVER_PORT}" >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 1
done

if [ "$READY" != "1" ]; then
  docker ps -a --filter "name=$CONTAINER_NAME" --format 'container_status={{.Status}}'
  docker logs --tail 80 "$CONTAINER_NAME" 2>&1 || true
  exit 1
fi

printf 'container_name=%%s\nport=%%s\nserver_name=%%s\nxhttp_path=%%s\ngrpc_service_name=%%s\ngrpc_authority=%%s\ngrpc_multi_mode=%%s\npublic_key=%%s\nshort_id=%%s\nuuid=%%s\nmldsa65_verify=%%s\nshort_ids_json=%%s\n' \
  "$CONTAINER_NAME" \
  "$XRAY_SERVER_PORT" \
  "$XRAY_SITE_NAME" \
  "$XRAY_GRPC_SERVICE_NAME" \
  "$XRAY_GRPC_SERVICE_NAME" \
  "$XRAY_GRPC_AUTHORITY" \
  "$XRAY_GRPC_MULTI_MODE" \
  "$(tr -d '\r\n' < "$CONFIG_DIR/xray_public.key")" \
  "$(tr -d '\r\n' < "$CONFIG_DIR/xray_short_id.key")" \
  "$(tr -d '\r\n' < "$CONFIG_DIR/xray_uuid.key")" \
  "" \
  "$XRAY_SHORT_IDS_JSON_ARRAY"
`, shellQuote(opts.ContainerName), shellQuote(opts.ImageName), shellQuote(opts.ConfigDir), opts.Port, shellQuote(opts.SNI), shellQuote(opts.Flow), shellQuote(opts.GrpcServiceName), shellQuote(opts.GrpcAuthority), boolJSON(opts.GrpcMultiMode), boolToInt(opts.ForceRegenerate), boolToInt(opts.RebuildImage), selfHostedXrayDockerfile, selfHostedXrayStartScript, defaultSelfHostedXrayShortIDs)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func boolJSON(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func defaultSelfHostedXrayBootstrapOptions(server *models.VPNServer, template *models.VLESSServerTemplate) selfHostedXrayBootstrapOptions {
	opts := selfHostedXrayBootstrapOptions{
		ContainerName:   defaultXrayContainer,
		ImageName:       defaultXrayContainer,
		ConfigDir:       defaultSelfHostedXrayConfigDir,
		Port:            defaultSelfHostedXrayPort,
		Network:         "xhttp",
		Flow:            "",
		GrpcServiceName: "/assets/7d91f0e4",
		GrpcMultiMode:   true,
	}

	if template != nil {
		if value := strings.TrimSpace(template.ContainerName); value != "" {
			opts.ContainerName = value
			opts.ImageName = value
		}
		if template.Port > 0 {
			opts.Port = template.Port
		}
		if value := strings.TrimSpace(template.Network); value != "" {
			opts.Network = value
		}
		if value := strings.TrimSpace(template.Flow); value != "" {
			opts.Flow = value
		}
		if value := strings.TrimSpace(template.ServerName); value != "" {
			opts.SNI = value
		}
		if value := strings.TrimSpace(template.GrpcServiceName); value != "" {
			opts.GrpcServiceName = value
		}
		if value := strings.TrimSpace(template.GrpcAuthority); value != "" {
			opts.GrpcAuthority = value
		}
		opts.GrpcMultiMode = template.GrpcMultiMode
	}
	if strings.TrimSpace(opts.GrpcAuthority) == "" {
		opts.GrpcAuthority = opts.SNI
	}

	if opts.Network == "xhttp" {
		opts.Flow = ""
		if !strings.HasPrefix(opts.GrpcServiceName, "/") {
			opts.GrpcServiceName = "/" + opts.GrpcServiceName
		}
	}

	return opts
}

func mergeFetchedBootstrapTemplate(fetched, requested *models.VLESSServerTemplate) *models.VLESSServerTemplate {
	if fetched == nil {
		return nil
	}
	if requested == nil {
		return fetched
	}

	if value := strings.TrimSpace(requested.Address); value != "" {
		fetched.Address = value
	}
	if value := strings.TrimSpace(requested.Fingerprint); value != "" {
		fetched.Fingerprint = value
	}
	if value := strings.TrimSpace(requested.SpiderX); value != "" {
		fetched.SpiderX = value
	}
	if value := strings.TrimSpace(requested.ContainerName); value != "" {
		fetched.ContainerName = value
	}
	if value := strings.TrimSpace(requested.GrpcServiceName); value != "" {
		fetched.GrpcServiceName = value
	}
	if value := strings.TrimSpace(requested.GrpcAuthority); value != "" {
		fetched.GrpcAuthority = value
	}
	fetched.GrpcMultiMode = requested.GrpcMultiMode
	return fetched
}

func bootstrapSelfHostedXray(server *models.VPNServer, template *models.VLESSServerTemplate, opts selfHostedXrayBootstrapOptions) (*models.VLESSServerTemplate, string, error) {
	if server == nil {
		return nil, "", fmt.Errorf("server is required")
	}
	if strings.TrimSpace(server.SSHPassword) == "" {
		return nil, "", fmt.Errorf("ssh password is required for self-hosted XRay bootstrap")
	}
	if strings.TrimSpace(opts.SNI) == "" {
		return nil, "", fmt.Errorf("vless_server_name is required for self-hosted XRay bootstrap")
	}
	if strings.TrimSpace(opts.GrpcServiceName) == "" {
		opts.GrpcServiceName = defaultXrayGrpcServiceName
	}
	if strings.TrimSpace(opts.GrpcAuthority) == "" {
		opts.GrpcAuthority = opts.SNI
	}

	output, err := sshExec(server, buildSelfHostedXrayBootstrapCommand(server, template, opts))
	if err != nil {
		trimmed := strings.TrimSpace(output)
		if trimmed == "" {
			trimmed = err.Error()
		}
		return nil, output, fmt.Errorf("self-hosted XRay bootstrap failed: %s", trimmed)
	}

	fetchedTemplate, fetchErr := fetchVLESSTemplateFromServer(server, template)
	if fetchErr != nil {
		return nil, output, fetchErr
	}

	mergedTemplate := mergeFetchedBootstrapTemplate(fetchedTemplate, template)

	if template != nil && template.AdvancedJSON != "" {
		container := template.ContainerName
		if container == "" {
			container = defaultXrayContainer
		}
		configPath := detectXrayConfigPath(server, container)
		if raw, err := readXrayFile(server, container, configPath); err == nil {
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
				applyAdvancedJSON(parsed, template.AdvancedJSON, true)
				if updatedJSON, err := json.Marshal(parsed); err == nil {
					_ = writeXrayFile(server, container, configPath, string(updatedJSON))
					_ = ensureXrayRuntimeReady(server, container, configPath, template.Port)
				}
			}
		}
	}

	return mergedTemplate, output, nil
}
