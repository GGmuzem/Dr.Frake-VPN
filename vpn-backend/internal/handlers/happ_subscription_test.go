package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		&models.RoutingProfile{},
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

func seedSharedHappTemplateClientID(t *testing.T, db *gorm.DB, serverID uint) {
	t.Helper()

	if err := db.Model(&models.VLESSServerTemplate{}).
		Where("server_id = ?", serverID).
		Update("client_id", "11111111-1111-4111-8111-111111111111").Error; err != nil {
		t.Fatalf("seed shared client id: %v", err)
	}
}

func seedHappCredential(t *testing.T, db *gorm.DB, userID uint, serverID uint, clientID string) {
	t.Helper()

	if err := db.Create(&models.VLESSCredential{
		UserID:   userID,
		ServerID: serverID,
		ClientID: clientID,
	}).Error; err != nil {
		t.Fatalf("seed happ credential: %v", err)
	}
}

func storeHappTokenForTest(t *testing.T, db *gorm.DB, userID uint, token string) {
	t.Helper()

	if err := db.Create(&models.HappSubscriptionToken{
		UserID:    userID,
		TokenHash: happTokenHash(token),
		Label:     "iOS Happ",
	}).Error; err != nil {
		t.Fatalf("store happ token: %v", err)
	}
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

	parsed, err := url.Parse(body.SubscriptionURL)
	if err != nil {
		t.Fatalf("parse subscription URL: %v", err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	return parts[len(parts)-1]
}

func TestCreateHappLinkUsesBase64DeepLinkPayload(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanBasic, time.Now().Add(24*time.Hour))

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("user_id", uint(1))
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/me/happ-link", nil)
	handler.CreateLink(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected create link status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		SubscriptionURL string `json:"subscription_url"`
		HappURL         string `json:"happ_url"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode create link response: %v", err)
	}
	if !strings.HasPrefix(body.HappURL, "happ://add/") {
		t.Fatalf("expected Happ deep link to use add/base64 format, got %s", body.HappURL)
	}

	payload := strings.TrimPrefix(body.HappURL, "happ://add/")
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("expected Happ payload to be standard base64: %v", err)
	}
	if string(decoded) != body.SubscriptionURL {
		t.Fatalf("expected deep link payload %q, got %q", body.SubscriptionURL, string(decoded))
	}

	parsed, err := url.Parse(body.SubscriptionURL)
	if err != nil {
		t.Fatalf("parse subscription URL: %v", err)
	}
	if parsed.Fragment != happSubscriptionTitle {
		t.Fatalf("expected subscription title %q, got %q", happSubscriptionTitle, parsed.Fragment)
	}
}

func TestCreateHappLinkRejectsFreeSubscription(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanFree, time.Now().Add(24*time.Hour))

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("user_id", uint(1))
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/me/happ-link", nil)
	handler.CreateLink(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected free subscription status 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateHappLinkUsesCryptoAPIWhenConfigured(t *testing.T) {
	db := openHappSubscriptionDB(t)
	seedHappUser(t, db, 1, models.PlanBasic, time.Now().Add(24*time.Hour))

	var receivedURL string
	cryptoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST to crypto API, got %s", r.Method)
		}
		var payload struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode crypto API payload: %v", err)
		}
		receivedURL = payload.URL
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"encrypted_link":"happ://crypt5/encrypted"}`))
	}))
	defer cryptoServer.Close()

	cfg := &config.Config{
		PublicBaseURL:    "https://srv.frakebit.com",
		HappCryptoAPIURL: cryptoServer.URL,
	}
	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("user_id", uint(1))
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/me/happ-link", nil)
	handler.CreateLink(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected create link status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		SubscriptionURL string `json:"subscription_url"`
		HappURL         string `json:"happ_url"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode create link response: %v", err)
	}
	if body.HappURL != "happ://crypt5/encrypted" {
		t.Fatalf("expected encrypted Happ link, got %s", body.HappURL)
	}
	if receivedURL != body.SubscriptionURL {
		t.Fatalf("expected crypto API URL %q, got %q", body.SubscriptionURL, receivedURL)
	}
}

func TestCreateHappLinkDoesNotBlockOnCredentialPreparation(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanBasic, time.Now().Add(24*time.Hour))
	server := seedHappServer(t, db, "Normal", false)
	if err := db.Model(&models.VPNServer{}).
		Where("id = ?", server.ID).
		Updates(map[string]interface{}{
			"ssh_host":     "203.0.113.1",
			"ssh_password": "bad-password",
		}).Error; err != nil {
		t.Fatalf("mark server ssh configured: %v", err)
	}

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("user_id", uint(1))
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/me/happ-link", nil)

	start := time.Now()
	handler.CreateLink(context)
	elapsed := time.Since(start)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected create link status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if elapsed > time.Second {
		t.Fatalf("create Happ link should not wait on SSH credential preparation, elapsed=%s", elapsed)
	}
}

func TestHappSubscriptionAllowsPremiumVLESSWithoutVIPOnlyServers(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanBasic, time.Now().Add(24*time.Hour))
	normalServer := seedHappServer(t, db, "Normal", false)
	vipServer := seedHappServer(t, db, "VIP", true)
	seedSharedHappTemplateClientID(t, db, normalServer.ID)
	seedHappCredential(t, db, 1, normalServer.ID, "22222222-2222-4222-8222-222222222222")
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

	var credential models.VLESSCredential
	if err := db.Where("user_id = ? AND server_id = ?", 1, normalServer.ID).First(&credential).Error; err != nil {
		t.Fatalf("expected personal Happ VLESS credential: %v", err)
	}
	if strings.Contains(body, "11111111-1111-4111-8111-111111111111") {
		t.Fatalf("Happ subscription leaked shared template client id: %s", body)
	}
	if !strings.Contains(body, credential.ClientID) {
		t.Fatalf("expected Happ subscription to use personal credential %s, got %s", credential.ClientID, body)
	}
}

func TestHappSubscriptionDoesNotUseSharedTemplateClientIDWithoutCredential(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanBasic, time.Now().Add(24*time.Hour))
	seedHappServer(t, db, "Normal", false)
	token := "manual-token"
	storeHappTokenForTest(t, db, 1, token)

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "token", Value: token}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/happ/sub/"+token, nil)
	handler.Subscription(context)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected no personal Happ configs status 503, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHappSubscriptionDoesNotRefreshTemplatesOverSSH(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanBasic, time.Now().Add(24*time.Hour))
	server := seedHappServer(t, db, "Normal", false)
	seedHappCredential(t, db, 1, server.ID, "33333333-3333-4333-8333-333333333333")
	token := issueHappTokenForTest(t, db, cfg, 1)

	stale := time.Now().Add(-48 * time.Hour)
	if err := db.Model(&models.VPNServer{}).
		Where("id = ?", server.ID).
		Updates(map[string]interface{}{
			"ssh_host":     "203.0.113.1",
			"ssh_password": "bad-password",
		}).Error; err != nil {
		t.Fatalf("mark server ssh configured: %v", err)
	}
	if err := db.Model(&models.VLESSServerTemplate{}).
		Where("server_id = ?", server.ID).
		Update("updated_at", stale).Error; err != nil {
		t.Fatalf("mark vless template stale: %v", err)
	}

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "token", Value: token}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/happ/sub/"+token, nil)

	start := time.Now()
	handler.Subscription(context)
	elapsed := time.Since(start)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected subscription status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Happ subscription should not wait on SSH refresh, elapsed=%s", elapsed)
	}
}

func TestHappSubscriptionAllowsVIPOnlyServersForVIP(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanVIP, time.Now().Add(24*time.Hour))
	normalServer := seedHappServer(t, db, "Normal", false)
	vipServer := seedHappServer(t, db, "VIP", true)
	seedHappCredential(t, db, 1, normalServer.ID, "44444444-4444-4444-8444-444444444444")
	seedHappCredential(t, db, 1, vipServer.ID, "55555555-5555-4555-8555-555555555555")
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

func TestHappSubscriptionIncludesRoutingProfileForVIP(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanVIP, time.Now().Add(24*time.Hour))
	server := seedHappServer(t, db, "Normal", false)
	seedHappCredential(t, db, 1, server.ID, "66666666-6666-4666-8666-666666666666")
	if err := db.Create(&models.RoutingProfile{
		UserID:             1,
		Name:               "RU direct",
		Kind:               models.RoutingProfileCustom,
		Action:             models.RoutingProfileDirect,
		Enabled:            true,
		DomainsJSON:        `["gosuslugi.ru"]`,
		DomainSuffixesJSON: `[".ru"]`,
		CIDRsJSON:          `["10.0.0.0/8"]`,
	}).Error; err != nil {
		t.Fatalf("create routing profile: %v", err)
	}
	token := issueHappTokenForTest(t, db, cfg, 1)

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/happ/sub/"+token, nil)
	context.Params = gin.Params{{Key: "token", Value: token}}

	handler.Subscription(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	routingLink := recorder.Header().Get("routing")
	if !strings.HasPrefix(routingLink, "happ://routing/onadd/") {
		t.Fatalf("expected Happ routing header, got %q", routingLink)
	}
	if !strings.Contains(recorder.Body.String(), routingLink) {
		t.Fatalf("expected subscription body to include Happ routing link, got %s", recorder.Body.String())
	}

	payload := strings.TrimPrefix(routingLink, "happ://routing/onadd/")
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode routing payload: %v", err)
	}
	var profile map[string]interface{}
	if err := json.Unmarshal(decoded, &profile); err != nil {
		t.Fatalf("decode routing profile json: %v", err)
	}
	if profile["Name"] != "FBLink VPN" {
		t.Fatalf("expected routing profile name FBLink VPN, got %v", profile["Name"])
	}
	directSites := profile["DirectSites"].([]interface{})
	if !containsInterfaceString(directSites, "full:gosuslugi.ru") || !containsInterfaceString(directSites, "domain:ru") {
		t.Fatalf("expected direct domains in Happ routing profile, got %v", directSites)
	}
	directIP := profile["DirectIp"].([]interface{})
	if !containsInterfaceString(directIP, "10.0.0.0/8") {
		t.Fatalf("expected direct CIDR in Happ routing profile, got %v", directIP)
	}
}

func TestHappSubscriptionRejectsExpiredSubscription(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanBasic, time.Now().Add(-time.Hour))
	seedHappServer(t, db, "Normal", false)
	token := "expired-token"
	storeHappTokenForTest(t, db, 1, token)

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

func TestHappSubscriptionRejectsFreeSubscription(t *testing.T) {
	db := openHappSubscriptionDB(t)
	cfg := &config.Config{PublicBaseURL: "https://srv.frakebit.com"}
	seedHappUser(t, db, 1, models.PlanFree, time.Now().Add(24*time.Hour))
	seedHappServer(t, db, "Normal", false)
	token := "free-token"
	storeHappTokenForTest(t, db, 1, token)

	handler := NewHappHandler(db, cfg)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "token", Value: token}}
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/happ/sub/"+token, nil)
	handler.Subscription(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected free subscription status 403, got %d body=%s", recorder.Code, recorder.Body.String())
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
