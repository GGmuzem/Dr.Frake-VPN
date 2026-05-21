package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"vpn-backend/internal/config"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var appDownloadPlatforms = []string{"android", "windows", "macos", "linux"}

var appDownloadExtensions = map[string][]string{
	"android": {".apk"},
	"windows": {".exe", ".msi"},
	"macos":   {".dmg", ".pkg"},
	"linux":   {".AppImage", ".deb", ".rpm", ".tar.gz"},
}

type DownloadHandler struct {
	db *gorm.DB
}

func NewDownloadHandler(db *gorm.DB) *DownloadHandler {
	return &DownloadHandler{db: db}
}

// GET /download/:platform
func (h *DownloadHandler) Download(c *gin.Context) {
	platform := strings.ToLower(strings.TrimSpace(c.Param("platform")))
	if _, ok := appDownloadExtensions[platform]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "download not found"})
		return
	}

	var download models.AppDownload
	if err := h.db.Where("platform = ?", platform).First(&download).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "download not found"})
		return
	}
	if _, err := os.Stat(download.Path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "download file not found"})
		return
	}

	c.FileAttachment(download.Path, download.OriginalName)
}

// GET /api/v1/admin/downloads
func (h *AdminHandler) GetDownloads(c *gin.Context) {
	var downloads []models.AppDownload
	if err := h.db.Find(&downloads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load downloads"})
		return
	}

	byPlatform := make(map[string]models.AppDownload, len(downloads))
	for _, download := range downloads {
		byPlatform[download.Platform] = download
	}

	result := make([]gin.H, 0, len(appDownloadPlatforms))
	for _, platform := range appDownloadPlatforms {
		item := gin.H{
			"platform":           platform,
			"allowed_extensions": appDownloadExtensions[platform],
		}
		if download, ok := byPlatform[platform]; ok {
			item["original_name"] = download.OriginalName
			item["file_name"] = download.FileName
			item["size"] = download.Size
			item["uploaded_at"] = download.UpdatedAt
			item["url"] = publicDownloadURL(h.cfg, platform)
		}
		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{"downloads": result})
}

// POST /api/v1/admin/downloads/:platform
func (h *AdminHandler) UploadDownload(c *gin.Context) {
	platform := strings.ToLower(strings.TrimSpace(c.Param("platform")))
	allowed, ok := appDownloadExtensions[platform]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown platform"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	if !hasAllowedDownloadExtension(file.Filename, allowed) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid file extension for %s, allowed: %s", platform, strings.Join(allowed, ", ")),
		})
		return
	}

	storedName := storedDownloadFileName(platform, file.Filename, allowed)
	dir := downloadStorageDir(h.cfg)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare downloads directory"})
		return
	}
	targetPath := filepath.Join(dir, storedName)

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to open uploaded file"})
		return
	}
	defer src.Close()

	dst, err := os.Create(targetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save uploaded file"})
		return
	}
	size, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(targetPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write uploaded file"})
		return
	}

	var existing models.AppDownload
	err = h.db.Where("platform = ?", platform).First(&existing).Error
	oldPath := existing.Path
	if errors.Is(err, gorm.ErrRecordNotFound) {
		existing = models.AppDownload{Platform: platform}
	} else if err != nil {
		_ = os.Remove(targetPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load existing download"})
		return
	}

	existing.OriginalName = filepath.Base(file.Filename)
	existing.FileName = storedName
	existing.Path = targetPath
	existing.Size = size

	if existing.ID == 0 {
		err = h.db.Create(&existing).Error
	} else {
		err = h.db.Save(&existing).Error
	}
	if err != nil {
		_ = os.Remove(targetPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store download metadata"})
		return
	}
	if oldPath != "" && oldPath != targetPath {
		_ = os.Remove(oldPath)
	}

	c.JSON(http.StatusOK, appDownloadResponse(h.cfg, existing))
}

func appDownloadResponse(cfg *config.Config, download models.AppDownload) gin.H {
	return gin.H{
		"platform":      download.Platform,
		"original_name": download.OriginalName,
		"file_name":     download.FileName,
		"size":          download.Size,
		"uploaded_at":   download.UpdatedAt,
		"url":           publicDownloadURL(cfg, download.Platform),
	}
}

func downloadStorageDir(cfg *config.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.DownloadsDir) == "" {
		return "data/downloads"
	}
	return cfg.DownloadsDir
}

func publicDownloadURL(cfg *config.Config, platform string) string {
	path := "/download/" + platform
	if cfg == nil || strings.TrimSpace(cfg.PublicBaseURL) == "" {
		return path
	}
	return strings.TrimRight(cfg.PublicBaseURL, "/") + path
}

func hasAllowedDownloadExtension(name string, allowed []string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	for _, ext := range allowed {
		if strings.HasSuffix(lowerName, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

func storedDownloadFileName(platform, originalName string, allowed []string) string {
	lowerName := strings.ToLower(strings.TrimSpace(originalName))
	for _, ext := range allowed {
		if strings.HasSuffix(lowerName, strings.ToLower(ext)) {
			return platform + normalizedDownloadExtension(ext)
		}
	}
	return platform + strings.ToLower(filepath.Ext(originalName))
}

func normalizedDownloadExtension(ext string) string {
	if strings.EqualFold(ext, ".AppImage") {
		return ".AppImage"
	}
	return strings.ToLower(ext)
}
