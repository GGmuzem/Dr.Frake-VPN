package handlers

import (
	"fmt"
	"strings"
	"time"
	"vpn-backend/internal/models"

	"gorm.io/gorm"
)

// vlessTemplateRefresherInterval controls how often the background
// scheduler walks active servers and refreshes their VLESS templates and
// reconciles pending credentials against the remote xray instance.
const vlessTemplateRefresherInterval = 15 * time.Minute

// RunVLESSTemplateRefresher loops forever, refreshing VLESS templates and
// reconciling outstanding credentials with each active server. It is meant
// to be launched in a goroutine from main().
//
// The refresher exists so the request-time path (Happ subscription,
// dashboard config) never has to wait on SSH I/O: it reads from the local
// cache while this loop keeps the cache warm in the background.
func RunVLESSTemplateRefresher(db *gorm.DB) {
	// Warm the cache once at startup so the first Happ subscription
	// request can serve immediately.
	RefreshAllVLESSTemplates(db)
	ticker := time.NewTicker(vlessTemplateRefresherInterval)
	defer ticker.Stop()
	for range ticker.C {
		RefreshAllVLESSTemplates(db)
	}
}

// RefreshAllVLESSTemplates walks every active server, refreshes its
// VLESS template via SSH (if needed), and re-pushes any credentials that
// the remote xray instance may have lost. Errors are logged but never
// fatal — a single broken server should not stop the others from being
// refreshed.
func RefreshAllVLESSTemplates(db *gorm.DB) {
	var servers []models.VPNServer
	if err := db.Where("active = ?", true).Preload("VLESSTemplate").Order("id asc").Find(&servers).Error; err != nil {
		fmt.Printf("[WARN] RefreshAllVLESSTemplates: %v\n", err)
		return
	}

	for i := range servers {
		server := &servers[i]
		refreshSingleVLESSTemplate(db, server)
	}
}

func refreshSingleVLESSTemplate(db *gorm.DB, server *models.VPNServer) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[PANIC] refreshSingleVLESSTemplate server=%s: %v\n", server.Name, r)
		}
	}()

	template, err := ensureVLESSTemplate(db, server)
	if err != nil {
		fmt.Printf("[WARN] refreshSingleVLESSTemplate server=%s: %v\n", server.Name, err)
		return
	}
	if template == nil || !hasUsableVLESSTemplate(template) {
		return
	}
	if strings.TrimSpace(template.ClientID) != "" {
		// Server uses a single shared client UUID — no per-user
		// credentials to reconcile.
		return
	}

	var credentials []models.VLESSCredential
	if err := db.Where("server_id = ? AND revoked_at IS NULL", server.ID).Find(&credentials).Error; err != nil {
		fmt.Printf("[WARN] refreshSingleVLESSTemplate server=%s credentials: %v\n", server.Name, err)
		return
	}
	for _, credential := range credentials {
		if err := addXrayClient(server, template, credential.ClientID); err != nil {
			fmt.Printf("[WARN] refreshSingleVLESSTemplate server=%s client=%s: %v\n", server.Name, credential.ClientID, err)
		}
	}
}
