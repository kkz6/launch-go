package channels

// ChannelType represents a notification channel type
type ChannelType string

const (
	ChannelTypeEmail    ChannelType = "email"
	ChannelTypeSlack    ChannelType = "slack"
	ChannelTypeDiscord  ChannelType = "discord"
	ChannelTypeTelegram ChannelType = "telegram"
)

// ChannelData represents the data stored in a notification channel
type ChannelData struct {
	Email          string `json:"email,omitempty"`
	WebhookURL     string `json:"webhook_url,omitempty"`
	BotToken       string `json:"bot_token,omitempty"`
	ChatID         string `json:"chat_id,omitempty"`
	AppDeploy      bool   `json:"appDeploy"`
	DatabaseBackup bool   `json:"databaseBackup"`
}

// NotificationChannel represents a channel configuration for sending notifications
type NotificationChannel struct {
	ID        string
	UserID    string
	TeamID    string
	Provider  ChannelType
	Label     string
	Data      ChannelData
	Connected bool
	IsDefault bool
}

// GetEmail returns the email from channel data
func (nc *NotificationChannel) GetEmail() string {
	return nc.Data.Email
}

// GetWebhookURL returns the webhook URL from channel data
func (nc *NotificationChannel) GetWebhookURL() string {
	return nc.Data.WebhookURL
}

// GetTelegramBotToken returns the Telegram bot token from channel data
func (nc *NotificationChannel) GetTelegramBotToken() string {
	return nc.Data.BotToken
}

// GetTelegramChatID returns the Telegram chat ID from channel data
func (nc *NotificationChannel) GetTelegramChatID() string {
	return nc.Data.ChatID
}

// EmailMessage represents an email notification message
type EmailMessage struct {
	Subject string
	Body    string
	IsHTML  bool
}

// Notification represents a notification that can be sent through channels
type Notification interface {
	// RawText returns the plain text representation
	RawText() string

	// ToEmail returns the email message content
	ToEmail() *EmailMessage

	// ToSlack returns the Slack message content
	ToSlack() string

	// ToDiscord returns the Discord message content
	ToDiscord() string

	// ToTelegram returns the Telegram message content
	ToTelegram() string
}

// MockNotification is a mock notification for testing
type MockNotification struct {
	text string
}

// NewMockNotification creates a new mock notification
func NewMockNotification(text string) *MockNotification {
	return &MockNotification{text: text}
}

// RawText returns the plain text representation
func (m *MockNotification) RawText() string {
	return m.text
}

// ToEmail returns the email message content
func (m *MockNotification) ToEmail() *EmailMessage {
	return &EmailMessage{
		Subject: "Notification",
		Body:    m.text,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (m *MockNotification) ToSlack() string {
	return m.text
}

// ToDiscord returns the Discord message content
func (m *MockNotification) ToDiscord() string {
	return m.text
}

// ToTelegram returns the Telegram message content
func (m *MockNotification) ToTelegram() string {
	return m.text
}
