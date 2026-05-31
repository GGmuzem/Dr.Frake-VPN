package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
)

func (h *AdminHandler) GetNotifications(c *gin.Context) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	status := c.Query("status")
	query := h.db.Preload("Server").Order("created_at desc").Limit(limit)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var notifications []models.AdminNotification
	if err := query.Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"notifications": adminNotificationResponseList(notifications)})
}

func (h *AdminHandler) AckNotification(c *gin.Context) {
	notification, ok := h.loadNotification(c)
	if !ok {
		return
	}
	now := time.Now().UTC()
	var userID *uint
	if raw, exists := c.Get("user_id"); exists {
		if id, ok := raw.(uint); ok {
			userID = &id
		}
	}
	if err := h.db.Model(&notification).Updates(map[string]interface{}{
		"acknowledged_at":    &now,
		"acknowledged_by_id": userID,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	auditAdminAction(h.db, c, "notification.ack", "admin_notification", fmt.Sprint(notification.ID), "ok", "")
	h.db.Preload("Server").First(&notification, notification.ID)
	c.JSON(http.StatusOK, adminNotificationResponse(notification))
}

func (h *AdminHandler) MuteNotification(c *gin.Context) {
	notification, ok := h.loadNotification(c)
	if !ok {
		return
	}
	var req struct {
		Minutes int `json:"minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Minutes <= 0 {
		req.Minutes = 30
	}
	if req.Minutes > 24*60 {
		req.Minutes = 24 * 60
	}
	mutedUntil := time.Now().UTC().Add(time.Duration(req.Minutes) * time.Minute)
	if err := h.db.Model(&notification).Update("muted_until", &mutedUntil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	auditAdminAction(h.db, c, "notification.mute", "admin_notification", fmt.Sprint(notification.ID), "ok", fmt.Sprintf("%d minutes", req.Minutes))
	h.db.Preload("Server").First(&notification, notification.ID)
	c.JSON(http.StatusOK, adminNotificationResponse(notification))
}

func (h *AdminHandler) GetServerHealth(c *gin.Context) {
	id := c.Param("id")
	var server models.VPNServer
	if err := h.db.Preload("VLESSTemplate").First(&server, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var notifications []models.AdminNotification
	h.db.Where("server_id = ? AND status = ?", server.ID, models.AdminNotificationStatusOpen).
		Order("created_at desc").
		Find(&notifications)
	c.JSON(http.StatusOK, gin.H{
		"server":        adminServerHealthResponse(server),
		"notifications": adminNotificationResponseList(notifications),
	})
}

func (h *AdminHandler) AdminEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	send := func() bool {
		var notifications []models.AdminNotification
		if err := h.db.Preload("Server").
			Where("status = ?", models.AdminNotificationStatusOpen).
			Order("created_at desc").
			Limit(20).
			Find(&notifications).Error; err != nil {
			return false
		}
		payload, _ := json.Marshal(gin.H{"notifications": adminNotificationResponseList(notifications)})
		_, _ = fmt.Fprintf(c.Writer, "event: notifications\ndata: %s\n\n", payload)
		c.Writer.Flush()
		return true
	}
	if !send() {
		return
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

func (h *AdminHandler) GetAuditLogs(c *gin.Context) {
	query := h.db.Order("created_at desc").Limit(100)
	if entity := c.Query("entity"); entity != "" {
		query = query.Where("entity = ?", entity)
	}
	if entityID := c.Query("entity_id"); entityID != "" {
		query = query.Where("entity_id = ?", entityID)
	}
	var logs []models.AdminAuditLog
	if err := query.Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		result = append(result, gin.H{
			"id":            log.ID,
			"actor_user_id": log.ActorUserID,
			"action":        log.Action,
			"entity":        log.Entity,
			"entity_id":     log.EntityID,
			"result":        log.Result,
			"message":       log.Message,
			"ip":            log.IP,
			"metadata_json": log.MetadataJSON,
			"created_at":    log.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"audit_logs": result})
}

func (h *AdminHandler) loadNotification(c *gin.Context) (models.AdminNotification, bool) {
	var notification models.AdminNotification
	if err := h.db.First(&notification, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
		return notification, false
	}
	return notification, true
}

func adminNotificationResponseList(notifications []models.AdminNotification) []gin.H {
	result := make([]gin.H, 0, len(notifications))
	for _, notification := range notifications {
		result = append(result, adminNotificationResponse(notification))
	}
	return result
}

func adminNotificationResponse(notification models.AdminNotification) gin.H {
	var server gin.H
	if notification.Server != nil {
		server = gin.H{
			"id":           notification.Server.ID,
			"name":         notification.Server.Name,
			"region":       notification.Server.Region,
			"endpoint":     notification.Server.Endpoint,
			"country_code": notification.Server.CountryCode,
		}
	}
	return gin.H{
		"id":                 notification.ID,
		"fingerprint":        notification.Fingerprint,
		"severity":           notification.Severity,
		"status":             notification.Status,
		"title":              notification.Title,
		"message":            notification.Message,
		"server_id":          notification.ServerID,
		"server":             server,
		"metadata_json":      notification.MetadataJSON,
		"last_seen_at":       notification.LastSeenAt,
		"acknowledged_at":    notification.AcknowledgedAt,
		"acknowledged_by_id": notification.AcknowledgedByID,
		"muted_until":        notification.MutedUntil,
		"resolved_at":        notification.ResolvedAt,
		"created_at":         notification.CreatedAt,
	}
}

func adminServerHealthResponse(server models.VPNServer) gin.H {
	stale := true
	if server.AgentLastHeartbeatAt != nil {
		stale = time.Since(server.AgentLastHeartbeatAt.UTC()) > 3*time.Minute
	}
	return gin.H{
		"id":                         server.ID,
		"name":                       server.Name,
		"host":                       server.Host,
		"endpoint":                   server.Endpoint,
		"region":                     server.Region,
		"country_code":               server.CountryCode,
		"active":                     server.Active,
		"agent_mode":                 serverAgentMode(server),
		"agent_node_id":              server.AgentNodeID,
		"agent_last_heartbeat_at":    server.AgentLastHeartbeatAt,
		"agent_heartbeat_stale":      stale,
		"agent_docker_available":     server.AgentDockerAvailable,
		"agent_uptime_seconds":       server.AgentUptimeSeconds,
		"agent_last_health_status":   server.AgentLastHealthStatus,
		"agent_last_version":         server.AgentLastVersion,
		"agent_last_commit":          server.AgentLastCommit,
		"agent_active_digest":        server.AgentActiveDigest,
		"agent_previous_digest":      server.AgentPreviousDigest,
		"agent_last_update_status":   server.AgentLastUpdateStatus,
		"agent_last_update_error":    server.AgentLastUpdateError,
		"agent_last_snapshot_hash":   server.AgentLastSnapshotHash,
		"agent_last_snapshot_at":     server.AgentLastSnapshotAt,
		"agent_last_snapshot_status": server.AgentLastSnapshotStatus,
		"agent_bootstrap_status":     server.AgentBootstrapStatus,
		"agent_bootstrap_error":      server.AgentBootstrapError,
		"agent_bootstrap_at":         server.AgentBootstrapAt,
		"pihole_enabled":             server.PiHoleEnabled,
		"pihole_last_sync_at":        server.PiHoleLastSyncAt,
		"pihole_last_sync_error":     server.PiHoleLastSyncError,
		"config_summary":             serverConfigSummary(server, server.VLESSTemplate),
	}
}
