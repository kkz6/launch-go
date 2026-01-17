package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

// PhpInstallationFailedNotification is sent when PHP installation fails
type PhpInstallationFailedNotification struct {
	*models.BaseNotification
	ServerName   string
	PhpVersion   string
	Output       string
	ErrorMessage string
}

// NewPhpInstallationFailedNotification creates a new PHP installation failed notification
func NewPhpInstallationFailedNotification(serverName, phpVersion, output, errorMessage string) *PhpInstallationFailedNotification {
	rawText := fmt.Sprintf("PHP %s installation failed on server '%s'.", phpVersion, serverName)
	return &PhpInstallationFailedNotification{
		BaseNotification: models.NewBaseNotification(enums.NotificationTypePhpInstallationFailed, rawText),
		ServerName:       serverName,
		PhpVersion:       phpVersion,
		Output:           output,
		ErrorMessage:     errorMessage,
	}
}

// ToEmail returns the email message content
func (n *PhpInstallationFailedNotification) ToEmail() *channels.EmailMessage {
	body := fmt.Sprintf("PHP %s installation failed on server '%s'.", n.PhpVersion, n.ServerName)

	if n.Output != "" {
		body += fmt.Sprintf("\n\nHere you'll find the last lines of the task that failed:\n\n%s", n.Output)
	}

	if n.ErrorMessage != "" {
		body += fmt.Sprintf("\n\nThis is the error message we received:\n\n%s", n.ErrorMessage)
	}

	return &channels.EmailMessage{
		Subject: "PHP Installation Failed",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *PhpInstallationFailedNotification) ToSlack() string {
	return n.formatWithLogs()
}

// ToDiscord returns the Discord message content
func (n *PhpInstallationFailedNotification) ToDiscord() string {
	return n.formatWithLogs()
}

// ToTelegram returns the Telegram message content
func (n *PhpInstallationFailedNotification) ToTelegram() string {
	return n.formatWithLogs()
}

func (n *PhpInstallationFailedNotification) formatWithLogs() string {
	message := "*🚨 PHP Installation Failed*\n\n"
	message += fmt.Sprintf("PHP %s installation failed on server '%s'.", n.PhpVersion, n.ServerName)

	if n.Output != "" {
		message += fmt.Sprintf("\n\n*Last lines of output:*\n```\n%s\n```", n.Output)
	}

	if n.ErrorMessage != "" {
		message += fmt.Sprintf("\n\n*Error message:*\n```\n%s\n```", n.ErrorMessage)
	}

	return message
}
