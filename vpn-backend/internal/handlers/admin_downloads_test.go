package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"vpn-backend/internal/config"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openDownloadsDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "downloads.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AppDownload{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func postDownloadUpload(t *testing.T, r http.Handler, platform, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/downloads/"+platform, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAdminUploadDownloadRejectsWrongPlatformExtension(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openDownloadsDB(t)
	cfg := &config.Config{
		PublicBaseURL: "https://srv.example.com",
		DownloadsDir:  t.TempDir(),
	}
	adminH := NewAdminHandler(db, cfg)
	r := gin.New()
	r.POST("/api/v1/admin/downloads/:platform", adminH.UploadDownload)

	w := postDownloadUpload(t, r, "windows", "client.apk", []byte("apk bytes"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), ".exe") || !strings.Contains(w.Body.String(), ".msi") {
		t.Fatalf("error should explain allowed windows extensions, got %s", w.Body.String())
	}
}

func TestAdminUploadDownloadStoresFileAndWebConfigUsesPublicURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openDownloadsDB(t)
	downloadsDir := t.TempDir()
	cfg := &config.Config{
		PublicBaseURL: "https://srv.example.com",
		DownloadsDir:  downloadsDir,
	}
	adminH := NewAdminHandler(db, cfg)
	webH := NewWebConfigHandler(db, cfg)
	r := gin.New()
	r.POST("/api/v1/admin/downloads/:platform", adminH.UploadDownload)
	r.GET("/api/v1/web/config", webH.Get)
	r.GET("/download/:platform", NewDownloadHandler(db).Download)

	upload := postDownloadUpload(t, r, "android", "fblink.apk", []byte("apk bytes"))
	if upload.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", upload.Code, upload.Body.String())
	}

	configReq := httptest.NewRequest(http.MethodGet, "/api/v1/web/config", nil)
	configResp := httptest.NewRecorder()
	r.ServeHTTP(configResp, configReq)
	if configResp.Code != http.StatusOK {
		t.Fatalf("config status = %d, body = %s", configResp.Code, configResp.Body.String())
	}

	var payload struct {
		Downloads map[string]string `json:"downloads"`
	}
	if err := json.Unmarshal(configResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if got, want := payload.Downloads["android"], "https://srv.example.com/download/android"; got != want {
		t.Fatalf("android download url = %q, want %q", got, want)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/download/android", nil)
	downloadResp := httptest.NewRecorder()
	r.ServeHTTP(downloadResp, downloadReq)
	if downloadResp.Code != http.StatusOK {
		t.Fatalf("download status = %d, body = %s", downloadResp.Code, downloadResp.Body.String())
	}
	if got := downloadResp.Body.String(); got != "apk bytes" {
		t.Fatalf("download body = %q, want apk bytes", got)
	}
}
