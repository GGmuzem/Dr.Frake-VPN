package handlers

import (
	"net/http"
	"sort"
	"time"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
)

func (h *AdminHandler) GetOverview(c *gin.Context) {
	var totalUsers, activeSubscriptions, activeKeys, activeVLESSKeys int64
	var totalServers, activeServers, openIncidents int64
	var monthlyRevenue float64

	h.db.Model(&models.User{}).Count(&totalUsers)
	h.db.Model(&models.Subscription{}).Where("status = ?", models.SubActive).Count(&activeSubscriptions)
	h.db.Model(&models.VPNKey{}).Where("revoked_at IS NULL").Count(&activeKeys)
	h.db.Model(&models.VLESSCredential{}).Where("revoked_at IS NULL").Count(&activeVLESSKeys)
	h.db.Model(&models.VPNServer{}).Count(&totalServers)
	h.db.Model(&models.VPNServer{}).Where("active = ?", true).Count(&activeServers)
	h.db.Model(&models.AdminNotification{}).Where("status = ?", models.AdminNotificationStatusOpen).Count(&openIncidents)

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	h.db.Model(&models.Payment{}).
		Where("status = ? AND confirmed_at >= ?", models.PaymentSucceeded, startOfMonth).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&monthlyRevenue)

	var notifications []models.AdminNotification
	h.db.Preload("Server").
		Where("status = ?", models.AdminNotificationStatusOpen).
		Order("created_at desc").
		Limit(20).
		Find(&notifications)

	var servers []models.VPNServer
	h.db.Preload("VLESSTemplate").Find(&servers)
	serverRows := make([]gin.H, 0, len(servers))
	for _, server := range servers {
		var awgCount int64
		var vlessCount int64
		h.db.Model(&models.VPNKey{}).Where("server_id = ? AND revoked_at IS NULL", server.ID).Count(&awgCount)
		h.db.Model(&models.VLESSCredential{}).Where("server_id = ? AND revoked_at IS NULL", server.ID).Count(&vlessCount)
		total := awgCount + vlessCount
		utilization := 0.0
		if server.MaxPeers > 0 {
			utilization = (float64(total) / float64(server.MaxPeers)) * 100
		}
		row := adminServerHealthResponse(server)
		row["active_awg"] = awgCount
		row["active_vless"] = vlessCount
		row["active_keys"] = total
		row["max_peers"] = server.MaxPeers
		row["utilization"] = utilization
		row["is_vip_only"] = server.VIPOnly
		serverRows = append(serverRows, row)
	}
	sort.Slice(serverRows, func(i, j int) bool {
		return serverRows[i]["utilization"].(float64) > serverRows[j]["utilization"].(float64)
	})

	c.JSON(http.StatusOK, gin.H{
		"kpis": gin.H{
			"total_users":          totalUsers,
			"active_subscriptions": activeSubscriptions,
			"active_vpn_keys":      activeKeys + activeVLESSKeys,
			"total_servers":        totalServers,
			"active_servers":       activeServers,
			"open_incidents":       openIncidents,
			"monthly_revenue":      monthlyRevenue,
		},
		"notifications": adminNotificationResponseList(notifications),
		"servers":       serverRows,
	})
}
