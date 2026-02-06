package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// ServerProvisionedNotification is sent when a server is successfully provisioned
type ServerProvisionedNotification struct {
	*models.BaseNotification
	ServerName       string
	ServerIP         string
	ServerUsername   string
	DatabasePassword string
	DashboardURL     string
}

// NewServerProvisionedNotification creates a new server provisioned notification
func NewServerProvisionedNotification(serverName, serverIP, serverUsername string) *ServerProvisionedNotification {
	rawText := fmt.Sprintf("Your server '%s' has been provisioned and is ready to use.", serverName)
	return &ServerProvisionedNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypeServerProvisioned, rawText),
		ServerName:       serverName,
		ServerIP:         serverIP,
		ServerUsername:   serverUsername,
	}
}

// WithServerUsername sets the server username
func (n *ServerProvisionedNotification) WithServerUsername(username string) *ServerProvisionedNotification {
	n.ServerUsername = username
	return n
}

// WithDatabasePassword sets the database password for display
func (n *ServerProvisionedNotification) WithDatabasePassword(password string) *ServerProvisionedNotification {
	n.DatabasePassword = password
	return n
}

// WithDashboardURL sets the dashboard URL for the button
func (n *ServerProvisionedNotification) WithDashboardURL(url string) *ServerProvisionedNotification {
	n.DashboardURL = url
	return n
}

// ToEmail returns the email message content
func (n *ServerProvisionedNotification) ToEmail() *channels.EmailMessage {
	html, _, err := templates.ServerProvisionedEmail(
		n.ServerName,
		n.ServerIP,
		n.ServerUsername,
		n.DashboardURL,
		n.DatabasePassword,
	)

	if err != nil {
		// Fallback to plain text if template fails
		return n.plainTextEmail()
	}

	return &channels.EmailMessage{
		Subject: "Server Provisioned Successfully",
		Body:    html,
		IsHTML:  true,
	}
}

func (n *ServerProvisionedNotification) plainTextEmail() *channels.EmailMessage {
	body := fmt.Sprintf(`Your server '%s' has been provisioned and is ready to use.

Server Details:
• Name: %s
• IP Address: %s
• Username: %s

You can now start deploying your applications to this server.`,
		n.ServerName, n.ServerName, n.ServerIP, n.ServerUsername)

	return &channels.EmailMessage{
		Subject: "Server Provisioned",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *ServerProvisionedNotification) ToSlack() string {
	return fmt.Sprintf("*✅ Server Provisioned*\n\nYour server '%s' has been provisioned and is ready to use.\n\n*Server IP:* %s",
		n.ServerName, n.ServerIP)
}

// ToDiscord returns the Discord message content
func (n *ServerProvisionedNotification) ToDiscord() string {
	return n.ToSlack()
}

// ToTelegram returns the Telegram message content
func (n *ServerProvisionedNotification) ToTelegram() string {
	return fmt.Sprintf("<b>✅ Server Provisioned</b>\n\nYour server '%s' has been provisioned and is ready to use.\n\n<b>Server IP:</b> %s",
		n.ServerName, n.ServerIP)
}
