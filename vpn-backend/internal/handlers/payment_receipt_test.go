package handlers

import (
	"testing"
	"vpn-backend/internal/models"
)

func TestFiscalProductName(t *testing.T) {
	tests := map[models.PlanType]string{
		models.PlanBasic:   "Подписка премиум на сервис 1 мес",
		models.PlanBasic3M: "Подписка премиум на сервис 3 мес",
		models.PlanVIP:     "Подписка вип на сервис 1 мес",
		models.PlanVIP3M:   "Подписка вип на сервис 3 мес",
	}

	for plan, want := range tests {
		if got := fiscalProductName(plan); got != want {
			t.Fatalf("fiscalProductName(%s) = %q, want %q", plan, got, want)
		}
	}
}

func TestYooKassaReceiptUsesCustomerEmailAndVat5Percent(t *testing.T) {
	receipt := yooKassaReceipt("buyer@example.com", models.PlanVIP3M, 1015)

	customer, ok := receipt["customer"].(map[string]interface{})
	if !ok {
		t.Fatalf("receipt customer has unexpected type: %#v", receipt["customer"])
	}
	if got := customer["email"]; got != "buyer@example.com" {
		t.Fatalf("customer email = %#v", got)
	}
	if got := receipt["internet"]; got != true {
		t.Fatalf("receipt internet = %#v, want true", got)
	}

	items, ok := receipt["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("receipt items = %#v", receipt["items"])
	}

	item := items[0]
	if got := item["description"]; got != "Подписка вип на сервис 3 мес" {
		t.Fatalf("item description = %#v", got)
	}
	if got := item["vat_code"]; got != yooKassaVatCode5Percent {
		t.Fatalf("item vat_code = %#v", got)
	}
	if got := item["payment_subject"]; got != "service" {
		t.Fatalf("item payment_subject = %#v", got)
	}
	amount, ok := item["amount"].(map[string]interface{})
	if !ok {
		t.Fatalf("item amount has unexpected type: %#v", item["amount"])
	}
	if got := amount["value"]; got != "1015.00" {
		t.Fatalf("item amount value = %#v", got)
	}
}

func TestYooKassaPaymentMethodForbidsSBP(t *testing.T) {
	method := yooKassaBankCardOnlyPaymentMethod()
	if got := method["type"]; got != "bank_card" {
		t.Fatalf("payment method type = %#v, want bank_card", got)
	}
	if got := method["type"]; got == "sbp" {
		t.Fatalf("payment method must not allow SBP")
	}
}

func TestYooKassaRecurringPaymentsAreDisabled(t *testing.T) {
	t.Setenv("YOOKASSA_RECURRING_ENABLED", "")

	if yooKassaRecurringPaymentsEnabled() {
		t.Fatal("recurring payments must stay disabled until YooKassa enables them for the shop")
	}

	payload := map[string]interface{}{
		"payment_method_data": yooKassaBankCardOnlyPaymentMethod(),
	}
	yooKassaApplyRecurringPaymentOptions(payload)
	if _, ok := payload["save_payment_method"]; ok {
		t.Fatalf("save_payment_method must be omitted while recurring payments are disabled: %#v", payload["save_payment_method"])
	}

	method, ok := payload["payment_method_data"].(map[string]interface{})
	if !ok {
		t.Fatalf("payment_method_data has unexpected type: %#v", payload["payment_method_data"])
	}
	if got := method["type"]; got != "bank_card" {
		t.Fatalf("payment method type = %#v, want bank_card", got)
	}
}

func TestYooKassaRecurringPaymentsCanBeEnabledForConfirmation(t *testing.T) {
	t.Setenv("YOOKASSA_RECURRING_ENABLED", "true")

	if !yooKassaRecurringPaymentsEnabled() {
		t.Fatal("recurring payments should be enabled by YOOKASSA_RECURRING_ENABLED=true")
	}

	payload := map[string]interface{}{
		"payment_method_data": yooKassaBankCardOnlyPaymentMethod(),
	}
	yooKassaApplyRecurringPaymentOptions(payload)
	if got := payload["save_payment_method"]; got != true {
		t.Fatalf("save_payment_method = %#v, want true", got)
	}
}
