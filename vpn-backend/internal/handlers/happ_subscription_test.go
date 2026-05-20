package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"vpn-backend/internal/config"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openHappSubscriptionDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "happ-subscription.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Subscription{},
		&models.VPNServer{},
		&models.VLESSServerTemplate{},
		&models.VLESSCredential{},
		&models.HappSubscriptionToken{},
	).Error; err != nil {
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

func seedHappUser(t *testing.T, db *gorm.DB, userID uint, plan models.PlanType, expiresAt time.Time) {
	t.Helper()

	if err := db.Create(&models.User{
		Model:        gorm.Model{ID: userID},
		Email:        "happ-user@example.com",
		PasswordHash: "hash",
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.Subscription{
		UserID:    userID,
		Plan:      plan,
		Status:    models.SubActive,
		ExpiresAt: expiresAt,
	}).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}
}

func seedHappServer(t *testing.T, db *gorm.DB, name string, vipOnly bool) models.VPNServer {
	t.Helper()

	server := models.VPNServer{
		Name:        name,
		Host:        strings.ToLower(name) + ".example.com",
		PublicKey:   "awg-public-key-" + name,
		Region:      name,
		CountryCode: "NL",
		Active:      true,
		VIPOnly:     vipOnly,
		H1:          "1",
		H2:          "2",
		H3:          "3",
		H4:          "4",
	}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}
	if err := db.Create(&models.VLESSServerTemplate{
		ServerID:    server.ID,
		Address:     server.Host,
		Port:        8443,
		ServerName:  "www.googletagmanager.com",
		PublicKey:   "reality-public-key-" + name,
		ShortID:     "0123456789abcdef",
		Fingerprint: "chrome",
		Flow:        "xtls-rprx-vision",
		Network:     "tcp",
		Security:    "reality",
		SpiderX:     "/",
	}).Error; err != nil {
		t.Fatalf("create vless template: %v", err)
	}
	return server
}

func issueHappTokenForTest(t *testing.T, db *gorm.DB, cfg *config.Config, userID uint) string {
	t.Helper()

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("user_id", userID)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/me/happ-link", nil)
	handler.CreateLink(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected token status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		SubscriptionURL string `json:"subscription_url"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode create link response: %v", err)
	}

	parts := strings.Split(body.SubscriptionURL, "/")
	return parts[len(parts)-1]
}

func TestHappSubscriptionAllowsPremiumVLESSWithoutVIPOnlyServers(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanBasic, time.Now().Add(24*time.Hour))
	normalServer := seedHappServer(t, db, "Normal", false)
	vipServer := seedHappServer(t, db, "VIP", true)
	token := issueHappTokenForTest(t, db, cfg, 1)

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "token", Value: token}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/happ/sub/"+token, nil)
	handler.Subscription(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected subscription status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "vless://") || !strings.Contains(body, normalServer.Host) {
		t.Fatalf("expected Premium Happ subscription to include normal VLESS server, got %s", body)
	}
	if strings.Contains(body, vipServer.Host) {
		t.Fatalf("Premium Happ subscription leaked VIP-only server: %s", body)
	}
}

func TestHappSubscriptionAllowsVIPOnlyServersForVIP(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanVIP, time.Now().Add(24*time.Hour))
	normalServer := seedHappServer(t, db, "Normal", false)
	vipServer := seedHappServer(t, db, "VIP", true)
	token := issueHappTokenForTest(t, db, cfg, 1)

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "token", Value: token}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/happ/sub/"+token, nil)
	handler.Subscription(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected subscription status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, normalServer.Host) || !strings.Contains(body, vipServer.Host) {
		t.Fatalf("expected VIP Happ subscription to include normal and VIP-only servers, got %s", body)
	}
}

func TestHappSubscriptionRejectsExpiredSubscription(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanBasic, time.Now().Add(-time.Hour))
	seedHappServer(t, db, "Normal", false)
	token := issueHappTokenForTest(t, db, cfg, 1)

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "token", Value: token}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/happ/sub/"+token, nil)
	handler.Subscription(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected expired subscription status 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHappSubscriptionRejectsUnknownToken(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	handler := NewHappHandler(db, cfg)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "token", Value: "unknown"}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/happ/sub/unknown", nil)
	handler.Subscription(context)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected unknown token status 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
