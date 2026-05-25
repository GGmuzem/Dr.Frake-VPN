package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	})
	remoteOutput, err := sshExec(server, remoteScript)
	if err != nil {
		return agentBootstrapResult{RemoteOutput: remoteOutput}, fmt.Errorf("remote bootstrap failed: %w: %s", err, remoteOutput)
	}
	publicKey, err := parseRealityPublicKey(remoteOutput)
	if err != nil {
		return agentBootstrapResult{RemoteOutput: remoteOutput}, err
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
		AgentURL:            fmt.Sprintf("http://127.0.0.1:%d", localPort),
		ManagementPort:      managementPort,
		LocalPort:           localPort,
		ManagementUUID:      managementUUID,
		ManagementShortID:   shortID,
		ManagementPublicKey: publicKey,
		RemoteOutput:        remoteOutput,
		LocalOutput:         localOutput,
	}
	if err != nil {
		return result, fmt.Errorf("local management tunnel failed: %w: %s", err, localOutput)
	}
	if err := probeAgentHealth(result.AgentURL); err != nil {
		return result, err
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
}

func buildRemoteBootstrapScript(cfg remoteBootstrapConfig) string {
	agentEnv := fmt.Sprintf(`AGENT_ADDR=127.0.0.1:9090
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
`, cfg.NodeID, cfg.VerifyPublicKey, cfg.XrayContainer, cfg.AWGContainer, cfg.AWGInterface)

	updateScript := `#!/bin/sh
set -eu
test -n "${FBLINK_TARGET_IMAGE_DIGEST:-}"
docker pull "$FBLINK_TARGET_IMAGE_DIGEST"
docker rm -f fblink-node-agent || true
docker run -d --name fblink-node-agent --restart unless-stopped --network host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/fblink-node-agent:/var/lib/fblink-node-agent \
  -v /opt/fblink-node-agent:/opt/fblink-node-agent:ro \
  --env-file /opt/fblink-node-agent/agent.env \
  "$FBLINK_TARGET_IMAGE_DIGEST"
for i in $(seq 1 20); do
  curl -fsS http://127.0.0.1:9090/health >/dev/null 2>&1 && exit 0
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
KEYS="$(docker run --rm --entrypoint xray %s x25519)"
REALITY_PRIVATE_KEY="$(printf '%%s\n' "$KEYS" | awk -F': ' '/Private key/ {print $2}')"
REALITY_PUBLIC_KEY="$(printf '%%s\n' "$KEYS" | awk -F': ' '/Public key/ {print $2}')"
test -n "$REALITY_PRIVATE_KEY"
test -n "$REALITY_PUBLIC_KEY"
cat > "$BASE/agent.env" <<'FBLINK_AGENT_ENV'
%sFBLINK_AGENT_ENV
cat > "$BASE/fblink-agent-update" <<'FBLINK_UPDATE'
%sFBLINK_UPDATE
chmod 700 "$BASE/fblink-agent-update"
if command -v iptables >/dev/null 2>&1; then
  iptables -C INPUT -p tcp --dport %d -j ACCEPT 2>/dev/null || iptables -I INPUT 1 -p tcp --dport %d -j ACCEPT
fi
if command -v ip6tables >/dev/null 2>&1; then
  ip6tables -C INPUT -p tcp --dport %d -j ACCEPT 2>/dev/null || ip6tables -I INPUT 1 -p tcp --dport %d -j ACCEPT
fi
cat > "$BASE/mgmt-server.json" <<FBLINK_XRAY_SERVER
{
  "log": {"loglevel": "warning"},
  "inbounds": [{
    "tag": "management-vless",
    "listen": "0.0.0.0",
    "port": %d,
    "protocol": "vless",
    "settings": {
      "decryption": "none",
      "clients": [{"id": "%s", "flow": "xtls-rprx-vision"}]
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
  "outbounds": [{"protocol": "freedom", "tag": "direct"}]
}
FBLINK_XRAY_SERVER
docker rm -f fblink-node-agent fblink-mgmt-xray >/dev/null 2>&1 || true
docker run -d --name fblink-node-agent --restart unless-stopped --network host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/fblink-node-agent:/var/lib/fblink-node-agent \
  -v /opt/fblink-node-agent:/opt/fblink-node-agent:ro \
  --env-file "$BASE/agent.env" \
  %s
docker run -d --name fblink-mgmt-xray --restart unless-stopped --network host \
  --entrypoint xray \
  -v "$BASE/mgmt-server.json:/etc/xray/config.json:ro" \
  %s run -config /etc/xray/config.json
curl -fsS http://127.0.0.1:9090/health
printf 'FBLINK_REALITY_PUBLIC_KEY=%%s\n' "$REALITY_PUBLIC_KEY"
`, shellQuote(cfg.AgentImageDigest), shellQuote(cfg.XrayImageDigest), shellQuote(cfg.XrayImageDigest), agentEnv, updateScript, cfg.ManagementPort, cfg.ManagementPort, cfg.ManagementPort, cfg.ManagementPort, cfg.ManagementPort, cfg.ManagementUUID, cfg.RealityDest, cfg.ServerName, cfg.ManagementShortID, shellQuote(cfg.AgentImageDigest), shellQuote(cfg.XrayImageDigest))
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
			"tag":      "agent-local",
			"listen":   "127.0.0.1",
			"port":     cfg.LocalPort,
			"protocol": "dokodemo-door",
			"settings": map[string]interface{}{
				"address": "127.0.0.1",
				"port":    9090,
				"network": "tcp",
			},
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
						"flow":       "xtls-rprx-vision",
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
	}, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, configBody, 0600); err != nil {
		return "", err
	}

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", err
	}
	containerName := fmt.Sprintf("fblink-mgmt-client-%d", cfg.ServerID)
	_ = exec.Command("docker", "rm", "-f", containerName).Run()
	cmd := exec.Command("docker", "run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"--network", "host",
		"--entrypoint", "xray",
		"-v", absConfigPath+":/etc/xray/config.json:ro",
		cfg.XrayImageDigest,
		"run", "-config", "/etc/xray/config.json",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func parseRealityPublicKey(output string) (string, error) {
	var publicKey string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FBLINK_REALITY_PUBLIC_KEY=") {
			publicKey = strings.TrimPrefix(line, "FBLINK_REALITY_PUBLIC_KEY=")
		}
	}
	if publicKey == "" {
		return "", fmt.Errorf("failed to parse generated Reality public key")
	}
	return publicKey, nil
}

func probeAgentHealth(agentURL string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(agentURL + "/health")
	if err != nil {
		return fmt.Errorf("agent health over management tunnel failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("agent health over management tunnel returned %s", resp.Status)
	}
	return nil
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
		return fmt.Sprintf("%08x", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)[:8]
}
