package handlers

import (
	"path/filepath"
	"testing"
	"time"
	"vpn-backend/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openRenewalDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "renewal.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Subscription{},
		&models.Payment{},
		&models.VPNServer{},
		&models.VPNKey{},
		&models.VLESSServerTemplate{},
		&models.VLESSCredential{},
		&models.HappSubscriptionToken{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}

func TestSubscriptionMaintenanceExpiresAndRevokesKeysWithoutAutoRenewalConfig(t *testing.T) {
	t.Setenv("YOOKASSA_RECURRING_ENABLED", "")

	db := openRenewalDB(t)
	user := models.User{Email: "expired@example.com", PasswordHash: "hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	subscription := models.Subscription{
		UserID:    user.ID,
		Plan:      models.PlanBasic,
		Status:    models.SubActive,
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if err := db.Create(&subscription).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	server := models.VPNServer{
		Name:      "Normal",
		Host:      "normal.example.com",
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
	key := models.VPNKey{
		UserID:     user.ID,
		ServerID:   server.ID,
		ConfigText: "client-config",
		IssuedAt:   time.Now().Add(-24 * time.Hour),
	}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create key: %v", err)
	}
	vlessCredential := models.VLESSCredential{
		UserID:   user.ID,
		ServerID: server.ID,
		ClientID: "11111111-1111-4111-8111-111111111111",
	}
	if err := db.Create(&vlessCredential).Error; err != nil {
		t.Fatalf("create vless credential: %v", err)
	}
	happToken := "expired-happ-token"
	if err := db.Create(&models.HappSubscriptionToken{
		UserID:    user.ID,
		TokenHash: happTokenHash(happToken),
	}).Error; err != nil {
		t.Fatalf("create happ token: %v", err)
	}

	processAutoRenewals(db, "", "")

	var updatedSub models.Subscription
	if err := db.First(&updatedSub, subscription.ID).Error; err != nil {
		t.Fatalf("load subscription: %v", err)
	}
	if updatedSub.Status != models.SubExpired {
		t.Fatalf("expected subscription to be expired, got %s", updatedSub.Status)
	}

	var updatedKey models.VPNKey
	if err := db.First(&updatedKey, key.ID).Error; err != nil {
		t.Fatalf("load key: %v", err)
	}
	if updatedKey.RevokedAt == nil {
		t.Fatal("expected expired subscription key to be revoked")
	}

	var updatedVLESSCredential models.VLESSCredential
	if err := db.First(&updatedVLESSCredential, vlessCredential.ID).Error; err != nil {
		t.Fatalf("load vless credential: %v", err)
	}
	if updatedVLESSCredential.RevokedAt == nil {
		t.Fatal("expected expired subscription VLESS credential to be revoked")
	}

	var updatedHappToken models.HappSubscriptionToken
	if err := db.Where("token_hash = ?", happTokenHash(happToken)).First(&updatedHappToken).Error; err != nil {
		t.Fatalf("load happ token: %v", err)
	}
	if updatedHappToken.RevokedAt == nil {
		t.Fatal("expected expired subscription Happ token to be revoked")
	}

	var paymentCount int64
	if err := db.Model(&models.Payment{}).Count(&paymentCount).Error; err != nil {
		t.Fatalf("count payments: %v", err)
	}
	if paymentCount != 0 {
		t.Fatalf("expected no auto-renewal payment without config, got %d", paymentCount)
	}
}

func TestAutoRenewalChargesRequireFeatureFlagAndCredentials(t *testing.T) {
	t.Setenv("YOOKASSA_RECURRING_ENABLED", "")
	if autoRenewalChargesEnabled("shop", "key") {
		t.Fatal("expected auto-renewal charges disabled without feature flag")
	}

	t.Setenv("YOOKASSA_RECURRING_ENABLED", "true")
	if autoRenewalChargesEnabled("", "key") {
		t.Fatal("expected auto-renewal charges disabled without shop id")
	}
	if autoRenewalChargesEnabled("shop", " ") {
		t.Fatal("expected auto-renewal charges disabled without secret key")
	}
	if !autoRenewalChargesEnabled("shop", "key") {
		t.Fatal("expected auto-renewal charges enabled with feature flag and credentials")
	}
}
