package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// ServerConnectionLostNotification is sent when SSH connection to a server is lost
type ServerConnectionLostNotification struct {
	*models.BaseNotification
	ServerName string
	ServerIP   string
	ServerURL  string
}

// NewServerConnectionLostNotification creates a new server connection lost notification
func NewServerConnectionLostNotification(serverName, serverIP string) *ServerConnectionLostNotification {
	rawText := fmt.Sprintf("Connection lost to server '%s'. We were unable to connect to the server via SSH.", serverName)
	return &ServerConnectionLostNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypeServerConnectionLost, rawText),
		ServerName:       serverName,
		ServerIP:         serverIP,
	}
}

// WithServerURL sets the server URL for the action button
func (n *ServerConnectionLostNotification) WithServerURL(url string) *ServerConnectionLostNotification {
	n.ServerURL = url
	return n
}

// ToEmail returns the email message content
func (n *ServerConnectionLostNotification) ToEmail() *channels.EmailMessage {
	builder := templates.NewEmail().
		WithGreeting("Server Connection Lost").
		WithIntro(fmt.Sprintf("Connection lost to server **%s**.", n.ServerName)).
		WithIntro("We were unable to connect to the server via SSH. This could be due to:")

	reasons := `- Server being offline or unreachable
- SSH service not running
- Firewall blocking the connection
- SSH key issues`
	builder.WithPanel(reasons)

	details := fmt.Sprintf(`**Server Details:**

- **Name:** %s
- **IP Address:** %s`, n.ServerName, n.ServerIP)
	builder.WithPanel(details)

	builder.WithOutro("Please check your server's status and connectivity.")

	if n.ServerURL != "" {
		builder.WithAction("View Server", n.ServerURL, "error")
	}

	html, err := builder.Build()
	if err != nil {
		return n.plainTextEmail()
	}

	return &channels.EmailMessage{
		Subject: "Server Connection Lost",
		Body:    html,
		IsHTML:  true,
	}
}

func (n *ServerConnectionLostNotification) plainTextEmail() *channels.EmailMessage {
	body := fmt.Sprintf(`Connection lost to server '%s'.

We were unable to connect to the server via SSH. This could be due to:
- Server being offline or unreachable
- SSH service not running
- Firewall blocking the connection
- SSH key issues

Server Details:
• Name: %s
• IP Address: %s

Please check your server's status and connectivity.`,
		n.ServerName, n.ServerName, n.ServerIP)

	return &channels.EmailMessage{
		Subject: "Server Connection Lost",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *ServerConnectionLostNotification) ToSlack() string {
	return fmt.Sprintf("*⚠️ Server Connection Lost*\n\nConnection lost to server '%s' (%s).\n\nWe were unable to connect to the server via SSH. Please check your server's status.", n.ServerName, n.ServerIP)
}

// ToDiscord returns the Discord message content
func (n *ServerConnectionLostNotification) ToDiscord() string {
	return n.ToSlack()
}

// ToTelegram returns the Telegram message content
func (n *ServerConnectionLostNotification) ToTelegram() string {
	return fmt.Sprintf("<b>⚠️ Server Connection Lost</b>\n\nConnection lost to server '%s' (%s).\n\nWe were unable to connect to the server via SSH. Please check your server's status.", n.ServerName, n.ServerIP)
}
