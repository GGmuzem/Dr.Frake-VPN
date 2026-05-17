package handlers

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"vpn-backend/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestReconcilePendingUserPaymentsActivatesSucceededPayment(t *testing.T) {
	originalClient := yooKassaStatusHTTPClient
	yooKassaStatusHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"status":"succeeded"}`)),
			}, nil
		}),
	}
	t.Cleanup(func() {
		yooKassaStatusHTTPClient = originalClient
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Subscription{}, &models.Payment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	payment := models.Payment{
		UserID:         42,
		YooKassaID:     "2f1f2f52-000f-5000-9000-1b7e7b000001",
		Amount:         399,
		OriginalAmount: 399,
		Currency:       "RUB",
		Status:         models.PaymentPending,
		Plan:           models.PlanVIP,
	}
	if err := db.Create(&payment).Error; err != nil {
		t.Fatalf("create payment: %v", err)
	}

	if err := reconcilePendingUserPayments(db, 42, "shop", "key"); err != nil {
		t.Fatalf("reconcilePendingUserPayments: %v", err)
	}

	var storedPayment models.Payment
	if err := db.First(&storedPayment, payment.ID).Error; err != nil {
		t.Fatalf("load payment: %v", err)
	}
	if storedPayment.Status != models.PaymentSucceeded {
		t.Fatalf("payment status = %q, want %q", storedPayment.Status, models.PaymentSucceeded)
	}
	if storedPayment.ConfirmedAt == nil {
		t.Fatalf("expected confirmed_at to be set")
	}

	var sub models.Subscription
	if err := db.Where("user_id = ?", 42).First(&sub).Error; err != nil {
		t.Fatalf("load subscription: %v", err)
	}
	if sub.Plan != models.PlanVIP || sub.Status != models.SubActive {
		t.Fatalf("subscription = plan %q status %q, want vip active", sub.Plan, sub.Status)
	}
	if sub.AutoRenew {
		t.Fatalf("expected auto_renew to stay false without saved payment method")
	}
}
