package handlers

import (
	"fmt"
	"strings"
	"time"

	"vpn-backend/internal/models"

	"gorm.io/gorm"
)

// RunAutoConfigGenerator starts a background loop to proactively generate missing
// VLESS credentials for active users and servers.
func RunAutoConfigGenerator(db *gorm.DB) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Run immediately on startup
	generateMissingConfigs(db)

	for {
		<-ticker.C
		generateMissingConfigs(db)
	}
}

func generateMissingConfigs(db *gorm.DB) {
	fmt.Println("[CONFIG_GEN] Starting background config generation pass...")
	
	var servers []models.VPNServer
	if err := db.Where("active = ?", true).Preload("VLESSTemplate").Order("id asc").Find(&servers).Error; err != nil {
		fmt.Printf("[CONFIG_GEN] Failed to fetch active servers: %v\n", err)
		return
	}

	var subscriptions []models.Subscription
	if err := db.Where("status = ? AND expires_at > ? AND user_id IN (SELECT id FROM users WHERE deleted_at IS NULL)", models.SubActive, time.Now()).Find(&subscriptions).Error; err != nil {
		fmt.Printf("[CONFIG_GEN] Failed to fetch subscriptions: %v\n", err)
		return
	}

	for i := range servers {
		server := servers[i]
		
		template, err := ensureVLESSTemplate(db, &server)
		if err != nil {
			fmt.Printf("[CONFIG_GEN] Skipping server %s (%d) due to template error: %v\n", server.Name, server.ID, err)
			continue
		}
		if !hasUsableVLESSTemplate(template) {
			continue
		}

		for _, sub := range subscriptions {
			// Skip VIP servers for non-VIP users
			if server.VIPOnly && !isVIPSubscription(sub) {
				continue
			}
			if template != nil && strings.TrimSpace(template.ClientID) != "" {
				continue
			}

			var cred models.VLESSCredential
			err := db.Where("user_id = ? AND server_id = ?", sub.UserID, server.ID).Limit(1).Find(&cred).Error
			
			// If credential already exists and is active, skip
			if err == nil && cred.ID != 0 && cred.RevokedAt == nil {
				continue
			}

			fmt.Printf("[CONFIG_GEN] Generating missing config for User %d on Server %d (%s)...\n", sub.UserID, server.ID, server.Name)
			if _, err := ensureVLESSCredential(db, sub.UserID, &server, template); err != nil {
				fmt.Printf("[CONFIG_GEN] Failed for User %d on Server %d: %v\n", sub.UserID, server.ID, err)
			}
			
			// Small delay to prevent SSH throttling when bulk generating
			time.Sleep(200 * time.Millisecond)
		}
	}
	
	fmt.Println("[CONFIG_GEN] Background config generation pass completed.")
}
