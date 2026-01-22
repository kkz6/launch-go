package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

// PhpExtensionInstallFailedNotification is sent when PHP extension installation fails
type PhpExtensionInstallFailedNotification struct {
	*models.BaseNotification
	ServerName    string
	ExtensionName string
	PhpVersion    string
	Output        string
	ErrorMessage  string
}

// NewPhpExtensionInstallFailedNotification creates a new PHP extension install failed notification
func NewPhpExtensionInstallFailedNotification(serverName, extensionName, phpVersion, output, errorMessage string) *PhpExtensionInstallFailedNotification {
	rawText := fmt.Sprintf("PHP extension '%s' installation failed on server '%s' (PHP %s).", extensionName, serverName, phpVersion)
	return &PhpExtensionInstallFailedNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypePhpExtensionInstallFailed, rawText),
		ServerName:       serverName,
		ExtensionName:    extensionName,
		PhpVersion:       phpVersion,
		Output:           output,
		ErrorMessage:     errorMessage,
	}
}

// ToEmail returns the email message content
func (n *PhpExtensionInstallFailedNotification) ToEmail() *channels.EmailMessage {
	body := fmt.Sprintf("PHP extension '%s' installation failed on server '%s' (PHP %s).", n.ExtensionName, n.ServerName, n.PhpVersion)

	if n.Output != "" {
		body += fmt.Sprintf("\n\nHere you'll find the last lines of the task that failed:\n\n%s", n.Output)
	}

	if n.ErrorMessage != "" {
		body += fmt.Sprintf("\n\nThis is the error message we received:\n\n%s", n.ErrorMessage)
	}

	return &channels.EmailMessage{
		Subject: "PHP Extension Installation Failed",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *PhpExtensionInstallFailedNotification) ToSlack() string {
	return n.formatWithLogs()
}

// ToDiscord returns the Discord message content
func (n *PhpExtensionInstallFailedNotification) ToDiscord() string {
	return n.formatWithLogs()
}

// ToTelegram returns the Telegram message content
func (n *PhpExtensionInstallFailedNotification) ToTelegram() string {
	return n.formatWithLogs()
}

func (n *PhpExtensionInstallFailedNotification) formatWithLogs() string {
	message := "*PHP Extension Installation Failed*\n\n"
	message += fmt.Sprintf("PHP extension '%s' installation failed on server '%s' (PHP %s).", n.ExtensionName, n.ServerName, n.PhpVersion)

	if n.Output != "" {
		message += fmt.Sprintf("\n\n*Last lines of output:*\n```\n%s\n```", n.Output)
	}

	if n.ErrorMessage != "" {
		message += fmt.Sprintf("\n\n*Error message:*\n```\n%s\n```", n.ErrorMessage)
	}

	return message
}
