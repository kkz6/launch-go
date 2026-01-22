package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

// PhpExtensionUninstallFailedNotification is sent when PHP extension removal fails
type PhpExtensionUninstallFailedNotification struct {
	*models.BaseNotification
	ServerName    string
	ExtensionName string
	PhpVersion    string
	Output        string
	ErrorMessage  string
}

// NewPhpExtensionUninstallFailedNotification creates a new PHP extension uninstall failed notification
func NewPhpExtensionUninstallFailedNotification(serverName, extensionName, phpVersion, output, errorMessage string) *PhpExtensionUninstallFailedNotification {
	rawText := fmt.Sprintf("PHP extension '%s' removal failed on server '%s' (PHP %s).", extensionName, serverName, phpVersion)
	return &PhpExtensionUninstallFailedNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypePhpExtensionUninstallFailed, rawText),
		ServerName:       serverName,
		ExtensionName:    extensionName,
		PhpVersion:       phpVersion,
		Output:           output,
		ErrorMessage:     errorMessage,
	}
}

// ToEmail returns the email message content
func (n *PhpExtensionUninstallFailedNotification) ToEmail() *channels.EmailMessage {
	body := fmt.Sprintf("PHP extension '%s' removal failed on server '%s' (PHP %s).", n.ExtensionName, n.ServerName, n.PhpVersion)

	if n.Output != "" {
		body += fmt.Sprintf("\n\nHere you'll find the last lines of the task that failed:\n\n%s", n.Output)
	}

	if n.ErrorMessage != "" {
		body += fmt.Sprintf("\n\nThis is the error message we received:\n\n%s", n.ErrorMessage)
	}

	return &channels.EmailMessage{
		Subject: "PHP Extension Removal Failed",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *PhpExtensionUninstallFailedNotification) ToSlack() string {
	return n.formatWithLogs()
}

// ToDiscord returns the Discord message content
func (n *PhpExtensionUninstallFailedNotification) ToDiscord() string {
	return n.formatWithLogs()
}

// ToTelegram returns the Telegram message content
func (n *PhpExtensionUninstallFailedNotification) ToTelegram() string {
	return n.formatWithLogs()
}

func (n *PhpExtensionUninstallFailedNotification) formatWithLogs() string {
	message := "*PHP Extension Removal Failed*\n\n"
	message += fmt.Sprintf("PHP extension '%s' removal failed on server '%s' (PHP %s).", n.ExtensionName, n.ServerName, n.PhpVersion)

	if n.Output != "" {
		message += fmt.Sprintf("\n\n*Last lines of output:*\n```\n%s\n```", n.Output)
	}

	if n.ErrorMessage != "" {
		message += fmt.Sprintf("\n\n*Error message:*\n```\n%s\n```", n.ErrorMessage)
	}

	return message
}
