package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

// FailedToDeleteServerNotification is sent when server deletion from provider fails
type FailedToDeleteServerNotification struct {
	*models.BaseNotification
	ServerName   string
	Provider     string
	ErrorMessage string
}

// NewFailedToDeleteServerNotification creates a new failed to delete server notification
func NewFailedToDeleteServerNotification(serverName, provider, errorMessage string) *FailedToDeleteServerNotification {
	rawText := fmt.Sprintf("Failed to delete server '%s' from %s. You may need to manually delete it from the provider.", serverName, provider)
	return &FailedToDeleteServerNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypeFailedToDeleteServer, rawText),
		ServerName:       serverName,
		Provider:         provider,
		ErrorMessage:     errorMessage,
	}
}

// ToEmail returns the email message content
func (n *FailedToDeleteServerNotification) ToEmail() *channels.EmailMessage {
	body := fmt.Sprintf(`Failed to delete server '%s' from %s.

The server has been removed from Launch, but we were unable to delete it from your cloud provider. You may need to manually delete it from your %s account to avoid additional charges.

Server: %s
Provider: %s`,
		n.ServerName, n.Provider, n.Provider, n.ServerName, n.Provider)

	if n.ErrorMessage != "" {
		body += fmt.Sprintf("\n\nError: %s", n.ErrorMessage)
	}

	return &channels.EmailMessage{
		Subject: "Failed to Delete Server from Provider",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *FailedToDeleteServerNotification) ToSlack() string {
	message := fmt.Sprintf("*⚠️ Failed to Delete Server from Provider*\n\nFailed to delete server '%s' from %s.\n\nYou may need to manually delete it from your provider to avoid additional charges.", n.ServerName, n.Provider)

	if n.ErrorMessage != "" {
		message += fmt.Sprintf("\n\n*Error:* %s", n.ErrorMessage)
	}

	return message
}

// ToDiscord returns the Discord message content
func (n *FailedToDeleteServerNotification) ToDiscord() string {
	return n.ToSlack()
}

// ToTelegram returns the Telegram message content
func (n *FailedToDeleteServerNotification) ToTelegram() string {
	message := fmt.Sprintf("<b>⚠️ Failed to Delete Server from Provider</b>\n\nFailed to delete server '%s' from %s.\n\nYou may need to manually delete it from your provider to avoid additional charges.", n.ServerName, n.Provider)

	if n.ErrorMessage != "" {
		message += fmt.Sprintf("\n\n<b>Error:</b> %s", n.ErrorMessage)
	}

	return message
}
