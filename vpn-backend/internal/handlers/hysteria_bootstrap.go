package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"vpn-backend/internal/models"
)

const (
	defaultSelfHostedHysteriaPort          = 443
	defaultSelfHostedHysteriaConfigDir     = "/etc/hysteria"
	defaultSelfHostedHysteriaMasqueradeURL = "https://www.microsoft.com"
)

type selfHostedHysteriaOptions struct {
	Port            int
	SNI             string
	Password        string
	ObfsPassword    string
	MasqueradeURL   string
	ConfigDir       string
	ForceRegenerate bool
}

type hysteriaBootstrapResult struct {
	Ran     bool   `json:"ran"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func parsePositiveInt(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, fmt.Errorf("expected positive integer, got %d", value)
	}
	return value, nil
}

func defaultSelfHostedHysteriaOptions(server *models.VPNServer, template *models.VLESSServerTemplate) selfHostedHysteriaOptions {
	opts := selfHostedHysteriaOptions{
		Port:          defaultSelfHostedHysteriaPort,
		ConfigDir:     defaultSelfHostedHysteriaConfigDir,
		MasqueradeURL: defaultSelfHostedHysteriaMasqueradeURL,
	}
	if template != nil {
		if template.HysteriaPort > 0 {
			opts.Port = template.HysteriaPort
		}
		if value := strings.TrimSpace(template.HysteriaSNI); value != "" {
			opts.SNI = value
		} else if value := strings.TrimSpace(template.ServerName); value != "" {
			opts.SNI = value
		}
		opts.Password = strings.TrimSpace(template.HysteriaPassword)
		opts.ObfsPassword = strings.TrimSpace(template.HysteriaObfsPassword)
		if value := strings.TrimSpace(template.HysteriaMasqueradeURL); value != "" {
			opts.MasqueradeURL = value
		}
	}
	if strings.TrimSpace(opts.SNI) == "" && server != nil {
		opts.SNI = strings.TrimSpace(server.Host)
	}
	return opts
}

func buildSelfHostedHysteriaBootstrapCommand(opts selfHostedHysteriaOptions) string {
	return fmt.Sprintf(`set -eu
HYSTERIA_PORT=%d
HYSTERIA_SNI=%s
HYSTERIA_PASSWORD=%s
HYSTERIA_OBFS_PASSWORD=%s
HYSTERIA_MASQUERADE_URL=%s
CONFIG_DIR=%s
FORCE_REGENERATE=%d
CERT_FILE="$CONFIG_DIR/server.crt"
KEY_FILE="$CONFIG_DIR/server.key"

mkdir -p "$CONFIG_DIR"

if ! command -v hysteria >/dev/null 2>&1; then
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL https://get.hy2.sh/ | bash
  else
    echo "curl is required to install hysteria" >&2
    exit 1
  fi
fi

if [ "$FORCE_REGENERATE" = "1" ] || [ -z "$HYSTERIA_PASSWORD" ]; then
  if [ "$FORCE_REGENERATE" = "1" ] || [ ! -s "$CONFIG_DIR/password.key" ]; then
    HYSTERIA_PASSWORD="$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-32)"
    printf '%%s' "$HYSTERIA_PASSWORD" > "$CONFIG_DIR/password.key"
    chmod 600 "$CONFIG_DIR/password.key"
  else
    HYSTERIA_PASSWORD="$(tr -d '\r\n' < "$CONFIG_DIR/password.key")"
  fi
fi

if [ "$FORCE_REGENERATE" = "1" ] || [ -z "$HYSTERIA_OBFS_PASSWORD" ]; then
  if [ "$FORCE_REGENERATE" = "1" ] || [ ! -s "$CONFIG_DIR/obfs_password.key" ]; then
    HYSTERIA_OBFS_PASSWORD="$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-32)"
    printf '%%s' "$HYSTERIA_OBFS_PASSWORD" > "$CONFIG_DIR/obfs_password.key"
    chmod 600 "$CONFIG_DIR/obfs_password.key"
  else
    HYSTERIA_OBFS_PASSWORD="$(tr -d '\r\n' < "$CONFIG_DIR/obfs_password.key")"
  fi
fi

if [ "$FORCE_REGENERATE" = "1" ] || [ ! -s "$CERT_FILE" ] || [ ! -s "$KEY_FILE" ]; then
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$KEY_FILE" \
    -out "$CERT_FILE" \
    -subj "/CN=${HYSTERIA_SNI}" \
    -days 3650 >/dev/null 2>&1
  chmod 600 "$KEY_FILE"
  chmod 644 "$CERT_FILE"
fi

cat > "$CONFIG_DIR/config.yaml" <<EOF
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
chmod 600 "$CONFIG_DIR/config.yaml"

if command -v iptables >/dev/null 2>&1; then
  iptables -C INPUT -p udp --dport "$HYSTERIA_PORT" -j ACCEPT >/dev/null 2>&1 || iptables -I INPUT 1 -p udp --dport "$HYSTERIA_PORT" -j ACCEPT
fi
if command -v ip6tables >/dev/null 2>&1; then
  ip6tables -C INPUT -p udp --dport "$HYSTERIA_PORT" -j ACCEPT >/dev/null 2>&1 || ip6tables -I INPUT 1 -p udp --dport "$HYSTERIA_PORT" -j ACCEPT
fi
if command -v ufw >/dev/null 2>&1; then
  ufw allow "${HYSTERIA_PORT}/udp" >/dev/null 2>&1 || true
fi
if command -v firewall-cmd >/dev/null 2>&1; then
  firewall-cmd --add-port="${HYSTERIA_PORT}/udp" --permanent >/dev/null 2>&1 || true
  firewall-cmd --reload >/dev/null 2>&1 || true
fi

systemctl enable hysteria-server.service >/dev/null 2>&1 || true
systemctl restart hysteria-server.service

printf 'port=%%s\nsni=%%s\npassword=%%s\nobfs_password=%%s\ninsecure=true\nmasquerade_url=%%s\n' \
  "$HYSTERIA_PORT" \
  "$HYSTERIA_SNI" \
  "$HYSTERIA_PASSWORD" \
  "$HYSTERIA_OBFS_PASSWORD" \
  "$HYSTERIA_MASQUERADE_URL"
`, opts.Port, shellQuote(opts.SNI), shellQuote(opts.Password), shellQuote(opts.ObfsPassword), shellQuote(opts.MasqueradeURL), shellQuote(opts.ConfigDir), boolToInt(opts.ForceRegenerate))
}

func bootstrapSelfHostedHysteria(server *models.VPNServer, template *models.VLESSServerTemplate, opts selfHostedHysteriaOptions) (*models.VLESSServerTemplate, string, error) {
	if server == nil {
		return nil, "", fmt.Errorf("server is required")
	}
	if strings.TrimSpace(server.SSHPassword) == "" {
		return nil, "", fmt.Errorf("ssh password is required for self-hosted Hysteria2 bootstrap")
	}
	if strings.TrimSpace(opts.SNI) == "" {
		return nil, "", fmt.Errorf("hysteria_sni or vless_server_name is required for self-hosted Hysteria2 bootstrap")
	}

	output, err := sshExec(server, buildSelfHostedHysteriaBootstrapCommand(opts))
	if err != nil {
		trimmed := strings.TrimSpace(output)
		if trimmed == "" {
			trimmed = err.Error()
		}
		return nil, output, fmt.Errorf("self-hosted Hysteria2 bootstrap failed: %s", trimmed)
	}

	parsed := parseKeyValueOutput(output)
	if template == nil {
		template = &models.VLESSServerTemplate{}
	}
	next := *template
	next.HysteriaEnabled = true
	next.HysteriaInsecure = parsed["insecure"] != "false"
	if port, err := parsePositiveInt(parsed["port"]); err == nil {
		next.HysteriaPort = port
	}
	if value := strings.TrimSpace(parsed["sni"]); value != "" {
		next.HysteriaSNI = value
	}
	if value := strings.TrimSpace(parsed["password"]); value != "" {
		next.HysteriaPassword = value
	}
	if value := strings.TrimSpace(parsed["obfs_password"]); value != "" {
		next.HysteriaObfsPassword = value
	}
	if value := strings.TrimSpace(parsed["masquerade_url"]); value != "" {
		next.HysteriaMasqueradeURL = value
	}
	return &next, output, nil
}
