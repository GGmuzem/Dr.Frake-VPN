package main

import (
	"fmt"
	"log"
	"os"

	"fblink-vpn/vpn-backend/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "fblink.db"
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	var users []models.User
	db.Find(&users)

	fmt.Println("Users in database:")
	for _, u := range users {
		fmt.Printf("Email: %s | Role: %s\n", u.Email, u.Role)
	}
}
