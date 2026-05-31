package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"vpn-backend/internal/config"
	"vpn-backend/internal/models"
)

type TelegramAdminAlertSender struct {
	botToken string
	chatIDs  []string
	client   *http.Client
}

func NewTelegramAdminAlertSender(cfg *config.Config) AdminAlertSender {
	if cfg == nil || strings.TrimSpace(cfg.TelegramBotToken) == "" || strings.TrimSpace(cfg.TelegramAdminChatIDs) == "" {
		return noopAdminAlertSender{}
	}
	ids := strings.Split(cfg.TelegramAdminChatIDs, ",")
	chatIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			chatIDs = append(chatIDs, id)
		}
	}
	if len(chatIDs) == 0 {
		return noopAdminAlertSender{}
	}
	return &TelegramAdminAlertSender{
		botToken: strings.TrimSpace(cfg.TelegramBotToken),
		chatIDs:  chatIDs,
		client:   &http.Client{Timeout: 8 * time.Second},
	}
}

func (s *TelegramAdminAlertSender) SendAdminAlert(notification models.AdminNotification) error {
	text := fmt.Sprintf(
		"FBLink VPN admin alert\nSeverity: %s\n%s\n%s",
		notification.Severity,
		notification.Title,
		notification.Message,
	)
	for _, chatID := range s.chatIDs {
		body, err := json.Marshal(map[string]string{
			"chat_id": chatID,
			"text":    text,
		})
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken), bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("telegram sendMessage returned %s", resp.Status)
		}
	}
	return nil
}
