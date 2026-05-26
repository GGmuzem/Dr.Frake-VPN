package handlers

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
)

var agentPushReplay = struct {
	sync.Mutex
	nonces map[string]time.Time
}{nonces: map[string]time.Time{}}

type agentPushHeartbeat struct {
	NodeID         string `json:"node_id"`
	Version        string `json:"version"`
	Commit         string `json:"commit"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
	DockerAvailable bool  `json:"docker_available"`
	ActiveDigest   string `json:"active_digest"`
	PreviousDigest string `json:"previous_digest"`
	UpdateStatus   string `json:"update_status"`
	UpdateError    string `json:"update_error"`
}

func (h *AdminHandler) AgentPushHeartbeat(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var payload agentPushHeartbeat
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	server, ok := h.verifyAgentPush(c, strings.TrimSpace(payload.NodeID), body)
	if !ok {
		return
	}

	h.db.Model(&server).Updates(map[string]interface{}{
		"agent_node_id":            payload.NodeID,
		"agent_last_version":       payload.Version,
		"agent_last_commit":        payload.Commit,
		"agent_active_digest":      payload.ActiveDigest,
		"agent_previous_digest":    payload.PreviousDigest,
		"agent_last_update_status": payload.UpdateStatus,
		"agent_last_update_error":  payload.UpdateError,
	})
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AdminHandler) AgentPushSnapshot(c *gin.Context) {
	nodeID := strings.TrimSpace(c.GetHeader("X-FBLink-Node-ID"))
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 25<<20)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	server, ok := h.verifyAgentPush(c, nodeID, body)
	if !ok {
		return
	}
	files, err := readSnapshotTarGz(body)
	if err != nil {
		now := time.Now().UTC()
		h.db.Model(&server).Updates(map[string]interface{}{
			"agent_last_snapshot_at":     &now,
			"agent_last_snapshot_status": "invalid_push_snapshot: " + err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	contentHash := strings.TrimSpace(c.GetHeader("X-FBLink-Snapshot-Hash"))
	if contentHash == "" {
		sum := sha256.Sum256(body)
		contentHash = hex.EncodeToString(sum[:])
	}
	if err := applySnapshotToServer(h.db, &server, files); err != nil {
		now := time.Now().UTC()
		h.db.Model(&server).Updates(map[string]interface{}{
			"agent_last_snapshot_at":     &now,
			"agent_last_snapshot_hash":   contentHash,
			"agent_last_snapshot_status": "push_parse_failed: " + err.Error(),
		})
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	now := time.Now().UTC()
	server.AgentLastSnapshotHash = contentHash
	server.AgentLastSnapshotAt = &now
	server.AgentLastSnapshotStatus = "push_ok"
	h.db.Save(&server)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "snapshot_hash": contentHash})
}

func (h *AdminHandler) verifyAgentPush(c *gin.Context, nodeID string, body []byte) (models.VPNServer, bool) {
	var server models.VPNServer
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing node id"})
		return server, false
	}
	if err := h.db.Where("agent_node_id = ?", nodeID).First(&server).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown node id"})
		return server, false
	}
	if strings.TrimSpace(server.AgentPushPublicKey) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "agent push key is not configured"})
		return server, false
	}
	if err := verifyAgentPushSignature(c.Request, body, server.AgentPushPublicKey, nodeID); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return server, false
	}
	return server, true
}

func verifyAgentPushSignature(r *http.Request, body []byte, publicKeyRaw, nodeID string) error {
	timestamp := r.Header.Get(agentSignatureTimestampHeader)
	nonce := r.Header.Get(agentSignatureNonceHeader)
	signatureB64 := r.Header.Get(agentSignatureHeader)
	if timestamp == "" || nonce == "" || signatureB64 == "" {
		return fmt.Errorf("missing signature headers")
	}
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	if skew := time.Since(ts.UTC()); skew > 5*time.Minute || skew < -5*time.Minute {
		return fmt.Errorf("signature timestamp outside allowed skew")
	}
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyRaw)
	if err != nil {
		return fmt.Errorf("invalid agent push public key")
	}
	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid agent push public key size")
	}
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), agentCanonicalRequest(r.Method, r.URL.Path, timestamp, nonce, body), signature) {
		return fmt.Errorf("signature verification failed")
	}

	replayKey := nodeID + ":" + nonce
	agentPushReplay.Lock()
	defer agentPushReplay.Unlock()
	cutoff := time.Now().UTC().Add(-5 * time.Minute)
	for key, seenAt := range agentPushReplay.nonces {
		if seenAt.Before(cutoff) {
			delete(agentPushReplay.nonces, key)
		}
	}
	if _, exists := agentPushReplay.nonces[replayKey]; exists {
		return fmt.Errorf("replayed nonce")
	}
	agentPushReplay.nonces[replayKey] = ts.UTC()
	return nil
}
