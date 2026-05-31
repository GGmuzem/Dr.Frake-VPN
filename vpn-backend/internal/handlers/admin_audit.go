package handlers

import (
	"fmt"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func auditAdminAction(db *gorm.DB, c *gin.Context, action, entity, entityID, result, message string) {
	var actorID *uint
	if c != nil {
		if raw, exists := c.Get("user_id"); exists {
			switch v := raw.(type) {
			case uint:
				actorID = &v
			case int:
				id := uint(v)
				actorID = &id
			case float64:
				id := uint(v)
				actorID = &id
			}
		}
	}
	ip := ""
	if c != nil && c.Request != nil {
		ip = c.ClientIP()
	}
	if result == "" {
		result = "ok"
	}
	_ = db.Create(&models.AdminAuditLog{
		ActorUserID: actorID,
		Action:      action,
		Entity:      entity,
		EntityID:    entityID,
		Result:      result,
		Message:     message,
		IP:          ip,
	}).Error
}

func auditServerAction(db *gorm.DB, c *gin.Context, action string, serverID uint, result, message string) {
	auditAdminAction(db, c, action, "vpn_server", fmt.Sprint(serverID), result, message)
}
