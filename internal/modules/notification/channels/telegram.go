package channels

import (
	"context"
	"fmt"
	"net/http"
)

const telegramAPIURL = "https://api.telegram.org/bot"

// TelegramChannel handles Telegram bot notifications
type TelegramChannel struct {
	channel    *NotificationChannel
	httpClient HTTPClient
	apiURL     string
}

// telegramMessage represents a Telegram message payload
type telegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// NewTelegramChannel creates a new Telegram channel
func NewTelegramChannel(channel *NotificationChannel, httpClient HTTPClient) *TelegramChannel {
	return &TelegramChannel{
		channel:    channel,
		httpClient: httpClient,
		apiURL:     telegramAPIURL,
	}
}

// NewTelegramChannelWithURL creates a new Telegram channel with a custom API URL (for testing)
func NewTelegramChannelWithURL(channel *NotificationChannel, httpClient HTTPClient, apiURL string) *TelegramChannel {
	return &TelegramChannel{
		channel:    channel,
		httpClient: httpClient,
		apiURL:     apiURL,
	}
}

// Send sends a notification via Telegram bot
func (t *TelegramChannel) Send(ctx context.Context, notif Notification) error {
	botToken := t.channel.GetTelegramBotToken()
	chatID := t.channel.GetTelegramChatID()

	if botToken == "" || chatID == "" {
		return ErrInvalidConfiguration
	}

	if t.httpClient == nil {
		return ErrSendFailed
	}

	url := fmt.Sprintf("%s%s/sendMessage", t.apiURL, botToken)

	payload := telegramMessage{
		ChatID:    chatID,
		Text:      notif.ToTelegram(),
		ParseMode: "HTML",
	}

	_, statusCode, err := t.httpClient.Post(ctx, url, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: received status code %d", ErrSendFailed, statusCode)
	}

	return nil
}

// Connect tests the Telegram bot connection
func (t *TelegramChannel) Connect(ctx context.Context) error {
	botToken := t.channel.GetTelegramBotToken()
	chatID := t.channel.GetTelegramChatID()

	if botToken == "" || chatID == "" {
		return ErrInvalidConfiguration
	}

	if t.httpClient == nil {
		return ErrConnectionFailed
	}

	url := fmt.Sprintf("%s%s/sendMessage", t.apiURL, botToken)

	payload := telegramMessage{
		ChatID:    chatID,
		Text:      "This message confirms that you have connected your Telegram to Launch.",
		ParseMode: "HTML",
	}

	_, statusCode, err := t.httpClient.Post(ctx, url, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: received status code %d", ErrConnectionFailed, statusCode)
	}

	return nil
}

// GetCreateRules returns the validation rules for creating a Telegram channel
func (t *TelegramChannel) GetCreateRules() map[string]string {
	return map[string]string{
		"bot_token":      "required",
		"chat_id":        "required",
		"appDeploy":      "boolean",
		"databaseBackup": "boolean",
	}
}

// GetData returns the Telegram channel data
func (t *TelegramChannel) GetData() ChannelData {
	return t.channel.Data
}
