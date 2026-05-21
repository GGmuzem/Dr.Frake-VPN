package main

import (
	"log"
	"time"

	"vpn-backend/internal/models"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("data/vpn.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	email := "test_billing@frakebit.com"
	password := "password123"

	var existingUser models.User
	if err := db.Where("email = ?", email).First(&existingUser).Error; err == nil {
		log.Printf("User %s already exists. Deleting it to recreate.", email)
		db.Unscoped().Delete(&existingUser)
		db.Unscoped().Where("user_id = ?", existingUser.ID).Delete(&models.Subscription{})
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := models.User{
		Email:        email,
		PasswordHash: string(hash),
		Role:         models.RoleUser,
	}

	if err := db.Create(&user).Error; err != nil {
		log.Fatalf("failed to create user: %v", err)
	}

	sub := models.Subscription{
		UserID:          user.ID,
		Plan:            models.PlanVIP,
		Status:          models.SubActive,
		ExpiresAt:       time.Now().Add(30 * 24 * time.Hour),
		AutoRenew:       true,
		PaymentMethodID: "test_payment_method_id_123", // Тестовый способ оплаты
	}

	if err := db.Create(&sub).Error; err != nil {
		log.Fatalf("failed to create subscription: %v", err)
	}

	log.Printf("Successfully created test user!")
	log.Printf("Email: %s", email)
	log.Printf("Password: %s", password)
	log.Printf("Payment Method ID: %s", sub.PaymentMethodID)
	log.Printf("AutoRenew: %v", sub.AutoRenew)
}
