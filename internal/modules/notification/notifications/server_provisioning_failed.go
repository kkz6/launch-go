package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

// ServerProvisioningFailedNotification is sent when server provisioning fails
type ServerProvisioningFailedNotification struct {
	*models.BaseNotification
	ServerName   string
	Output       string
	ErrorMessage string
}

// NewServerProvisioningFailedNotification creates a new server provisioning failed notification
func NewServerProvisioningFailedNotification(serverName, output, errorMessage string) *ServerProvisioningFailedNotification {
	rawText := fmt.Sprintf("The server '%s' failed to provision. You might need to manually remove it from Launch and from your provider for safety reasons.", serverName)
	return &ServerProvisioningFailedNotification{
		BaseNotification: models.NewBaseNotification(enums.NotificationTypeServerProvisioningFailed, rawText),
		ServerName:       serverName,
		Output:           output,
		ErrorMessage:     errorMessage,
	}
}

// ToEmail returns the email message content
func (n *ServerProvisioningFailedNotification) ToEmail() *channels.EmailMessage {
	body := fmt.Sprintf("The server '%s' failed to provision. You might need to manually remove it from Launch and from your provider for safety reasons.", n.ServerName)

	if n.Output != "" {
		body += fmt.Sprintf("\n\nHere you'll find the last lines of the task that failed:\n\n%s", n.Output)
	}

	if n.ErrorMessage != "" {
		body += fmt.Sprintf("\n\nThis is the error message we received:\n\n%s", n.ErrorMessage)
	}

	return &channels.EmailMessage{
		Subject: "Server Provisioning Failed",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *ServerProvisioningFailedNotification) ToSlack() string {
	return n.formatWithLogs()
}

// ToDiscord returns the Discord message content
func (n *ServerProvisioningFailedNotification) ToDiscord() string {
	return n.formatWithLogs()
}

// ToTelegram returns the Telegram message content
func (n *ServerProvisioningFailedNotification) ToTelegram() string {
	return n.formatWithLogs()
}

func (n *ServerProvisioningFailedNotification) formatWithLogs() string {
	message := "*🚨 Server Provisioning Failed*\n\n"
	message += fmt.Sprintf("The server '%s' failed to provision. You might need to manually remove it from Launch and from your provider for safety reasons.", n.ServerName)

	if n.Output != "" {
		message += fmt.Sprintf("\n\n*Last lines of output:*\n```\n%s\n```", n.Output)
	}

	if n.ErrorMessage != "" {
		message += fmt.Sprintf("\n\n*Error message:*\n```\n%s\n```", n.ErrorMessage)
	}

	return message
}
