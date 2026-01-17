package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

// ServerProvisionedNotification is sent when a server is successfully provisioned
type ServerProvisionedNotification struct {
	*models.BaseNotification
	ServerName string
	ServerIP   string
}

// NewServerProvisionedNotification creates a new server provisioned notification
func NewServerProvisionedNotification(serverName, serverIP string) *ServerProvisionedNotification {
	rawText := fmt.Sprintf("Your server '%s' has been provisioned and is ready to use.", serverName)
	return &ServerProvisionedNotification{
		BaseNotification: models.NewBaseNotification(enums.NotificationTypeServerProvisioned, rawText),
		ServerName:       serverName,
		ServerIP:         serverIP,
	}
}

// ToEmail returns the email message content
func (n *ServerProvisionedNotification) ToEmail() *channels.EmailMessage {
	body := fmt.Sprintf(`Your server '%s' has been provisioned and is ready to use.

Server Details:
• Name: %s
• IP Address: %s

You can now start deploying your applications to this server.`,
		n.ServerName, n.ServerName, n.ServerIP)

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
