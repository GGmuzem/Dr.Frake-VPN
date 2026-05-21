package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DBPath         string
	JWTSecret      string
	AllowedOrigins string // CORS: "*" или "https://example.com,https://app.example.com"
	PublicBaseURL  string
	YooKassaShopID string
	YooKassaKey    string

	// SMTP для email
	SMTPHost            string
	SMTPPort            int
	SMTPUser            string
	SMTPPassword        string
	SMTPFrom            string
	BackupIntervalHours int

	// Платежи
	PaymentReturnURL string // URL для редиректа после оплаты

	// Web app public settings
	AndroidDownloadURL string
	WindowsDownloadURL string
	MacOSDownloadURL   string
	LinuxDownloadURL   string
	HappAppURL         string
	DownloadsDir       string
	SupportEmail       string
	SupportTelegramURL string

	// Client Updater
	ClientLatestVersion  string
	ClientDownloadURL    string
	ClientReleaseNotes   string
	ClientUpdateCritical bool
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	secret := getEnv("JWT_SECRET", "")
	if secret == "" || secret == "change-me-in-production" {
		log.Fatal("[FATAL] JWT_SECRET не задан или использует дефолтное значение. Укажите безопасный секрет в .env")
	}

	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	backupInterval, _ := strconv.Atoi(getEnv("BACKUP_INTERVAL_HOURS", "24"))
	if backupInterval <= 0 {
		backupInterval = 24
	}

	return &Config{
		Port:           getEnv("PORT", "8081"),
		DBPath:         getEnv("DB_PATH", "data/vpn.db"),
		JWTSecret:      secret,
		PublicBaseURL:  getEnv("PUBLIC_BASE_URL", "https://srv.frakebit.com"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "https://srv.frakebit.com,http://localhost:3000,http://localhost:3001"),
		YooKassaShopID: getEnv("YOOKASSA_SHOP_ID", ""),
		YooKassaKey:    getEnv("YOOKASSA_SECRET_KEY", ""),

		SMTPHost:            getEnv("SMTP_HOST", ""),
		SMTPPort:            smtpPort,
		SMTPUser:            getEnv("SMTP_USER", ""),
		SMTPPassword:        getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:            getEnv("SMTP_FROM", ""),
		BackupIntervalHours: backupInterval,
		PaymentReturnURL:    getEnv("PAYMENT_RETURN_URL", "https://frakebit.com/payment/success"),
		AndroidDownloadURL:  getEnv("ANDROID_DOWNLOAD_URL", "https://srv.frakebit.com/download/android"),
		WindowsDownloadURL:  getEnv("WINDOWS_DOWNLOAD_URL", "https://srv.frakebit.com/download/windows"),
		MacOSDownloadURL:    getEnv("MACOS_DOWNLOAD_URL", "https://srv.frakebit.com/download/macos"),
		LinuxDownloadURL:    getEnv("LINUX_DOWNLOAD_URL", "https://srv.frakebit.com/download/linux"),
		HappAppURL:          getEnv("HAPP_APP_URL", "https://apps.apple.com/search?term=happ%20proxy"),
		DownloadsDir:        getEnv("DOWNLOADS_DIR", "data/downloads"),
		SupportEmail:        getEnv("SUPPORT_EMAIL", "support@frakebit.com"),
		SupportTelegramURL:  getEnv("SUPPORT_TELEGRAM_URL", "https://t.me/fblinkvpn_support"),

		ClientLatestVersion:  getEnv("CLIENT_LATEST_VERSION", "1.0.0"),
		ClientDownloadURL:    getEnv("CLIENT_DOWNLOAD_URL", "https://frakebit.com/download"),
		ClientReleaseNotes:   getEnv("CLIENT_RELEASE_NOTES", "Улучшена стабильность и скорость."),
		ClientUpdateCritical: getEnv("CLIENT_UPDATE_CRITICAL", "false") == "true",
	}
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
