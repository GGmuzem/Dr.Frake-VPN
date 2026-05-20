package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

// POST /api/v1/me/happ-link
func (h *HappHandler) CreateLink(c *gin.Context) {
	userID := c.GetUint("user_id")

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

	subscriptionURL := h.publicBaseURL(c) + "/api/v1/happ/sub/" + token
	c.JSON(http.StatusOK, gin.H{
		"subscription_url": subscriptionURL,
		"happ_url":         happAddURL(subscriptionURL),
	})
}

func happAddURL(subscriptionURL string) string {
	return "happ://add/" + base64.StdEncoding.EncodeToString([]byte(subscriptionURL))
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
	if sub.Status != models.SubActive || time.Now().After(sub.ExpiresAt) {
		c.JSON(http.StatusForbidden, gin.H{"error": "active subscription required"})
		return
	}

	lines, err := h.happVLESSLinks(token.UserID, sub)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build happ subscription"})
		return
	}
	if len(lines) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available happ configs"})
		return
	}

	now := time.Now()
	_ = h.db.Model(&models.HappSubscriptionToken{}).Where("id = ?", token.ID).Update("last_used_at", &now).Error

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="fblink-happ.txt"`)
	c.Header("Cache-Control", "no-store")
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
		// Never touch SSH on the subscription hot path — the per-server
		// refresh can stack tens of seconds and iOS clients give up with
		// "url подписки не валиден". Templates are refreshed by the
		// background scheduler (see RefreshAllVLESSTemplates).
		template := loadUsableVLESSTemplate(h.db, server)
		if template == nil {
			continue
		}

		clientID := strings.TrimSpace(template.ClientID)
		if clientID == "" {
			credential, err := ensureVLESSCredentialNoSSH(h.db, userID, server, template)
			if err != nil || credential == nil {
				continue
			}
			clientID = credential.ClientID
		}

		link := buildHappVLESSURI(clientID, server, template)
		if link != "" {
			links = append(links, link)
		}
	}
	return links, nil
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
	fragment := url.QueryEscape("FBLink VPN - " + name)

	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		url.PathEscape(clientID),
		address,
		template.Port,
		params.Encode(),
		fragment,
	)
}
