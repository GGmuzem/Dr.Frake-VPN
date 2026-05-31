package main

import (
	"log"
	"time"
	"vpn-backend/internal/config"
	"vpn-backend/internal/database"
	"vpn-backend/internal/handlers"
	"vpn-backend/internal/models"
	"vpn-backend/internal/router"

	"golang.org/x/crypto/bcrypt"
)

func safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] goroutine %s: %v", name, r)
			}
		}()
		fn()
	}()
}

func main() {
	cfg := config.Load()

	db := database.Init(cfg.DBPath)
	database.AutoMigrate(db)

	// --- TEMPORARY SEED LOGIC ---
	safeGo("seed-test-user", func() {
		email := "test_billing@frakebit.com"
		password := "password123"

		var user models.User
		if err := db.Where("email = ?", email).First(&user).Error; err == nil {
			// User exists. Delete old subscription.
			db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.Subscription{})

			// Optional: reset password just in case
			hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			db.Model(&user).Update("password_hash", string(hash))
		} else {
			// User does not exist, create it
			hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			user = models.User{
				Email:        email,
				PasswordHash: string(hash),
				Role:         models.RoleUser,
			}
			db.Create(&user)
		}

		db.Create(&models.Subscription{
			UserID:          user.ID,
			Plan:            models.PlanVIP,
			Status:          models.SubActive,
			ExpiresAt:       time.Now().Add(30 * 24 * time.Hour),
			AutoRenew:       true,
			PaymentMethodID: "test_payment_method_id_123",
		})
		log.Printf("Test user %s recreated with linked payment method", email)
	})
	// ---------------------------

	safeGo("sync-servers", func() { handlers.SyncAllServers(db) })
	safeGo("restore-tunnels", func() { handlers.RestoreAgentTunnels(db) })
	safeGo("admin-health-monitor", func() { handlers.RunAdminHealthMonitor(db, cfg) })
	safeGo("renewal-scheduler", func() {
		handlers.RunAutoRenewalScheduler(db, cfg.YooKassaShopID, cfg.YooKassaKey)
	})
	safeGo("config-generator", func() { handlers.RunAutoConfigGenerator(db) })
	// Очистка истёкших кодов подтверждения раз в час
	safeGo("code-cleanup", func() {
		for {
			time.Sleep(1 * time.Hour)
			db.Where("expires_at < ? OR used = true", time.Now().Add(-24*time.Hour)).
				Delete(&models.VerificationCode{})
			db.Where("expires_at < ? OR status IN ?", time.Now().Add(-24*time.Hour),
				[]models.TVLoginStatus{models.TVLoginConsumed, models.TVLoginExpired}).
				Delete(&models.TVLogin{})
		}
	})

	r := router.New(db, cfg)

	log.Printf("Server starting on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
