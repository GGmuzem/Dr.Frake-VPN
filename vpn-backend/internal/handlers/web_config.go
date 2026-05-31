package handlers

import (
	"net/http"
	"vpn-backend/internal/config"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type WebConfigHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewWebConfigHandler(db *gorm.DB, cfg *config.Config) *WebConfigHandler {
	return &WebConfigHandler{db: db, cfg: cfg}
}

// GET /api/v1/web/config
func (h *WebConfigHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"brand": "FBLink VPN",
		"plans": []gin.H{
			webPlan("premium", "Premium", models.PlanBasic, models.PlanBasic3M),
			webPlan("vip", "VIP", models.PlanVIP, models.PlanVIP3M),
		},
		"downloads": gin.H{
			"android": h.downloadURL("android", "https://fblink-sc.com/download/android"),
			"windows": h.downloadURL("windows", "https://fblink-sc.com/download/windows"),
			"macos":   h.downloadURL("macos", "https://fblink-sc.com/download/macos"),
			"linux":   h.downloadURL("linux", "https://fblink-sc.com/download/linux"),
			"happ":    h.configValue("happ", "https://apps.apple.com/search?term=happ%20proxy"),
		},
		"support": gin.H{
			"email":    h.configValue("support_email", "support@frakebit.com"),
			"telegram": h.configValue("support_telegram", "https://t.me/+79966732628"),
		},
	})
}

func (h *WebConfigHandler) downloadURL(platform, fallback string) string {
	if h.db != nil {
		var download models.AppDownload
		if err := h.db.Select("id").Where("platform = ?", platform).First(&download).Error; err == nil {
			return publicDownloadURL(h.cfg, platform)
		}
	}
	return h.configValue(platform, fallback)
}

func (h *WebConfigHandler) configValue(key, fallback string) string {
	if h.cfg == nil {
		return fallback
	}
	value := ""
	switch key {
	case "android":
		value = h.cfg.AndroidDownloadURL
	case "windows":
		value = h.cfg.WindowsDownloadURL
	case "macos":
		value = h.cfg.MacOSDownloadURL
	case "linux":
		value = h.cfg.LinuxDownloadURL
	case "happ":
		value = h.cfg.HappAppURL
	case "support_email":
		value = h.cfg.SupportEmail
	case "support_telegram":
		value = h.cfg.SupportTelegramURL
	}
	if value == "" {
		return fallback
	}
	return value
}

func webPlan(code, title string, monthlyPlan, threeMonthPlan models.PlanType) gin.H {
	monthly := planPrices[monthlyPlan]
	threeMonth := planPrices[threeMonthPlan]
	description := "Быстрый защищенный доступ для ежедневной работы."
	features := []string{"Безлимитный трафик", "Все основные платформы", "Быстрое подключение"}
	if code == "vip" {
		description = "Приоритетная сеть, Xray и расширенные функции приватности."
		features = []string{"VLESS/Xray Reality", "VIP-серверы", "AdBlock DNS"}
	}
	return gin.H{
		"code":        code,
		"title":       title,
		"description": description,
		"features":    features,
		"periods": []gin.H{
			{
				"id":            monthlyPlan,
				"label":         "1 месяц",
				"duration_days": monthly.DurationDays,
				"amount":        monthly.Amount,
				"currency":      "RUB",
			},
			{
				"id":            threeMonthPlan,
				"label":         "3 месяца",
				"duration_days": threeMonth.DurationDays,
				"amount":        threeMonth.Amount,
				"currency":      "RUB",
			},
		},
	}
}
