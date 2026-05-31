package handlers

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
	"vpn-backend/internal/config"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/curve25519"
	"gorm.io/gorm"
)

type recordingAdminAlertSender struct {
	notifications []models.AdminNotification
}

func (s *recordingAdminAlertSender) SendAdminAlert(notification models.AdminNotification) error {
	s.notifications = append(s.notifications, notification)
	return nil
}

func openAdminMonitorDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "admin-monitor.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Subscription{},
		&models.VPNServer{},
		&models.VLESSServerTemplate{},
		&models.AdminNotification{},
		&models.AdminAuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestAdminImportServerConfigsPersistsAWGAndXrayTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminMonitorDB(t)
	admin := models.User{Email: "admin@example.com", PasswordHash: "hash", Role: models.RoleAdmin}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	server := models.VPNServer{
		Name:      "Berlin-1",
		Host:      "berlin.example.com",
		Endpoint:  "berlin.example.com:443",
		PublicKey: "server-public-key",
		Active:    true,
		H1:        "1",
		H2:        "2",
		H3:        "3",
		H4:        "4",
	}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	body := []byte(`{
		"awg_config":"ListenPort = 51830\nJc = 7\nJmin = 35\nJmax = 85\nS3 = 1\nH1 = 11\nH2 = 22\nH3 = 33\nH4 = 44",
		"xray_config_json":"{\"inbounds\":[{\"protocol\":\"vless\",\"port\":8443,\"streamSettings\":{\"network\":\"xhttp\",\"security\":\"reality\",\"realitySettings\":{\"serverNames\":[\"www.cloudflare.com\"],\"shortIds\":[\"abcdef1234567890\"]},\"xhttpSettings\":{\"path\":\"/assets/admin\"}}}]}",
		"xray_public_key":"reality-public-key",
		"xray_short_id":"abcdef1234567890",
		"xray_client_id":"11111111-1111-4111-8111-111111111111"
	}`)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(server.ID)}}
	context.Set("user_id", admin.ID)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/servers/1/configs/import", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")

	NewAdminHandler(db, &config.Config{}).ImportServerConfigs(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated models.VPNServer
	if err := db.First(&updated, server.ID).Error; err != nil {
		t.Fatalf("load server: %v", err)
	}
	if updated.AWGPort != 51830 || updated.Jc != "7" || updated.H4 != "44" {
		t.Fatalf("AWG fields were not imported: port=%d jc=%q h4=%q", updated.AWGPort, updated.Jc, updated.H4)
	}

	var template models.VLESSServerTemplate
	if err := db.Where("server_id = ?", server.ID).First(&template).Error; err != nil {
		t.Fatalf("load VLESS template: %v", err)
	}
	if template.Port != 8443 || template.Network != "xhttp" || template.PublicKey != "reality-public-key" || template.ServerName != "www.cloudflare.com" {
		t.Fatalf("VLESS template not imported: %#v", template)
	}

	var audit models.AdminAuditLog
	if err := db.Where("action = ? AND entity_id = ?", "server.config_import", fmt.Sprint(server.ID)).First(&audit).Error; err != nil {
		t.Fatalf("expected audit entry: %v", err)
	}
}

func TestAdminImportServerConfigsDerivesXrayRealityPublicKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminMonitorDB(t)
	admin := models.User{Email: "admin@example.com", PasswordHash: "hash", Role: models.RoleAdmin}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	server := models.VPNServer{
		Name:      "Berlin-2",
		Host:      "berlin2.example.com",
		Endpoint:  "berlin2.example.com:443",
		PublicKey: "server-public-key",
		Active:    true,
		H1:        "1",
		H2:        "2",
		H3:        "3",
		H4:        "4",
	}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	privateRaw := make([]byte, curve25519.ScalarSize)
	for index := range privateRaw {
		privateRaw[index] = byte(index + 1)
	}
	expectedPublicRaw, err := curve25519.X25519(privateRaw, curve25519.Basepoint)
	if err != nil {
		t.Fatalf("derive expected public key: %v", err)
	}
	privateKey := base64.RawURLEncoding.EncodeToString(privateRaw)
	expectedPublicKey := base64.RawURLEncoding.EncodeToString(expectedPublicRaw)
	serverJSON := fmt.Sprintf(`{"inbounds":[{"protocol":"vless","port":9443,"streamSettings":{"network":"xhttp","security":"reality","realitySettings":{"privateKey":%q,"serverNames":["www.cloudflare.com"],"shortIds":["feedface"],"mldsa65Verify":"verify-key"},"xhttpSettings":{"path":"/xray/admin"}}}]}`, privateKey)
	body := []byte(fmt.Sprintf(`{
		"xray_config_json":%q,
		"xray_client_id":"22222222-2222-4222-8222-222222222222"
	}`, serverJSON))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(server.ID)}}
	context.Set("user_id", admin.ID)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/servers/1/configs/import", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")

	NewAdminHandler(db, &config.Config{}).ImportServerConfigs(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var template models.VLESSServerTemplate
	if err := db.Where("server_id = ?", server.ID).First(&template).Error; err != nil {
		t.Fatalf("load VLESS template: %v", err)
	}
	if template.PublicKey != expectedPublicKey || template.ShortID != "feedface" || template.MLDSA65Verify != "verify-key" {
		t.Fatalf("VLESS reality fields not derived from server.json: %#v", template)
	}
}

func TestAdminMonitorCreatesDedupesAndResolvesStaleHeartbeatIncident(t *testing.T) {
	db := openAdminMonitorDB(t)
	now := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	oldHeartbeat := now.Add(-4 * time.Minute)
	server := models.VPNServer{
		Name:                 "Amsterdam-1",
		Host:                 "ams.example.com",
		Endpoint:             "ams.example.com:443",
		PublicKey:            "server-public-key",
		Region:               "NL",
		Active:               true,
		AgentNodeID:          "node-ams-1",
		AgentLastHeartbeatAt: &oldHeartbeat,
		H1:                   "1",
		H2:                   "2",
		H3:                   "3",
		H4:                   "4",
	}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	sender := &recordingAdminAlertSender{}
	cfg := &config.Config{
		AdminAlertsEnabled:         true,
		AdminHeartbeatStaleSeconds: 180,
	}
	if err := EvaluateAdminHealthOnce(db, cfg, sender, now); err != nil {
		t.Fatalf("evaluate stale heartbeat: %v", err)
	}
	if err := EvaluateAdminHealthOnce(db, cfg, sender, now.Add(30*time.Second)); err != nil {
		t.Fatalf("evaluate duplicate stale heartbeat: %v", err)
	}

	var notifications []models.AdminNotification
	if err := db.Find(&notifications).Error; err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("expected one deduped notification, got %d", len(notifications))
	}
	incident := notifications[0]
	if incident.Severity != models.AdminNotificationSeverityCritical || incident.Status != models.AdminNotificationStatusOpen {
		t.Fatalf("notification severity/status = %q/%q, want critical/open", incident.Severity, incident.Status)
	}
	if incident.ServerID == nil || *incident.ServerID != server.ID {
		t.Fatalf("notification server id = %#v, want %d", incident.ServerID, server.ID)
	}
	if len(sender.notifications) != 1 {
		t.Fatalf("expected one Telegram alert for deduped incident, got %d", len(sender.notifications))
	}

	freshHeartbeat := now.Add(time.Minute)
	if err := db.Model(&server).Updates(map[string]interface{}{
		"agent_last_heartbeat_at":  &freshHeartbeat,
		"agent_last_health_status": "ok",
	}).Error; err != nil {
		t.Fatalf("fresh heartbeat: %v", err)
	}
	if err := EvaluateAdminHealthOnce(db, cfg, sender, freshHeartbeat); err != nil {
		t.Fatalf("evaluate resolved heartbeat: %v", err)
	}

	var resolved models.AdminNotification
	if err := db.First(&resolved, incident.ID).Error; err != nil {
		t.Fatalf("load resolved notification: %v", err)
	}
	if resolved.Status != models.AdminNotificationStatusResolved || resolved.ResolvedAt == nil {
		t.Fatalf("notification was not resolved: status=%q resolved_at=%v", resolved.Status, resolved.ResolvedAt)
	}
}

func TestAgentPushHeartbeatPersistsHealthFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminMonitorDB(t)
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	server := models.VPNServer{
		Name:               "Paris-1",
		Host:               "par.example.com",
		PublicKey:          "server-public-key",
		Active:             true,
		AgentNodeID:        "node-par-1",
		AgentPushPublicKey: base64.StdEncoding.EncodeToString(publicKey),
		H1:                 "1",
		H2:                 "2",
		H3:                 "3",
		H4:                 "4",
	}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}
	notification := models.AdminNotification{
		Fingerprint: fmt.Sprintf("server:%d:agent-heartbeat-stale", server.ID),
		Severity:    models.AdminNotificationSeverityCritical,
		Status:      models.AdminNotificationStatusOpen,
		Title:       "VPS недоступен",
		Message:     "Нет heartbeat",
		ServerID:    &server.ID,
	}
	if err := db.Create(&notification).Error; err != nil {
		t.Fatalf("create notification: %v", err)
	}

	body := []byte(`{"node_id":"node-par-1","version":"1.2.3","commit":"abc123","uptime_seconds":456,"docker_available":true,"active_digest":"repo/app@sha256:abc","previous_digest":"repo/app@sha256:def","update_status":"ok","update_error":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/node-agent/heartbeat", bytes.NewReader(body))
	signAgentPushRequest(t, req, privateKey, body, "node-par-1", "nonce-heartbeat-1", time.Now().UTC())
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = req

	NewAdminHandler(db, &config.Config{}).AgentPushHeartbeat(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected heartbeat 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated models.VPNServer
	if err := db.First(&updated, server.ID).Error; err != nil {
		t.Fatalf("load server: %v", err)
	}
	if updated.AgentLastHeartbeatAt == nil {
		t.Fatal("expected heartbeat timestamp")
	}
	if !updated.AgentDockerAvailable || updated.AgentUptimeSeconds != 456 || updated.AgentLastHealthStatus != "ok" {
		t.Fatalf("health fields not persisted: docker=%v uptime=%d status=%q", updated.AgentDockerAvailable, updated.AgentUptimeSeconds, updated.AgentLastHealthStatus)
	}
	var resolved models.AdminNotification
	if err := db.First(&resolved, notification.ID).Error; err != nil {
		t.Fatalf("load notification: %v", err)
	}
	if resolved.Status != models.AdminNotificationStatusResolved || resolved.ResolvedAt == nil {
		t.Fatalf("heartbeat did not resolve stale notification: %#v", resolved)
	}
}

func TestGetServerHealthPollsAgentWithoutSigningKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminMonitorDB(t)
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("unexpected agent path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"2.0.0","commit":"def456","node_id":"node-health-1","uptime_seconds":789,"docker_available":true}`))
	}))
	defer agent.Close()

	server := models.VPNServer{
		Name:        "Health-1",
		Host:        "health.example.com",
		PublicKey:   "server-public-key",
		Active:      true,
		AgentURL:    agent.URL,
		AgentNodeID: "node-health-1",
		H1:          "1",
		H2:          "2",
		H3:          "3",
		H4:          "4",
	}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}
	notification := models.AdminNotification{
		Fingerprint: fmt.Sprintf("server:%d:agent-heartbeat-stale", server.ID),
		Severity:    models.AdminNotificationSeverityCritical,
		Status:      models.AdminNotificationStatusOpen,
		Title:       "VPS недоступен",
		Message:     "Нет heartbeat",
		ServerID:    &server.ID,
	}
	if err := db.Create(&notification).Error; err != nil {
		t.Fatalf("create notification: %v", err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(server.ID)}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/servers/1/health", nil)

	NewAdminHandler(db, &config.Config{}).GetServerHealth(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected health 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updated models.VPNServer
	if err := db.First(&updated, server.ID).Error; err != nil {
		t.Fatalf("load server: %v", err)
	}
	if updated.AgentLastHeartbeatAt == nil || updated.AgentLastVersion != "2.0.0" || updated.AgentLastCommit != "def456" || updated.AgentUptimeSeconds != 789 {
		t.Fatalf("agent health was not persisted: %#v", updated)
	}
	var resolved models.AdminNotification
	if err := db.First(&resolved, notification.ID).Error; err != nil {
		t.Fatalf("load notification: %v", err)
	}
	if resolved.Status != models.AdminNotificationStatusResolved || resolved.ResolvedAt == nil {
		t.Fatalf("health poll did not resolve stale notification: %#v", resolved)
	}
}

func TestAdminNotificationAckAndMute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminMonitorDB(t)
	user := models.User{Email: "admin@example.com", PasswordHash: "hash", Role: models.RoleAdmin}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	notification := models.AdminNotification{
		Fingerprint: "server:1:agent-stale",
		Severity:    models.AdminNotificationSeverityCritical,
		Status:      models.AdminNotificationStatusOpen,
		Title:       "VPS недоступен",
		Message:     "Нет heartbeat",
	}
	if err := db.Create(&notification).Error; err != nil {
		t.Fatalf("create notification: %v", err)
	}

	handler := NewAdminHandler(db, &config.Config{})
	ack := httptest.NewRecorder()
	ackCtx, _ := gin.CreateTestContext(ack)
	ackCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(notification.ID)}}
	ackCtx.Set("user_id", user.ID)
	ackCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/notifications/1/ack", nil)
	handler.AckNotification(ackCtx)
	if ack.Code != http.StatusOK {
		t.Fatalf("expected ack 200, got %d body=%s", ack.Code, ack.Body.String())
	}

	mute := httptest.NewRecorder()
	muteCtx, _ := gin.CreateTestContext(mute)
	muteCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(notification.ID)}}
	muteCtx.Set("user_id", user.ID)
	muteCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/notifications/1/mute", bytes.NewReader([]byte(`{"minutes":30}`)))
	muteCtx.Request.Header.Set("Content-Type", "application/json")
	handler.MuteNotification(muteCtx)
	if mute.Code != http.StatusOK {
		t.Fatalf("expected mute 200, got %d body=%s", mute.Code, mute.Body.String())
	}

	var updated models.AdminNotification
	if err := db.First(&updated, notification.ID).Error; err != nil {
		t.Fatalf("load notification: %v", err)
	}
	if updated.AcknowledgedAt == nil || updated.AcknowledgedByID == nil || *updated.AcknowledgedByID != user.ID {
		t.Fatalf("notification was not acknowledged by admin: %#v", updated)
	}
	if updated.MutedUntil == nil || time.Until(*updated.MutedUntil) < 20*time.Minute {
		t.Fatalf("notification was not muted for long enough: %v", updated.MutedUntil)
	}

	var auditCount int64
	db.Model(&models.AdminAuditLog{}).Where("entity = ? AND entity_id = ?", "admin_notification", fmt.Sprint(notification.ID)).Count(&auditCount)
	if auditCount != 2 {
		t.Fatalf("expected two audit entries, got %d", auditCount)
	}
}

func signAgentPushRequest(t *testing.T, req *http.Request, privateKey ed25519.PrivateKey, body []byte, nodeID, nonce string, ts time.Time) {
	t.Helper()
	timestamp := ts.Format(time.RFC3339)
	req.Header.Set("X-FBLink-Node-ID", nodeID)
	req.Header.Set(agentSignatureTimestampHeader, timestamp)
	req.Header.Set(agentSignatureNonceHeader, nonce)
	signature := ed25519.Sign(privateKey, agentCanonicalRequest(req.Method, req.URL.Path, timestamp, nonce, body))
	req.Header.Set(agentSignatureHeader, base64.StdEncoding.EncodeToString(signature))
}
