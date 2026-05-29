package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestInit(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db := Init(dbPath)
	if db == nil {
		t.Fatal("Expected db to be initialized")
	}

	// Verify the database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("Expected db file to be created at %s", dbPath)
	}
}

func TestAutoMigrate(t *testing.T) {
	// Create an in-memory database
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Call AutoMigrate
	AutoMigrate(db)

	// List of tables to check
	tablesToCheck := []string{
		"users",
		"subscriptions",
		"tv_logins",
		"app_downloads",
		"vpn_servers",
		"vpn_keys",
		"vless_server_templates",
		"vless_credentials",
		"happ_subscription_tokens",
		"routing_profiles",
		"promo_codes",
		"payments",
		"verification_codes",
	}

	for _, tableName := range tablesToCheck {
		if !db.Migrator().HasTable(tableName) {
			t.Errorf("Expected table '%s' to exist after migration", tableName)
		}
	}
}
