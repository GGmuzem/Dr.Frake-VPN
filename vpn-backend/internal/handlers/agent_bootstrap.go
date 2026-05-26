package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/curve25519"
)

type agentBootstrapRequest struct {
	AgentImageDigest string `json:"agent_image_digest" binding:"required"`
	XrayImageDigest  string `json:"xray_image_digest" binding:"required"`
	ManagementPort   int    `json:"management_port"`
	LocalPort        int    `json:"local_port"`
	NodeID           string `json:"node_id"`
	ServerName       string `json:"server_name"`
	RealityDest      string `json:"reality_dest"`
	Force            bool   `json:"force"`
}

type agentBootstrapResult struct {
	NodeID              string `json:"node_id"`
	AgentURL            string `json:"agent_url"`
	ManagementPort      int    `json:"management_port"`
	LocalPort           int    `json:"local_port"`
	ManagementUUID      string `json:"management_uuid"`
	ManagementShortID   string `json:"management_short_id"`
	ManagementPublicKey string `json:"management_public_key"`
	AgentPushPublicKey  string `json:"agent_push_public_key"`
	RemoteOutput        string `json:"remote_output,omitempty"`
	LocalOutput         string `json:"local_output,omitempty"`
}

func (h *AdminHandler) AgentBootstrap(c *gin.Context) {
	var req agentBootstrapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !isImmutableImageDigest(req.AgentImageDigest) || !isImmutableImageDigest(req.XrayImageDigest) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_image_digest and xray_image_digest must be immutable image@sha256 digests"})
		return
	}

	var server models.VPNServer
	if err := h.db.First(&server, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	if strings.TrimSpace(server.SSHPassword) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ssh_password is required for bootstrap"})
		return
	}

	verifyKey, err := agentVerifyPublicKey(h.cfg.AgentSigningPrivateKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AGENT_SIGNING_PRIVATE_KEY is not configured correctly: " + err.Error()})
		return
	}

	result, err := h.bootstrapAgent(&server, req, verifyKey)
	now := time.Now().UTC()
	if err != nil {
		h.db.Model(&server).Updates(map[string]interface{}{
			"agent_bootstrap_status": "failed",
			"agent_bootstrap_error":  err.Error(),
			"agent_bootstrap_at":     &now,
		})
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "result": result})
		return
	}

	updates := map[string]interface{}{
		"agent_url":                   result.AgentURL,
		"agent_node_id":               result.NodeID,
		"agent_bootstrap_status":      "ok",
		"agent_bootstrap_error":       "",
		"agent_bootstrap_at":          &now,
		"agent_management_port":       result.ManagementPort,
		"agent_local_port":            result.LocalPort,
		"agent_management_uuid":       result.ManagementUUID,
		"agent_management_short_id":   result.ManagementShortID,
		"agent_management_public_key": result.ManagementPublicKey,
		"agent_push_public_key":       result.AgentPushPublicKey,
	}
	if err := h.db.Model(&server).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bootstrap succeeded but failed to persist server state: " + err.Error(), "result": result})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "agent bootstrap completed", "result": result})
}

func (h *AdminHandler) bootstrapAgent(server *models.VPNServer, req agentBootstrapRequest, verifyKey string) (agentBootstrapResult, error) {
	nodeID := strings.TrimSpace(req.NodeID)
	if nodeID == "" {
		nodeID = fmt.Sprintf("server-%d", server.ID)
	}
	managementPort := req.ManagementPort
	if managementPort <= 0 {
		managementPort = 39000 + int(server.ID)
	}
	localPort := req.LocalPort
	if localPort <= 0 {
		localPort = 19000 + int(server.ID)
	}
	serverName := strings.TrimSpace(req.ServerName)
	if serverName == "" {
		serverName = "www.microsoft.com"
	}
	realityDest := strings.TrimSpace(req.RealityDest)
	if realityDest == "" {
		realityDest = serverName + ":443"
	}
	managementUUID := uuid.NewString()
	shortID := randomShortID()
	privateKey, publicKey, err := generateXrayRealityKeyPair()
	if err != nil {
		return agentBootstrapResult{}, fmt.Errorf("failed to generate reality keys: %w", err)
	}
	pushPrivateKey, pushPublicKey, err := generateAgentPushKeyPair()
	if err != nil {
		return agentBootstrapResult{}, fmt.Errorf("failed to generate agent push key: %w", err)
	}

	remoteScript := buildRemoteBootstrapScript(remoteBootstrapConfig{
		NodeID:            nodeID,
		AgentImageDigest:  req.AgentImageDigest,
		XrayImageDigest:   req.XrayImageDigest,
		VerifyPublicKey:   verifyKey,
		ManagementPort:    managementPort,
		ManagementUUID:    managementUUID,
		ManagementShortID: shortID,
		ServerName:        serverName,
		RealityDest:       realityDest,
		AWGContainer:      defaultString(server.AWGContainer, "amnezia-awg2"),
		AWGInterface:      defaultString(server.AWGInterface, "awg0"),
		XrayContainer:     defaultXrayContainer,
		RealityPrivateKey: privateKey,
		BackendURL:        strings.TrimRight(h.cfg.PublicBaseURL, "/") + "/api/v1",
		PushPrivateKey:    pushPrivateKey,
	})
	remoteOutput, err := sshExec(server, remoteScript)
	if err != nil {
		return agentBootstrapResult{RemoteOutput: remoteOutput}, fmt.Errorf("remote bootstrap failed: %w: %s", err, remoteOutput)
	}

	localOutput, err := startLocalManagementTunnel(localTunnelConfig{
		ServerID:        server.ID,
		ServerHost:      managementHost(server),
		LocalPort:       localPort,
		ManagementPort:  managementPort,
		ManagementUUID:  managementUUID,
		RealityPublicKey: publicKey,
		ShortID:         shortID,
		ServerName:      serverName,
		XrayImageDigest: req.XrayImageDigest,
	})
	result := agentBootstrapResult{
		NodeID:              nodeID,
		AgentURL:            "http://fblink-node-agent:19090",
		ManagementPort:      managementPort,
		LocalPort:           localPort,
		ManagementUUID:      managementUUID,
		ManagementShortID:   shortID,
		ManagementPublicKey: publicKey,
		AgentPushPublicKey:  pushPublicKey,
		RemoteOutput:        remoteOutput,
		LocalOutput:         localOutput,
	}
	if err != nil {
		return result, fmt.Errorf("local management tunnel failed: %w: %s", err, localOutput)
	}
	if err := probeAgentHealth(result.AgentURL, localPort); err != nil {
		diagnostics := collectAgentTunnelDiagnostics(server, server.ID, localPort)
		return result, fmt.Errorf("%w\n%s", err, diagnostics)
	}
	return result, nil
}

type remoteBootstrapConfig struct {
	NodeID            string
	AgentImageDigest  string
	XrayImageDigest   string
	VerifyPublicKey   string
	ManagementPort    int
	ManagementUUID    string
	ManagementShortID string
	ServerName        string
	RealityDest       string
	AWGContainer      string
	AWGInterface      string
	XrayContainer     string
	RealityPrivateKey string
	BackendURL        string
	PushPrivateKey    string
}

func buildRemoteBootstrapScript(cfg remoteBootstrapConfig) string {
	agentEnv := fmt.Sprintf(`AGENT_ADDR=0.0.0.0:19090
AGENT_NODE_ID=%s
AGENT_VERIFY_PUBLIC_KEY=%s
AGENT_DOCKER_BIN=docker
AGENT_XRAY_CONTAINER=%s
AGENT_AWG_CONTAINER=%s
AGENT_AWG_INTERFACE=%s
AGENT_PIHOLE_CONTAINER=pihole
AGENT_STATE_PATH=/var/lib/fblink-node-agent/update-state.json
AGENT_UPDATE_COMMAND=/opt/fblink-node-agent/fblink-agent-update
AGENT_ROLLBACK_COMMAND=/opt/fblink-node-agent/fblink-agent-update
AGENT_BACKEND_URL=%s
AGENT_PUSH_PRIVATE_KEY=%s
AGENT_PUSH_INTERVAL_SECONDS=60
AGENT_PUSH_SNAPSHOT_SECONDS=600
`, cfg.NodeID, cfg.VerifyPublicKey, cfg.XrayContainer, cfg.AWGContainer, cfg.AWGInterface, cfg.BackendURL, cfg.PushPrivateKey)

	updateScript := `#!/bin/sh
set -eu
test -n "${FBLINK_TARGET_IMAGE_DIGEST:-}"
docker pull "$FBLINK_TARGET_IMAGE_DIGEST"
docker rm -f fblink-node-agent || true
docker network inspect fblink-mgmt >/dev/null 2>&1 || docker network create fblink-mgmt
docker run -d --name fblink-node-agent --restart unless-stopped --network fblink-mgmt \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/fblink-node-agent:/var/lib/fblink-node-agent \
  -v /opt/fblink-node-agent:/opt/fblink-node-agent:ro \
  --env-file /opt/fblink-node-agent/agent.env \
  "$FBLINK_TARGET_IMAGE_DIGEST"
for i in $(seq 1 20); do
  docker exec fblink-node-agent sh -lc 'health_url=http://127.0.0.1:19090/health; if command -v curl >/dev/null 2>&1; then curl -fsS "$health_url"; elif command -v wget >/dev/null 2>&1; then wget -qO- "$health_url"; else grep -qi ":4A92 " /proc/net/tcp /proc/net/tcp6 2>/dev/null; fi' >/dev/null 2>&1 && exit 0
  sleep 1
done
docker logs --tail=100 fblink-node-agent 2>/dev/null || true
exit 1
`

	return fmt.Sprintf(`set -eu
BASE=/opt/fblink-node-agent
mkdir -p "$BASE" /var/lib/fblink-node-agent
docker pull %s
docker pull %s
REALITY_PRIVATE_KEY="%s"
cat > "$BASE/agent.env" <<'FBLINK_AGENT_ENV'
%sFBLINK_AGENT_ENV
cat > "$BASE/fblink-agent-update" <<'FBLINK_UPDATE'
%sFBLINK_UPDATE
chmod 700 "$BASE/fblink-agent-update"
docker network inspect fblink-mgmt >/dev/null 2>&1 || docker network create fblink-mgmt
if command -v iptables >/dev/null 2>&1; then
  iptables -C INPUT -p tcp --dport %d -j ACCEPT 2>/dev/null || iptables -I INPUT 1 -p tcp --dport %d -j ACCEPT
fi
if command -v ip6tables >/dev/null 2>&1; then
  ip6tables -C INPUT -p tcp --dport %d -j ACCEPT 2>/dev/null || ip6tables -I INPUT 1 -p tcp --dport %d -j ACCEPT
fi
cat > "$BASE/mgmt-server.json" <<FBLINK_XRAY_SERVER
{
  "log": {"loglevel": "debug"},
  "inbounds": [{
    "tag": "management-vless",
    "listen": "0.0.0.0",
    "port": %d,
    "protocol": "vless",
    "settings": {
      "decryption": "none",
      "clients": [{"id": "%s"}]
    },
    "streamSettings": {
      "network": "tcp",
      "security": "reality",
      "realitySettings": {
        "show": false,
        "dest": "%s",
        "serverNames": ["%s"],
        "privateKey": "$REALITY_PRIVATE_KEY",
        "shortIds": ["%s"]
      }
    }
  }],
  "outbounds": [{
    "protocol": "freedom",
    "tag": "agent-loopback",
    "settings": {
      "redirect": "fblink-node-agent:19090",
      "finalRules": [{
        "action": "allow",
        "network": "tcp",
        "port": "19090",
        "ip": ["geoip:private"]
      }]
    }
  }],
  "routing": {
    "rules": [{
      "type": "field",
      "inboundTag": ["management-vless"],
      "outboundTag": "agent-loopback"
    }]
  }
}
FBLINK_XRAY_SERVER
docker rm -f fblink-node-agent fblink-mgmt-xray >/dev/null 2>&1 || true
docker run -d --name fblink-node-agent --restart unless-stopped --network fblink-mgmt \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/fblink-node-agent:/var/lib/fblink-node-agent \
  -v /opt/fblink-node-agent:/opt/fblink-node-agent:ro \
  --env-file "$BASE/agent.env" \
  %s
docker run -d --name fblink-mgmt-xray --restart unless-stopped --network fblink-mgmt \
  -p %d:%d \
  --entrypoint /usr/bin/xray \
  -v "$BASE/mgmt-server.json:/etc/xray/config.json:ro" \
  %s run -config /etc/xray/config.json
for i in $(seq 1 15); do
  if docker exec fblink-node-agent sh -lc 'health_url=http://127.0.0.1:19090/health; if command -v curl >/dev/null 2>&1; then curl -fsS "$health_url"; elif command -v wget >/dev/null 2>&1; then wget -qO- "$health_url"; else grep -qi ":4A92 " /proc/net/tcp /proc/net/tcp6 2>/dev/null; fi' >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! docker exec fblink-node-agent sh -lc 'health_url=http://127.0.0.1:19090/health; if command -v curl >/dev/null 2>&1; then curl -fsS "$health_url"; elif command -v wget >/dev/null 2>&1; then wget -qO- "$health_url"; else grep -qi ":4A92 " /proc/net/tcp /proc/net/tcp6 2>/dev/null; fi' >/dev/null; then
  echo "--- Agent Logs ---"
  docker logs --tail 50 fblink-node-agent || true
  echo "--- Xray Logs ---"
  docker logs --tail 50 fblink-mgmt-xray || true
  exit 1
fi
`, shellQuote(cfg.AgentImageDigest), shellQuote(cfg.XrayImageDigest), cfg.RealityPrivateKey, agentEnv, updateScript, cfg.ManagementPort, cfg.ManagementPort, cfg.ManagementPort, cfg.ManagementPort, cfg.ManagementPort, cfg.ManagementUUID, cfg.RealityDest, cfg.ServerName, cfg.ManagementShortID, shellQuote(cfg.AgentImageDigest), cfg.ManagementPort, cfg.ManagementPort, shellQuote(cfg.XrayImageDigest))
}

type localTunnelConfig struct {
	ServerID         uint
	ServerHost       string
	LocalPort        int
	ManagementPort   int
	ManagementUUID   string
	RealityPublicKey string
	ShortID          string
	ServerName       string
	XrayImageDigest  string
}

func startLocalManagementTunnel(cfg localTunnelConfig) (string, error) {
	if err := ensureLocalPortAvailable(cfg.LocalPort); err != nil {
		return "", err
	}
	dir := filepath.Join("data", "agent-tunnels", fmt.Sprintf("server-%d", cfg.ServerID))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	configPath := filepath.Join(dir, "client.json")
	configBody, err := json.MarshalIndent(map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "warning"},
		"inbounds": []map[string]interface{}{{
			"tag":      "agent-http-proxy",
			"listen":   "127.0.0.1",
			"port":     cfg.LocalPort,
			"protocol": "http",
			"settings": map[string]interface{}{},
		}},
		"outbounds": []map[string]interface{}{{
			"tag":      "agent-vless",
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []map[string]interface{}{{
					"address": cfg.ServerHost,
					"port":    cfg.ManagementPort,
					"users": []map[string]interface{}{{
						"id":         cfg.ManagementUUID,
						"encryption": "none",
					}},
				}},
			},
			"streamSettings": map[string]interface{}{
				"network":  "tcp",
				"security": "reality",
				"realitySettings": map[string]interface{}{
					"serverName":  cfg.ServerName,
					"fingerprint": "chrome",
					"publicKey":   cfg.RealityPublicKey,
					"shortId":     cfg.ShortID,
					"spiderX":     "/",
				},
			},
		}},
		"routing": map[string]interface{}{
			"rules": []map[string]interface{}{{
				"type":        "field",
				"inboundTag":  []string{"agent-http-proxy"},
				"outboundTag": "agent-vless",
			}},
		},
	}, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, configBody, 0600); err != nil {
		return "", err
	}

	containerName := fmt.Sprintf("fblink-mgmt-client-%d", cfg.ServerID)
	_ = exec.Command("docker", "rm", "-f", containerName).Run()
	time.Sleep(500 * time.Millisecond)

	cmd := exec.Command("docker", "run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"--network", "container:vpn-backend",
		"-e", "XRAY_CONFIG="+string(configBody),
		"--entrypoint", "/bin/sh",
		cfg.XrayImageDigest,
		"-c", "echo \"$XRAY_CONFIG\" > /etc/xray/config.json && /usr/bin/xray run -config /etc/xray/config.json",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func generateXrayRealityKeyPair() (privateKey, publicKey string, err error) {
	var priv, pub [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return "", "", err
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	curve25519.ScalarBaseMult(&pub, &priv)
	return base64.RawURLEncoding.EncodeToString(priv[:]), base64.RawURLEncoding.EncodeToString(pub[:]), nil
}

func generateAgentPushKeyPair() (privateKey, publicKey string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv), base64.StdEncoding.EncodeToString(pub), nil
}

func probeAgentHealth(agentURL string, proxyPort int) error {
	proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}
	var lastErr error
	for i := 0; i < 20; i++ {
		resp, err := client.Get(agentURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("agent health over management tunnel returned %s", resp.Status)
		} else {
			lastErr = err
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("agent health over management tunnel failed: %w", lastErr)
}

func collectAgentTunnelDiagnostics(server *models.VPNServer, serverID uint, localPort int) string {
	parts := []string{
		"--- local management tunnel diagnostics ---",
		runLocalDiagnostic("docker", "ps", "-a", "--filter", fmt.Sprintf("name=fblink-mgmt-client-%d", serverID), "--format", "table {{.Names}}\t{{.Status}}\t{{.Ports}}"),
		runLocalDiagnostic("docker", "logs", "--tail", "120", fmt.Sprintf("fblink-mgmt-client-%d", serverID)),
		fmt.Sprintf("agent_proxy_url=http://127.0.0.1:%d", localPort),
		"agent_target_url=http://fblink-node-agent:19090/health",
		"--- remote management tunnel diagnostics ---",
	}
	remoteCmd := `printf '%s\n' 'docker containers:'; docker ps -a --filter 'name=fblink' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' || true
printf '%s\n' 'node-agent health:'; docker exec fblink-node-agent sh -lc 'health_url=http://127.0.0.1:19090/health; if command -v curl >/dev/null 2>&1; then curl -fsS "$health_url"; elif command -v wget >/dev/null 2>&1; then wget -qO- "$health_url"; else grep -qi ":4A92 " /proc/net/tcp /proc/net/tcp6 2>/dev/null && printf "agent port is listening\n"; fi' || true
printf '%s\n' 'node-agent raw tcp:'; docker exec fblink-node-agent sh -lc "printf 'GET /health HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n' | timeout 5 nc 127.0.0.1 19090 | head -20" 2>&1 || true
printf '%s\n' 'management config:'; sed -n '1,220p' /opt/fblink-node-agent/mgmt-server.json 2>/dev/null || true
printf '%s\n' 'docker network:'; docker network inspect fblink-mgmt 2>/dev/null || true
printf '%s\n' 'listeners:'; (ss -ltnp 2>/dev/null || netstat -ltnp 2>/dev/null || true) | grep -E '(:19090|:39[0-9]+)' || true
printf '%s\n' 'node-agent logs:'; docker logs --tail 120 fblink-node-agent 2>&1 || true
printf '%s\n' 'mgmt-xray logs:'; docker logs --tail 120 fblink-mgmt-xray 2>&1 || true`
	if out, err := sshExec(server, remoteCmd); err == nil {
		parts = append(parts, out)
	} else {
		parts = append(parts, fmt.Sprintf("remote diagnostics failed: %v\n%s", err, out))
	}
	return strings.Join(parts, "\n")
}

func runLocalDiagnostic(name string, args ...string) string {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("$ %s %s\nerror: %v\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return fmt.Sprintf("$ %s %s\n%s", name, strings.Join(args, " "), strings.TrimSpace(string(out)))
}

func ensureLocalPortAvailable(port int) error {
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("local management port %d is not available: %w", port, err)
	}
	return ln.Close()
}

func managementHost(server *models.VPNServer) string {
	if server.SSHHost != "" {
		host := server.SSHHost
		if h, _, err := net.SplitHostPort(host); err == nil {
			return h
		}
		return host
	}
	host := server.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func isImmutableImageDigest(image string) bool {
	// Relaxed to allow standard tags (e.g., :latest) alongside digests
	return strings.Contains(image, ":") && !strings.ContainsAny(image, " \t\r\n;&|`$()")
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func randomShortID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
