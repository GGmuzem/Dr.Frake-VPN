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

type yooKassaPaymentDetails struct {
	Status              string
	PaymentMethodID     string
	ReceiptRegistration string
}

func verifyYooKassaPaymentStatus(shopID, key, paymentID string) (string, error) {
	details, err := fetchYooKassaPaymentDetails(shopID, key, paymentID)
	if err != nil {
		return "", err
	}
	return details.Status, nil
}

func fetchYooKassaPaymentDetails(shopID, key, paymentID string) (yooKassaPaymentDetails, error) {
	req, err := http.NewRequest("GET", "https://api.yookassa.ru/v3/payments/"+paymentID, nil)
	if err != nil {
		return yooKassaPaymentDetails{}, err
	}
	req.SetBasicAuth(shopID, key)

	resp, err := yooKassaStatusHTTPClient.Do(req)
	if err != nil {
		return yooKassaPaymentDetails{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return yooKassaPaymentDetails{}, fmt.Errorf("yookassa status check returned http %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return yooKassaPaymentDetails{}, err
	}

	status, _ := data["status"].(string)
	receiptRegistration, _ := data["receipt_registration"].(string)
	details := yooKassaPaymentDetails{
		Status:              status,
		ReceiptRegistration: receiptRegistration,
	}
	if pm, ok := data["payment_method"].(map[string]interface{}); ok {
		if saved, _ := pm["saved"].(bool); saved {
			details.PaymentMethodID, _ = pm["id"].(string)
		}
	}
	return details, nil
}

func activateSubscriptionFromPayment(tx *gorm.DB, payment *models.Payment, paymentMethodID string) error {
	now := time.Now()
	priceInfo := planPrices[payment.Plan]
	if !yooKassaRecurringPaymentsEnabled() {
		paymentMethodID = ""
	}
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
		return tx.Create(&sub).Error
	}

	var newExpiry time.Time
	if payment.Plan != models.PlanTrial && sub.ExpiresAt.After(now) && sub.Plan != models.PlanFree {
		if payment.Plan == sub.Plan {
			// Early renewal for the exact same plan: add to remaining time
			newExpiry = sub.ExpiresAt.AddDate(0, 0, priceInfo.DurationDays)
		} else {
			// Plan upgrade/downgrade: start from now to avoid exploits
			newExpiry = now.AddDate(0, 0, priceInfo.DurationDays)
		}
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

		details, err := fetchYooKassaPaymentDetails(shopID, key, payment.YooKassaID)
		if err != nil {
			log.Printf("[payment-sync] status check failed payment_id=%d yookassa_id=%s user_id=%d err=%v",
				payment.ID, payment.YooKassaID, userID, err)
			continue
		}

		switch details.Status {
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

				if err := activateSubscriptionFromPayment(tx, &current, details.PaymentMethodID); err != nil {
					return err
				}
				return markPromoCodeUsed(tx, &current)
			}); err != nil {
				return err
			}
			log.Printf("[payment-sync] activated pending payment payment_id=%d yookassa_id=%s user_id=%d receipt_registration=%q payment_method_saved=%t",
				payment.ID, payment.YooKassaID, userID, details.ReceiptRegistration, details.PaymentMethodID != "")
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
