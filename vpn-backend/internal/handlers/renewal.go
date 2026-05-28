package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"vpn-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RunAutoRenewalScheduler проверяет каждый час подписки, истекшие сегодня,
// и списывает оплату с сохранённой карты. Вызывать как горутину из main.
func RunAutoRenewalScheduler(db *gorm.DB, shopID, key string) {
	if autoRenewalChargesEnabled(shopID, key) {
		log.Println("[renewal] Subscription maintenance and auto-renewal scheduler started (interval: 1h)")
	} else {
		log.Println("[renewal] Subscription maintenance scheduler started (interval: 1h); auto-renewal charges disabled")
	}

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	processAutoRenewals(db, shopID, key)

	for range ticker.C {
		processAutoRenewals(db, shopID, key)
	}
}

func autoRenewalChargesEnabled(shopID, key string) bool {
	return yooKassaRecurringPaymentsEnabled() && strings.TrimSpace(shopID) != "" && strings.TrimSpace(key) != ""
}

func processAutoRenewals(db *gorm.DB, shopID, key string) {
	now := time.Now()

	// 1. Автосписание для подписок с картой и включённым auto_renew
	if autoRenewalChargesEnabled(shopID, key) {
		var subs []models.Subscription
		db.Where(
			"auto_renew = true AND payment_method_id != '' AND status = ? AND expires_at <= ?",
			models.SubActive, now,
		).Find(&subs)

		if len(subs) > 0 {
			log.Printf("[renewal] Найдено %d истёкших подписок к автосписанию", len(subs))
			for _, sub := range subs {
				if err := chargeAutoRenewal(db, shopID, key, sub); err != nil {
					log.Printf("[renewal] Ошибка списания user_id=%d: %v", sub.UserID, err)
				}
			}
		}
	}

	// 2. Пометить как expired только те активные истёкшие подписки,
	//    у которых НЕТ свежего (< 2ч) платежа в статусе pending.
	//    Это предотвращает race condition: пользователь не теряет доступ,
	//    пока его pending-платёж ещё обрабатывается YooKassa.
	recentCutoff := now.Add(-2 * time.Hour)
	var subsWithPendingPayment []uint
	db.Model(&models.Payment{}).
		Where("status = ? AND created_at >= ?", models.PaymentPending, recentCutoff).
		Pluck("user_id", &subsWithPendingPayment)

	expireQuery := db.Model(&models.Subscription{}).
		Where("status = ? AND expires_at <= ? AND plan != ?", models.SubActive, now, models.PlanFree)
	if len(subsWithPendingPayment) > 0 {
		expireQuery = expireQuery.Where("user_id NOT IN ?", subsWithPendingPayment)
	}
	result := expireQuery.Update("status", models.SubExpired)
	if result.RowsAffected > 0 {
		log.Printf("[renewal] Помечено как expired: %d подписок", result.RowsAffected)

		// Найдём все эти истекшие подписки и отзовём активные ключи их пользователей
		var expiredSubs []models.Subscription
		db.Where("status = ? AND expires_at <= ?", models.SubExpired, now).Find(&expiredSubs)

		for _, s := range expiredSubs {
			if err := revokeHappTokens(db, s.UserID, now); err != nil {
				log.Printf("[renewal] Failed to revoke Happ tokens for expired subscription user_id=%d: %v", s.UserID, err)
			}

			var keys []models.VPNKey
			// Ищем активные ключи этого пользователя (revoked_at IS NULL)
			if err := db.Where("user_id = ? AND revoked_at IS NULL", s.UserID).Preload("Server").Find(&keys).Error; err == nil && len(keys) > 0 {
				revokedKeyIDs := make([]uint, 0, len(keys))
				for _, k := range keys {
					if k.PublicKey != "" {
						if err := removeAWGPeer(&k.Server, k.PublicKey); err != nil {
							log.Printf("[renewal] Failed to remove AWG key user_id=%d server=%s: %v", s.UserID, k.Server.Name, err)
							continue
						}
					}
					revokedKeyIDs = append(revokedKeyIDs, k.ID)
				}
				if len(revokedKeyIDs) > 0 {
					db.Model(&models.VPNKey{}).Where("id IN ? AND revoked_at IS NULL", revokedKeyIDs).Update("revoked_at", now)
					log.Printf("[renewal] Revoked %d AWG keys for expired subscription user_id=%d", len(revokedKeyIDs), s.UserID)
				}
			}

			var xrayCreds []models.VLESSCredential
			if err := db.Where("user_id = ? AND revoked_at IS NULL", s.UserID).Preload("Server.VLESSTemplate").Find(&xrayCreds).Error; err == nil {
				revokedCredentialIDs := make([]uint, 0, len(xrayCreds))
				for _, cred := range xrayCreds {
					if err := removeXrayClient(&cred.Server, cred.Server.VLESSTemplate, cred.ClientID); err != nil {
						log.Printf("[renewal] Failed to remove VLESS credential user_id=%d server=%s: %v", s.UserID, cred.Server.Name, err)
						continue
					}
					revokedCredentialIDs = append(revokedCredentialIDs, cred.ID)
				}
				if len(revokedCredentialIDs) > 0 {
					db.Model(&models.VLESSCredential{}).Where("id IN ? AND revoked_at IS NULL", revokedCredentialIDs).Update("revoked_at", now)
					log.Printf("[renewal] Revoked %d VLESS credentials for expired subscription user_id=%d", len(revokedCredentialIDs), s.UserID)
				}
			}
		}
	}
}

func chargeAutoRenewal(db *gorm.DB, shopID, key string, sub models.Subscription) error {
	priceInfo, ok := planPrices[sub.Plan]
	if !ok {
		return fmt.Errorf("неизвестный план: %s", sub.Plan)
	}

	var user models.User
	if err := db.First(&user, sub.UserID).Error; err != nil {
		return fmt.Errorf("получение email для чека: %w", err)
	}

	idempotencyKey := uuid.New().String()
	payload := map[string]interface{}{
		"amount": map[string]interface{}{
			"value":    fmt.Sprintf("%.2f", priceInfo.Amount),
			"currency": "RUB",
		},
		"capture":           true,
		"payment_method_id": sub.PaymentMethodID,
		"description":       fiscalProductName(sub.Plan),
		"receipt":           yooKassaReceipt(user.Email, sub.Plan, priceInfo.Amount),
		"metadata": map[string]interface{}{
			"user_id":    sub.UserID,
			"plan":       sub.Plan,
			"auto_renew": true,
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.yookassa.ru/v3/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", idempotencyKey)
	req.SetBasicAuth(shopID, key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("запрос к YooKassa: %w", err)
	}
	defer resp.Body.Close()

	var ykResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&ykResp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("YooKassa вернула %d: %v", resp.StatusCode, ykResp)
	}

	status, _ := ykResp["status"].(string)
	ykPaymentID, _ := ykResp["id"].(string)

	// Сохраняем платёж в БД
	now := time.Now()
	payment := models.Payment{
		UserID:         sub.UserID,
		YooKassaID:     ykPaymentID,
		Amount:         priceInfo.Amount,
		OriginalAmount: priceInfo.Amount,
		Currency:       "RUB",
		Plan:           sub.Plan,
	}

	if status == "succeeded" {
		payment.Status = models.PaymentSucceeded
		payment.ConfirmedAt = &now

		// Продлеваем подписку сразу
		newExpiry := sub.ExpiresAt.AddDate(0, 0, priceInfo.DurationDays)
		if newExpiry.Before(now) {
			// Protection against runaway charges if auto-renew ran too late
			newExpiry = now.AddDate(0, 0, priceInfo.DurationDays)
		}
		
		db.Model(&sub).Updates(map[string]interface{}{
			"status":     models.SubActive,
			"expires_at": newExpiry,
		})
		log.Printf("[renewal] Подписка user_id=%d продлена до %s", sub.UserID, newExpiry.Format("2006-01-02"))
	} else {
		// pending — вебхук payment.succeeded сам продлит подписку
		payment.Status = models.PaymentPending
		log.Printf("[renewal] Платёж user_id=%d создан, ожидаем вебхук (status=%s)", sub.UserID, status)
	}

	db.Create(&payment)
	return nil
}
