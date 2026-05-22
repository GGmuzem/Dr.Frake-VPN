package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
	"vpn-backend/internal/config"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openAdminRevokeDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "admin-revoke.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Subscription{},
		&models.VPNServer{},
		&models.VPNKey{},
		&models.VLESSServerTemplate{},
		&models.VLESSCredential{},
		&models.HappSubscriptionToken{},
		&models.RoutingProfile{},
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

func TestAdminRevokeSubscriptionRevokesHappTokenAndCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminRevokeDB(t)

	user := models.User{Email: "user@example.com", PasswordHash: "hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	sub := models.Subscription{
		UserID:    user.ID,
		Plan:      models.PlanVIP,
		Status:    models.SubActive,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := db.Create(&sub).Error; err != nil {
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
	credential := models.VLESSCredential{
		UserID:   user.ID,
		ServerID: server.ID,
		ClientID: "11111111-1111-4111-8111-111111111111",
	}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	token := "happ-token"
	if err := db.Create(&models.HappSubscriptionToken{
		UserID:    user.ID,
		TokenHash: happTokenHash(token),
	}).Error; err != nil {
		t.Fatalf("create happ token: %v", err)
	}

	handler := NewAdminHandler(db, &config.Config{})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: "1"}}
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/1/subscription/revoke", nil)
	handler.RevokeUserSubscription(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected revoke status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var updatedSub models.Subscription
	if err := db.First(&updatedSub, sub.ID).Error; err != nil {
		t.Fatalf("load subscription: %v", err)
	}
	if updatedSub.Plan != models.PlanFree || updatedSub.Status != models.SubCancelled {
		t.Fatalf("subscription = plan %q status %q, want free cancelled", updatedSub.Plan, updatedSub.Status)
	}

	var updatedCredential models.VLESSCredential
	if err := db.First(&updatedCredential, credential.ID).Error; err != nil {
		t.Fatalf("load credential: %v", err)
	}
	if updatedCredential.RevokedAt == nil {
		t.Fatal("expected VLESS credential to be revoked")
	}

	var updatedToken models.HappSubscriptionToken
	if err := db.Where("token_hash = ?", happTokenHash(token)).First(&updatedToken).Error; err != nil {
		t.Fatalf("load happ token: %v", err)
	}
	if updatedToken.RevokedAt == nil {
		t.Fatal("expected Happ token to be revoked")
	}
}
