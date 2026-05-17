package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"vpn-backend/internal/models"

	"gorm.io/gorm"
)

const yooKassaStatusTimeout = 1500 * time.Millisecond

var yooKassaStatusHTTPClient = &http.Client{
	Timeout: yooKassaStatusTimeout,
}

func verifyYooKassaPaymentStatus(shopID, key, paymentID string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.yookassa.ru/v3/payments/"+paymentID, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(shopID, key)

	resp, err := yooKassaStatusHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("yookassa status check returned http %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	status, _ := data["status"].(string)
	return status, nil
}

func activateSubscriptionFromPayment(tx *gorm.DB, payment *models.Payment, paymentMethodID string) error {
	now := time.Now()
	priceInfo := planPrices[payment.Plan]
	autoRenew := payment.Plan != models.PlanTrial && paymentMethodID != ""

	var sub models.Subscription
	if err := tx.Where("user_id = ?", payment.UserID).First(&sub).Error; err != nil {
		sub = models.Subscription{
			UserID:          payment.UserID,
			Plan:            payment.Plan,
			Status:          models.SubActive,
			ExpiresAt:       now.AddDate(0, 0, priceInfo.DurationDays),
			AutoRenew:       autoRenew,
			PaymentMethodID: paymentMethodID,
		}
		return tx.Model(&models.Subscription{}).Create(map[string]interface{}{
			"user_id":           sub.UserID,
			"plan":              sub.Plan,
			"status":            sub.Status,
			"expires_at":        sub.ExpiresAt,
			"auto_renew":        sub.AutoRenew,
			"payment_method_id": sub.PaymentMethodID,
		}).Error
	}

	var newExpiry time.Time
	if payment.Plan != models.PlanTrial && sub.ExpiresAt.After(now) && sub.Plan != models.PlanFree {
		newExpiry = sub.ExpiresAt.AddDate(0, 0, priceInfo.DurationDays)
	} else {
		newExpiry = now.AddDate(0, 0, priceInfo.DurationDays)
	}

	updates := map[string]interface{}{
		"plan":       payment.Plan,
		"status":     models.SubActive,
		"expires_at": newExpiry,
		"auto_renew": autoRenew,
	}
	if paymentMethodID != "" {
		updates["payment_method_id"] = paymentMethodID
	}

	return tx.Model(&sub).Updates(updates).Error
}

func reconcilePendingUserPayments(db *gorm.DB, userID uint, shopID, key string) error {
	if shopID == "" || key == "" {
		return nil
	}

	var payments []models.Payment
	if err := db.
		Where("user_id = ? AND status = ?", userID, models.PaymentPending).
		Order("created_at desc").
		Limit(5).
		Find(&payments).Error; err != nil {
		return err
	}

	for _, payment := range payments {
		if strings.HasPrefix(payment.YooKassaID, "promo-") {
			continue
		}

		status, err := verifyYooKassaPaymentStatus(shopID, key, payment.YooKassaID)
		if err != nil {
			log.Printf("[payment-sync] status check failed payment_id=%d yookassa_id=%s user_id=%d err=%v",
				payment.ID, payment.YooKassaID, userID, err)
			continue
		}

		switch status {
		case "succeeded":
			if err := db.Transaction(func(tx *gorm.DB) error {
				var current models.Payment
				if err := tx.First(&current, payment.ID).Error; err != nil {
					return err
				}
				if current.Status == models.PaymentSucceeded {
					return markPromoCodeUsed(tx, &current)
				}

				now := time.Now()
				if err := tx.Model(&current).Updates(map[string]interface{}{
					"status":       models.PaymentSucceeded,
					"confirmed_at": now,
				}).Error; err != nil {
					return err
				}
				current.Status = models.PaymentSucceeded
				current.ConfirmedAt = &now

				if err := activateSubscriptionFromPayment(tx, &current, ""); err != nil {
					return err
				}
				return markPromoCodeUsed(tx, &current)
			}); err != nil {
				return err
			}
			log.Printf("[payment-sync] activated pending payment payment_id=%d yookassa_id=%s user_id=%d",
				payment.ID, payment.YooKassaID, userID)
		case "canceled":
			if err := db.Model(&models.Payment{}).
				Where("id = ? AND status = ?", payment.ID, models.PaymentPending).
				Update("status", models.PaymentCancelled).Error; err != nil {
				return err
			}
			log.Printf("[payment-sync] marked pending payment canceled payment_id=%d yookassa_id=%s user_id=%d",
				payment.ID, payment.YooKassaID, userID)
		}
	}

	return nil
}
