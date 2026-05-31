package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func serverAgentMode(server models.VPNServer) string {
	if strings.TrimSpace(server.AgentURL) != "" {
		return "agent"
	}
	if strings.TrimSpace(server.SSHPassword) != "" {
		return "ssh fallback"
	}
	return "manual"
}

func (h *AdminHandler) AgentSnapshot(c *gin.Context) {
	server, client, ok := h.loadAgentClient(c)
	if !ok {
		return
	}

	snapshot, err := client.Snapshot()
	if err != nil {
		now := time.Now().UTC()
		h.db.Model(&server).Updates(map[string]interface{}{
			"agent_last_snapshot_at":     &now,
			"agent_last_snapshot_status": "failed: " + err.Error(),
		})
		auditServerAction(h.db, c, "agent.snapshot", server.ID, "failed", err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	if err := applySnapshotToServer(h.db, &server, snapshot.Files); err != nil {
		now := time.Now().UTC()
		h.db.Model(&server).Updates(map[string]interface{}{
			"agent_last_snapshot_at":     &now,
			"agent_last_snapshot_hash":   snapshot.ContentHash,
			"agent_last_snapshot_status": "parse_failed: " + err.Error(),
		})
		auditServerAction(h.db, c, "agent.snapshot", server.ID, "failed", err.Error())
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	now := time.Now().UTC()
	server.AgentLastSnapshotHash = snapshot.ContentHash
	server.AgentLastSnapshotAt = &now
	server.AgentLastSnapshotStatus = "ok"
	h.db.Save(&server)
	auditServerAction(h.db, c, "agent.snapshot", server.ID, "ok", snapshot.ContentHash)

	c.JSON(http.StatusOK, gin.H{
		"message":       "snapshot imported",
		"snapshot_hash": snapshot.ContentHash,
		"agent_mode":    serverAgentMode(server),
	})
}

func (h *AdminHandler) AgentUpdate(c *gin.Context) {
	server, client, ok := h.loadAgentClient(c)
	if !ok {
		return
	}
	var req struct {
		ImageDigest string `json:"image_digest" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	state, err := client.Update(server.ID, req.ImageDigest)
	h.persistAgentUpdateState(&server, state, err)
	if err != nil {
		auditServerAction(h.db, c, "agent.update", server.ID, "failed", err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "state": state})
		return
	}
	auditServerAction(h.db, c, "agent.update", server.ID, "ok", req.ImageDigest)
	c.JSON(http.StatusOK, gin.H{"state": state})
}

func (h *AdminHandler) AgentRollback(c *gin.Context) {
	server, client, ok := h.loadAgentClient(c)
	if !ok {
		return
	}
	state, err := client.Rollback()
	h.persistAgentUpdateState(&server, state, err)
	if err != nil {
		auditServerAction(h.db, c, "agent.rollback", server.ID, "failed", err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "state": state})
		return
	}
	auditServerAction(h.db, c, "agent.rollback", server.ID, "ok", state.ActiveDigest)
	c.JSON(http.StatusOK, gin.H{"state": state})
}

func (h *AdminHandler) AgentStatus(c *gin.Context) {
	server, client, ok := h.loadAgentClient(c)
	if !ok {
		return
	}
	health, healthErr := client.Health()
	state, statusErr := client.Status()
	h.persistAgentHealth(&server, health, healthErr)
	h.persistAgentUpdateState(&server, state, statusErr)
	if healthErr != nil || statusErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"health_error": fmt.Sprint(healthErr),
			"status_error": fmt.Sprint(statusErr),
			"agent_mode":   serverAgentMode(server),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"health":     health,
		"state":      state,
		"agent_mode": serverAgentMode(server),
	})
}

func (h *AdminHandler) loadAgentClient(c *gin.Context) (models.VPNServer, *nodeAgentClient, bool) {
	id := c.Param("id")
	var server models.VPNServer
	if err := h.db.First(&server, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return server, nil, false
	}
	proxyURL := ""
	if server.AgentLocalPort > 0 {
		proxyURL = fmt.Sprintf("http://127.0.0.1:%d", server.AgentLocalPort)
	}
	client, err := newNodeAgentClientWithProxy(server.AgentURL, h.cfg.AgentSigningPrivateKey, proxyURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "agent_mode": serverAgentMode(server)})
		return server, nil, false
	}
	return server, client, true
}

func (h *AdminHandler) persistAgentHealth(server *models.VPNServer, health nodeAgentHealth, callErr error) {
	updates := map[string]interface{}{}
	if callErr != nil {
		updates["agent_last_update_error"] = callErr.Error()
		updates["agent_last_health_status"] = "failed"
	} else {
		now := time.Now().UTC()
		updates["agent_last_heartbeat_at"] = &now
		updates["agent_docker_available"] = health.DockerAvailable
		updates["agent_uptime_seconds"] = health.UptimeSeconds
		updates["agent_last_health_status"] = "ok"
		updates["agent_last_version"] = health.Version
		updates["agent_last_commit"] = health.Commit
		if health.NodeID != "" {
			updates["agent_node_id"] = health.NodeID
		}
	}
	if len(updates) > 0 {
		h.db.Model(server).Updates(updates)
	}
}

func (h *AdminHandler) persistAgentUpdateState(server *models.VPNServer, state nodeAgentUpdateState, callErr error) {
	updates := map[string]interface{}{
		"agent_active_digest":      state.ActiveDigest,
		"agent_previous_digest":    state.PreviousDigest,
		"agent_last_update_status": state.Status,
		"agent_last_update_error":  state.LastError,
	}
	if callErr != nil {
		updates["agent_last_update_status"] = "failed"
		updates["agent_last_update_error"] = callErr.Error()
	}
	h.db.Model(server).Updates(updates)
}

func applySnapshotToServer(db *gorm.DB, server *models.VPNServer, files map[string][]byte) error {
	if awgRaw := firstSnapshotFile(files, "awg/"); awgRaw != "" {
		applyAWGSnapshot(server, awgRaw)
	}

	var template models.VLESSServerTemplate
	db.Where("server_id = ?", server.ID).FirstOrInit(&template, models.VLESSServerTemplate{ServerID: server.ID})
	if err := applyVLESSSnapshot(server, &template, files); err != nil {
		return err
	}
	if err := db.Save(server).Error; err != nil {
		return err
	}
	if hasUsableVLESSTemplate(&template) {
		return db.Save(&template).Error
	}
	return nil
}

func firstSnapshotFile(files map[string][]byte, prefix string) string {
	for name, content := range files {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimSpace(string(content))
		}
	}
	return ""
}

func applyAWGSnapshot(server *models.VPNServer, raw string) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "Jc":
			server.Jc = val
		case "Jmin":
			server.Jmin = val
		case "Jmax":
			server.Jmax = val
		case "S1":
			server.S1 = val
		case "S2":
			server.S2 = val
		case "S3":
			server.S3 = val
		case "S4":
			server.S4 = val
		case "H1":
			server.H1 = val
		case "H2":
			server.H2 = val
		case "H3":
			server.H3 = val
		case "H4":
			server.H4 = val
		case "ListenPort":
			if port, err := strconv.Atoi(val); err == nil && port > 0 {
				server.AWGPort = port
			}
		}
	}
}

func applyVLESSSnapshot(server *models.VPNServer, template *models.VLESSServerTemplate, files map[string][]byte) error {
	serverConfigRaw := strings.TrimSpace(string(files["xray/server.json"]))
	if serverConfigRaw == "" {
		return nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(serverConfigRaw), &parsed); err != nil {
		return err
	}

	template.ServerID = server.ID
	if strings.TrimSpace(template.Address) == "" {
		template.Address = server.Host
		if server.Endpoint != "" {
			template.Address = strings.Split(server.Endpoint, ":")[0]
		}
	}
	template.PublicKey = strings.TrimSpace(string(files["xray/xray_public.key"]))
	template.ShortID = strings.TrimSpace(string(files["xray/xray_short_id.key"]))
	template.ClientID = strings.TrimSpace(string(files["xray/xray_uuid.key"]))
	template.MLDSA65Verify = ""
	template.ContainerName = "amnezia-xray"

	if inbounds, ok := parsed["inbounds"].([]interface{}); ok && len(inbounds) > 0 {
		for _, rawInbound := range inbounds {
			inbound, ok := rawInbound.(map[string]interface{})
			if !ok {
				continue
			}
			protocol, _ := inbound["protocol"].(string)
			if protocol != "vless" {
				continue
			}

			if port, ok := inbound["port"].(float64); ok && port > 0 {
				template.Port = int(port)
			}
			if streamSettings, ok := inbound["streamSettings"].(map[string]interface{}); ok {
				if network, ok := streamSettings["network"].(string); ok && network != "" {
					template.Network = network
				}
				if security, ok := streamSettings["security"].(string); ok && security != "" {
					template.Security = security
				}
				if realitySettings, ok := streamSettings["realitySettings"].(map[string]interface{}); ok {
					if serverNames, ok := realitySettings["serverNames"].([]interface{}); ok && len(serverNames) > 0 {
						if serverName, ok := serverNames[0].(string); ok {
							template.ServerName = serverName
						}
					}
					if shortIDs, ok := realitySettings["shortIds"].([]interface{}); ok && len(shortIDs) > 0 {
						collected := make([]string, 0, len(shortIDs))
						for _, item := range shortIDs {
							if sid, ok := item.(string); ok && strings.TrimSpace(sid) != "" {
								collected = append(collected, strings.TrimSpace(sid))
							}
						}
						if len(collected) > 0 {
							template.ShortID = collected[0]
							if encoded, err := json.Marshal(collected); err == nil {
								template.ShortIDsJSON = string(encoded)
							}
						}
					}
					template.MLDSA65Verify = ""
					if verify, ok := realitySettings["mldsa65Verify"].(string); ok && strings.TrimSpace(verify) != "" {
						template.MLDSA65Verify = strings.TrimSpace(verify)
					}
				}
			}
			break
		}
	}
	xrayTemplateDefaults(template, server)
	return nil
}
