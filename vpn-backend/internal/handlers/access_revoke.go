package handlers

import (
	"time"
	"vpn-backend/internal/models"

	"gorm.io/gorm"
)

func revokeHappTokens(db *gorm.DB, userID uint, now time.Time) error {
	return db.Model(&models.HappSubscriptionToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", &now).Error
}
