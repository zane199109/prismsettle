package notify

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/logger"
)

// SendTelegramMsg sends Telegram alert
func SendTelegramMsg(msg string) error {
	if config.Cfg.Notify.TelegramToken == "" || config.Cfg.Notify.ChatID == "" {
		return fmt.Errorf("telegram config not set")
	}

	// Construct URL
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.Cfg.Notify.TelegramToken)
	params := url.Values{}
	params.Add("chat_id", config.Cfg.Notify.ChatID)
	params.Add("text", msg)
	params.Add("parse_mode", "Markdown")

	// Send request
	resp, err := http.PostForm(apiURL, params)
	if err != nil {
		logger.Errorf("send telegram msg failed", logger.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram api return status: %s", resp.Status)
	}

	logger.Info("telegram msg sent successfully", logger.String("msg", msg))
	return nil
}
