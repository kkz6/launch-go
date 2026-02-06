package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// JobOnServerFailedNotification is sent when a generic job fails on a server
type JobOnServerFailedNotification struct {
	*models.BaseNotification
	ServerName string
	ServerID   string
	Reference  string
	ServerURL  string
}

// NewJobOnServerFailedNotification creates a new job on server failed notification
func NewJobOnServerFailedNotification(serverName, serverID, reference string) *JobOnServerFailedNotification {
	rawText := "Job on server failed. We tried to run a job on your server, but it failed."
	return &JobOnServerFailedNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypeJobOnServerFailed, rawText),
		ServerName:       serverName,
		ServerID:         serverID,
		Reference:        reference,
	}
}

// WithServerURL sets the server URL for the action button
func (n *JobOnServerFailedNotification) WithServerURL(url string) *JobOnServerFailedNotification {
	n.ServerURL = url
	return n
}

// ToEmail returns the email message content
func (n *JobOnServerFailedNotification) ToEmail() *channels.EmailMessage {
	builder := templates.NewEmail().
		WithGreeting("Job on Server Failed").
		WithIntro(fmt.Sprintf("We tried to run a job on your server **%s**, but it failed.", n.ServerName)).
		WithIntro("Here's what we tried to do:").
		WithPanel(n.Reference)

	if n.ServerURL != "" {
		builder.WithAction("View Server", n.ServerURL, "error")
	}

	html, err := builder.Build()
	if err != nil {
		return n.plainTextEmail()
	}

	return &channels.EmailMessage{
		Subject: "Job on Server Failed",
		Body:    html,
		IsHTML:  true,
	}
}

func (n *JobOnServerFailedNotification) plainTextEmail() *channels.EmailMessage {
	body := fmt.Sprintf("We tried to run a job on your server '%s', but it failed.\n\nHere's what we tried to do:\n\n%s", n.ServerName, n.Reference)

	return &channels.EmailMessage{
		Subject: "Job on server failed",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *JobOnServerFailedNotification) ToSlack() string {
	return fmt.Sprintf("*🚨 Job on Server Failed*\n\nWe tried to run a job on your server '%s', but it failed.\n\n*What we tried to do:*\n%s", n.ServerName, n.Reference)
}

// ToDiscord returns the Discord message content
func (n *JobOnServerFailedNotification) ToDiscord() string {
	return n.ToSlack()
}

// ToTelegram returns the Telegram message content
func (n *JobOnServerFailedNotification) ToTelegram() string {
	return fmt.Sprintf("<b>🚨 Job on Server Failed</b>\n\nWe tried to run a job on your server '%s', but it failed.\n\n<b>What we tried to do:</b>\n%s", n.ServerName, n.Reference)
}
