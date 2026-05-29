package database

import (
	"path/filepath"
	"testing"
	"os"

	"github.com/stretchr/testify/assert"
)

func TestInit_Success(t *testing.T) {
	// Create a temporary directory for the database file
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_database.db")

	// Verify that the file does not exist yet
	_, err := os.Stat(dbPath)
	assert.True(t, os.IsNotExist(err), "database file should not exist yet")

	// Call Init with the temporary path
	db := Init(dbPath)

	// Verify the returned *gorm.DB is not nil
	assert.NotNil(t, db, "Init should return a non-nil database connection")

	// Verify the database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "database file should have been created")

	// Execute a simple query to ensure the connection is active
	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err, "should be able to execute a simple query")
	assert.Equal(t, 1, result, "query should return 1")
}
