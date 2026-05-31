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
	routingLink, err := h.happRoutingLink(token.UserID, sub)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build happ routing profile"})
		return
	}
	if len(lines) == 0 {
		go func() {
			if err := h.ensureHappCredentials(token.UserID, sub); err != nil {
				// pass
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
	if routingLink != "" {
		c.Header("routing", routingLink)
		lines = append([]string{routingLink}, lines...)
	}

	c.String(http.StatusOK, strings.Join(lines, "\n"))
}

func (h *HappHandler) happRoutingLink(userID uint, sub models.Subscription) (string, error) {
	if !isVIPSubscription(sub) {
		return "", nil
	}
	if err := ensureDefaultRoutingProfiles(h.db, userID); err != nil {
		return "", err
	}

	var profiles []models.RoutingProfile
	if err := h.db.Where("user_id = ?", userID).Order("sort_order asc, kind asc, id asc").Find(&profiles).Error; err != nil {
		return "", err
	}

	routingProfile := buildHappRoutingProfile(profiles)
	if routingProfile == nil {
		return "", nil
	}

	payload, err := json.Marshal(routingProfile)
	if err != nil {
		return "", err
	}
	return "happ://routing/onadd/" + base64.StdEncoding.EncodeToString(payload), nil
}

func buildHappRoutingProfile(profiles []models.RoutingProfile) map[string]interface{} {
	directSites := []string{}
	directIP := []string{}
	proxySites := []string{}
	proxyIP := []string{}
	seen := map[string]struct{}{}

	hasEnabledCustomProfile := false
	for _, profile := range profiles {
		if profile.Enabled && profile.Kind == models.RoutingProfileCustom {
			hasEnabledCustomProfile = true
			break
		}
	}

	newestUpdate := int64(0)
	appendUnique := func(target *[]string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := fmt.Sprintf("%p:%s", target, value)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		*target = append(*target, value)
	}

	appendDomainRules := func(target *[]string, profile models.RoutingProfile) {
		for _, domain := range decodeJSONStringArray(profile.DomainsJSON) {
			domain = strings.TrimSpace(domain)
			if domain == "" {
				continue
			}
			appendUnique(target, "full:"+domain)
		}
		for _, suffix := range decodeJSONStringArray(profile.DomainSuffixesJSON) {
			suffix = strings.TrimPrefix(strings.TrimSpace(suffix), ".")
			if suffix == "" {
				continue
			}
			appendUnique(target, "domain:"+suffix)
		}
	}

	for _, profile := range profiles {
		if !profile.Enabled {
			continue
		}
		if profile.Kind == models.RoutingProfileSystem && hasEnabledCustomProfile {
			continue
		}
		if profile.UpdatedAt.Unix() > newestUpdate {
			newestUpdate = profile.UpdatedAt.Unix()
		}

		sites := &directSites
		ips := &directIP
		if profile.Action == models.RoutingProfileProxy {
			sites = &proxySites
			ips = &proxyIP
		}
		appendDomainRules(sites, profile)
		for _, cidr := range decodeJSONStringArray(profile.CIDRsJSON) {
			appendUnique(ips, cidr)
		}
	}

	if len(directSites) == 0 && len(directIP) == 0 && len(proxySites) == 0 && len(proxyIP) == 0 {
		return nil
	}

	lastUpdated := ""
	if newestUpdate > 0 {
		lastUpdated = fmt.Sprintf("%d", newestUpdate)
	}

	return map[string]interface{}{
		"Name":              happSubscriptionTitle,
		"GlobalProxy":       "true",
		"RemoteDNSType":     "DoH",
		"RemoteDNSDomain":   "https://cloudflare-dns.com/dns-query",
		"RemoteDNSIP":       "1.1.1.1",
		"DomesticDNSType":   "DoH",
		"DomesticDNSDomain": "https://dns.google/dns-query",
		"DomesticDNSIP":     "8.8.8.8",
		"Geoipurl":          "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat",
		"Geositeurl":        "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
		"LastUpdated":       lastUpdated,
		"DnsHosts": map[string]string{
			"cloudflare-dns.com": "1.1.1.1",
			"dns.google":         "8.8.8.8",
		},
		"DirectSites":    directSites,
		"DirectIp":       directIP,
		"ProxySites":     proxySites,
		"ProxyIp":        proxyIP,
		"BlockSites":     []string{},
		"BlockIp":        []string{},
		"DomainStrategy": "IPIfNonMatch",
		"FakeDNS":        "false",
	}
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
		if isVIP && hasUsableHysteriaTemplate(template) {
			link := buildHappHysteria2URI(server, template)
			if link != "" {
				links = append(links, link)
			}
			continue
		}
		if isVIP && template != nil && template.HysteriaEnabled {
			continue
		}

		clientID := ""
		var credentials []models.VLESSCredential
		err := h.db.
			Where("user_id = ? AND server_id = ? AND revoked_at IS NULL", userID, server.ID).
			Limit(1).
			Find(&credentials).Error
		if err != nil {
			continue
		}
		if len(credentials) > 0 {
			credential := credentials[0]
			clientID = strings.TrimSpace(credential.ClientID)
		}

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
			if isVIP && hasUsableHysteriaTemplate(template) {
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

func hasUsableHysteriaTemplate(template *models.VLESSServerTemplate) bool {
	if template == nil || !template.HysteriaEnabled {
		return false
	}
	return strings.TrimSpace(template.HysteriaPassword) != "" && template.HysteriaPort > 0
}

func buildHappHysteria2URI(server *models.VPNServer, template *models.VLESSServerTemplate) string {
	if server == nil || template == nil {
		return ""
	}
	xrayTemplateDefaults(template, server)
	if !hasUsableHysteriaTemplate(template) {
		return ""
	}

	address := strings.TrimSpace(template.Address)
	if address == "" {
		address = strings.TrimSpace(server.Endpoint)
	}
	if address == "" {
		address = strings.TrimSpace(server.Host)
	}
	if address == "" {
		return ""
	}

	params := url.Values{}
	if sni := strings.TrimSpace(template.HysteriaSNI); sni != "" {
		params.Set("sni", sni)
	}
	if template.HysteriaInsecure {
		params.Set("insecure", "1")
	}
	if obfsPassword := strings.TrimSpace(template.HysteriaObfsPassword); obfsPassword != "" {
		params.Set("obfs", "salamander")
		params.Set("obfs-password", obfsPassword)
	}

	name := strings.TrimSpace(server.Region)
	if name == "" {
		name = strings.TrimSpace(server.Name)
	}
	if name == "" {
		name = address
	}
	prefix := "FBLink VIP"
	if flag := countryCodeToEmoji(server.CountryCode); flag != "" {
		prefix = flag + " FBLink VIP"
	}
	fragment := url.PathEscape(prefix + " - " + name)
	query := params.Encode()
	if query != "" {
		query = "?" + query
	}

	return fmt.Sprintf("hy2://%s@%s:%d%s#%s",
		url.PathEscape(strings.TrimSpace(template.HysteriaPassword)),
		address,
		template.HysteriaPort,
		query,
		fragment,
	)
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
	if template.Security == "reality" {
		params.Set("pbk", template.PublicKey)
		params.Set("sid", template.ShortID)
		params.Set("spx", template.SpiderX)
	}
	if template.Network == "xhttp" {
		path := strings.TrimSpace(template.XHTTPPath)
		if path == "" {
			path = strings.TrimSpace(template.GrpcServiceName)
		}
		if path == "" {
			path = "/assets/7d91f0e4"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		host := strings.TrimSpace(template.XHTTPHost)
		if host == "" {
			host = template.ServerName
		}
		params.Set("path", path)
		params.Set("host", host)
		params.Set("mode", "auto")
		padding := strings.TrimSpace(template.XHTTPPadding)
		if padding == "" {
			padding = "100-1000"
		}
		params.Set("x_padding_bytes", padding)
		extraJSON := fmt.Sprintf(`{"mode":"auto","scMaxEachPostBytes":"1000000","xPaddingBytes":%q}`, padding)
		params.Set("extra", extraJSON)
		if template.Security == "tls" {
			params.Set("alpn", "h3")
		}
	} else {
		if strings.TrimSpace(template.Flow) != "" {
			params.Set("flow", template.Flow)
		}
	}
	if template.Security == "reality" && strings.TrimSpace(template.MLDSA65Verify) != "" {
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
