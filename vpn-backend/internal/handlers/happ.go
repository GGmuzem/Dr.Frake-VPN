package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"vpn-backend/internal/config"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HappHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

const happSubscriptionTitle = "FBLink VPN"

func NewHappHandler(db *gorm.DB, cfg *config.Config) *HappHandler {
	return &HappHandler{db: db, cfg: cfg}
}

func randomSubscriptionToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func happTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func (h *HappHandler) publicBaseURL(c *gin.Context) string {
	if h.cfg != nil && strings.TrimSpace(h.cfg.PublicBaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(h.cfg.PublicBaseURL), "/")
	}
	return strings.TrimRight(requestBaseURL(c), "/")
}

func isActivePaidHappSubscription(sub models.Subscription) bool {
	if sub.Status != models.SubActive || time.Now().After(sub.ExpiresAt) {
		return false
	}

	switch sub.Plan {
	case models.PlanBasic, models.PlanBasic3M, models.PlanVIP, models.PlanVIP3M:
		return true
	default:
		return false
	}
}

// POST /api/v1/me/happ-link
func (h *HappHandler) CreateLink(c *gin.Context) {
	userID := c.GetUint("user_id")

	sub, err := ensureDefaultSubscription(h.db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
		return
	}
	if !isActivePaidHappSubscription(sub) {
		c.JSON(http.StatusForbidden, gin.H{"error": "paid subscription required"})
		return
	}

	token, err := randomSubscriptionToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate subscription token"})
		return
	}

	record := models.HappSubscriptionToken{
		UserID:    userID,
		TokenHash: happTokenHash(token),
		Label:     "iOS Happ",
	}
	if err := h.db.Create(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store subscription token"})
		return
	}

	go func() {
		if err := h.ensureHappCredentials(userID, sub); err != nil {
			fmt.Printf("[WARN] failed to prepare Happ credentials for user %d: %v\n", userID, err)
		}
	}()

	subscriptionURL := h.publicBaseURL(c) + "/api/v1/happ/sub/" + token + "#" + url.PathEscape(happSubscriptionTitle)
	c.JSON(http.StatusOK, gin.H{
		"subscription_url": subscriptionURL,
		"happ_url":         h.happDeepLink(subscriptionURL),
	})
}

func happAddURL(subscriptionURL string) string {
	return "happ://add/" + base64.StdEncoding.EncodeToString([]byte(subscriptionURL))
}

func (h *HappHandler) happDeepLink(subscriptionURL string) string {
	cryptoAPIURL := ""
	if h.cfg != nil {
		cryptoAPIURL = strings.TrimSpace(h.cfg.HappCryptoAPIURL)
	}
	if cryptoAPIURL == "" {
		return happAddURL(subscriptionURL)
	}

	encrypted, err := happCryptoLink(cryptoAPIURL, subscriptionURL)
	if err != nil {
		fmt.Printf("[WARN] Happ crypto link failed, falling back to plain deep link: %v\n", err)
		return happAddURL(subscriptionURL)
	}
	return encrypted
}

func happCryptoLink(apiURL, subscriptionURL string) (string, error) {
	payload, err := json.Marshal(map[string]string{"url": subscriptionURL})
	if err != nil {
		return "", err
	}

	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Post(apiURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 16*1024))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("crypto API status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		EncryptedLink string `json:"encrypted_link"`
		Error         string `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	if decoded.Error != "" {
		return "", errors.New(decoded.Error)
	}
	encrypted := strings.TrimSpace(decoded.EncryptedLink)
	if !strings.HasPrefix(encrypted, "happ://crypt") {
		return "", fmt.Errorf("crypto API returned invalid Happ link")
	}
	return encrypted, nil
}

// GET /api/v1/happ/sub/:token
func (h *HappHandler) Subscription(c *gin.Context) {
	rawToken := strings.TrimSpace(c.Param("token"))
	if rawToken == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	var token models.HappSubscriptionToken
	if err := h.db.Where("token_hash = ? AND revoked_at IS NULL", happTokenHash(rawToken)).First(&token).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	sub, err := ensureDefaultSubscription(h.db, token.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
		return
	}
	if !isActivePaidHappSubscription(sub) {
		c.JSON(http.StatusForbidden, gin.H{"error": "paid subscription required"})
		return
	}

	lines, err := h.happVLESSLinks(token.UserID, sub)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build happ subscription"})
		return
	}
	if len(lines) == 0 {
		go func() {
			if err := h.ensureHappCredentials(token.UserID, sub); err != nil {
				fmt.Printf("[WARN] failed to prepare Happ credentials for user %d: %v\n", token.UserID, err)
			}
		}()
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "happ configs are being prepared"})
		return
	}

	now := time.Now()
	_ = h.db.Model(&models.HappSubscriptionToken{}).Where("id = ?", token.ID).Update("last_used_at", &now).Error

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="fblink-happ.txt"`)
	c.Header("Cache-Control", "no-store")

	totalBytes := int64(100) * 1024 * 1024 * 1024 * 1024 // 100 TB to represent unlimited
	expireUnix := sub.ExpiresAt.Unix()
	c.Header("Subscription-Userinfo", fmt.Sprintf("upload=0; download=0; total=%d; expire=%d", totalBytes, expireUnix))
	c.Header("profile-update-interval", "24")
	c.Header("profile-web-page-url", h.publicBaseURL(c))
	c.Header("profile-title", happSubscriptionTitle)

	c.String(http.StatusOK, strings.Join(lines, "\n"))
}

func (h *HappHandler) happVLESSLinks(userID uint, sub models.Subscription) ([]string, error) {
	isVIP := isVIPSubscription(sub)
	var servers []models.VPNServer
	query := h.db.Where("active = ?", true)
	if !isVIP {
		query = query.Where("vip_only = ?", false)
	}
	if err := query.Preload("VLESSTemplate").Order("id asc").Find(&servers).Error; err != nil {
		return nil, err
	}

	links := make([]string, 0, len(servers))
	for i := range servers {
		server := &servers[i]
		template := server.VLESSTemplate
		if template != nil {
			xrayTemplateDefaults(template, server)
		}
		if !hasUsableVLESSTemplate(template) {
			continue
		}

		var credentials []models.VLESSCredential
		err := h.db.
			Where("user_id = ? AND server_id = ? AND revoked_at IS NULL", userID, server.ID).
			Limit(1).
			Find(&credentials).Error
		if err != nil {
			continue
		}
		if len(credentials) == 0 {
			continue
		}
		credential := credentials[0]
		clientID := strings.TrimSpace(credential.ClientID)
		if clientID == "" {
			continue
		}

		link := buildHappVLESSURI(clientID, server, template)
		if link != "" {
			links = append(links, link)
		}
	}
	return links, nil
}

func (h *HappHandler) ensureHappCredentials(userID uint, sub models.Subscription) error {
	isVIP := isVIPSubscription(sub)
	var servers []models.VPNServer
	query := h.db.Where("active = ?", true)
	if !isVIP {
		query = query.Where("vip_only = ?", false)
	}
	if err := query.Preload("VLESSTemplate").Order("id asc").Find(&servers).Error; err != nil {
		return err
	}

	errCh := make(chan error, len(servers))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for i := range servers {
		server := servers[i]
		wg.Add(1)
		go func() {
			defer wg.Done()

			template := server.VLESSTemplate
			if template != nil {
				xrayTemplateDefaults(template, &server)
			}
			if !hasUsableVLESSTemplate(template) {
				return
			}

			sem <- struct{}{}
			defer func() { <-sem }()

			if _, err := ensureVLESSCredential(h.db, userID, &server, template); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func buildHappVLESSURI(clientID string, server *models.VPNServer, template *models.VLESSServerTemplate) string {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" || template == nil || server == nil {
		return ""
	}

	xrayTemplateDefaults(template, server)
	address := strings.TrimSpace(template.Address)
	if address == "" {
		address = strings.TrimSpace(server.Endpoint)
	}
	if address == "" {
		address = strings.TrimSpace(server.Host)
	}
	if address == "" || template.Port <= 0 {
		return ""
	}

	params := url.Values{}
	params.Set("encryption", "none")
	params.Set("security", template.Security)
	params.Set("type", template.Network)
	params.Set("sni", template.ServerName)
	params.Set("fp", template.Fingerprint)
	params.Set("pbk", template.PublicKey)
	params.Set("sid", template.ShortID)
	params.Set("spx", template.SpiderX)
	if strings.TrimSpace(template.Flow) != "" {
		params.Set("flow", template.Flow)
	}
	if strings.TrimSpace(template.MLDSA65Verify) != "" {
		params.Set("pqv", template.MLDSA65Verify)
	}

	name := strings.TrimSpace(server.Region)
	if name == "" {
		name = strings.TrimSpace(server.Name)
	}
	if name == "" {
		name = address
	}

	prefix := "FBLink VPN"
	if flag := countryCodeToEmoji(server.CountryCode); flag != "" {
		prefix = flag + " FBLink"
	}
	fragment := url.PathEscape(prefix + " - " + name)

	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		url.PathEscape(clientID),
		address,
		template.Port,
		params.Encode(),
		fragment,
	)
}

func countryCodeToEmoji(cc string) string {
	cc = strings.ToUpper(strings.TrimSpace(cc))
	if len(cc) != 2 {
		return ""
	}
	return string([]rune{rune(cc[0]) + 127397, rune(cc[1]) + 127397})
}
