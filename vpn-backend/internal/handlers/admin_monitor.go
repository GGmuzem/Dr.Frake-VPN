package handlers

import (
	"fmt"
	"log"
	"strings"
	"time"
	"vpn-backend/internal/config"
	"vpn-backend/internal/models"

	"gorm.io/gorm"
)

type AdminAlertSender interface {
	SendAdminAlert(notification models.AdminNotification) error
}

type noopAdminAlertSender struct{}

func (noopAdminAlertSender) SendAdminAlert(models.AdminNotification) error { return nil }

func RunAdminHealthMonitor(db *gorm.DB, cfg *config.Config) {
	sender := NewTelegramAdminAlertSender(cfg)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if err := EvaluateAdminHealthOnce(db, cfg, sender, time.Now().UTC()); err != nil {
			log.Printf("admin health monitor failed: %v", err)
		}
		<-ticker.C
	}
}

func EvaluateAdminHealthOnce(db *gorm.DB, cfg *config.Config, sender AdminAlertSender, now time.Time) error {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if sender == nil {
		sender = noopAdminAlertSender{}
	}
	staleSeconds := cfg.AdminHeartbeatStaleSeconds
	if staleSeconds <= 0 {
		staleSeconds = 180
	}

	var servers []models.VPNServer
	if err := db.Find(&servers).Error; err != nil {
		return err
	}

	for _, server := range servers {
		if !server.Active || strings.TrimSpace(server.AgentNodeID) == "" {
			continue
		}
		fingerprint := fmt.Sprintf("server:%d:agent-heartbeat-stale", server.ID)
		stale := server.AgentLastHeartbeatAt == nil || now.Sub(server.AgentLastHeartbeatAt.UTC()) > time.Duration(staleSeconds)*time.Second
		if stale {
			message := "Node-agent heartbeat не поступал в допустимое окно"
			if server.AgentLastHeartbeatAt != nil {
				message = fmt.Sprintf("Последний heartbeat был %s", server.AgentLastHeartbeatAt.UTC().Format(time.RFC3339))
			}
			notification, created, err := upsertAdminNotification(db, models.AdminNotification{
				Fingerprint:  fingerprint,
				Severity:     models.AdminNotificationSeverityCritical,
				Status:       models.AdminNotificationStatusOpen,
				Title:        "VPS недоступен",
				Message:      message,
				ServerID:     &server.ID,
				MetadataJSON: fmt.Sprintf(`{"server_name":%q,"region":%q,"endpoint":%q,"reason":"stale_heartbeat"}`, server.Name, server.Region, server.Endpoint),
				LastSeenAt:   now,
			})
			if err != nil {
				return err
			}
			if created && cfg.AdminAlertsEnabled {
				if err := sender.SendAdminAlert(notification); err != nil {
					log.Printf("telegram admin alert failed: %v", err)
				}
			}
			continue
		}
		if err := resolveAdminNotification(db, fingerprint, now); err != nil {
			return err
		}
	}

	return nil
}

func upsertAdminNotification(db *gorm.DB, next models.AdminNotification) (models.AdminNotification, bool, error) {
	var current models.AdminNotification
	err := db.Where("fingerprint = ?", next.Fingerprint).First(&current).Error
	if err == nil {
		updates := map[string]interface{}{
			"severity":      next.Severity,
			"status":        models.AdminNotificationStatusOpen,
			"title":         next.Title,
			"message":       next.Message,
			"server_id":     next.ServerID,
			"metadata_json": next.MetadataJSON,
			"last_seen_at":  next.LastSeenAt,
			"resolved_at":   nil,
		}
		if current.Status == models.AdminNotificationStatusResolved {
			updates["acknowledged_at"] = nil
			updates["acknowledged_by_id"] = nil
			updates["muted_until"] = nil
		}
		if err := db.Model(&current).Updates(updates).Error; err != nil {
			return current, false, err
		}
		return current, false, db.First(&current, current.ID).Error
	}
	if err != gorm.ErrRecordNotFound {
		return current, false, err
	}
	if next.LastSeenAt.IsZero() {
		next.LastSeenAt = time.Now().UTC()
	}
	if err := db.Create(&next).Error; err != nil {
		return next, false, err
	}
	return next, true, nil
}

func resolveAdminNotification(db *gorm.DB, fingerprint string, now time.Time) error {
	return db.Model(&models.AdminNotification{}).
		Where("fingerprint = ? AND status = ?", fingerprint, models.AdminNotificationStatusOpen).
		Updates(map[string]interface{}{
			"status":       models.AdminNotificationStatusResolved,
			"resolved_at":  &now,
			"last_seen_at": now,
		}).Error
}
