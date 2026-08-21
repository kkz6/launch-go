package notifications

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// QueueStoppedNotification reports one or more stopped queue workers.
type QueueStoppedNotification struct {
	*models.BaseNotification
	SiteAddress string
	ServerName  string
	QueueNames  []string
	QueuesURL   string
}

var buildQueueStoppedHTML = func(builder *templates.EmailBuilder) (string, error) {
	return builder.Build()
}

// NewQueueStoppedNotification creates a stopped queue notification.
func NewQueueStoppedNotification(siteAddress, serverName string, queueNames []string) *QueueStoppedNotification {
	workerLabel := "queue worker"
	if len(queueNames) != 1 {
		workerLabel = "queue workers"
	}
	rawText := fmt.Sprintf("%d %s stopped for site '%s' on server '%s'.", len(queueNames), workerLabel, siteAddress, serverName)

	return &QueueStoppedNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypeQueueStopped, rawText),
		SiteAddress:      siteAddress,
		ServerName:       serverName,
		QueueNames:       append([]string(nil), queueNames...),
	}
}

// WithQueuesURL adds a direct link to the site's queue management tab.
func (n *QueueStoppedNotification) WithQueuesURL(url string) *QueueStoppedNotification {
	n.QueuesURL = url
	return n
}

// ToEmail returns the email message content.
func (n *QueueStoppedNotification) ToEmail() *channels.EmailMessage {
	queueList := "- " + strings.Join(n.QueueNames, "\n- ")
	builder := templates.NewEmail().
		WithContext("lctl / queues").
		WithState("QUEUE STOPPED", "error").
		WithGreeting("Queue Worker Stopped").
		WithIntro(fmt.Sprintf("One or more queue workers are not running for site **%s** on server **%s**.", n.SiteAddress, n.ServerName)).
		WithPanel("**Stopped workers:**\n\n" + queueList).
		WithOutro("Review the worker logs and configuration, restart the worker, then sync its status again.")

	if n.QueuesURL != "" {
		builder.WithAction("Manage Queue Workers", n.QueuesURL, "error")
	}

	html, err := buildQueueStoppedHTML(builder)
	if err != nil {
		return n.plainTextEmail()
	}

	return &channels.EmailMessage{Subject: "Queue Worker Stopped", Body: html, IsHTML: true}
}

func (n *QueueStoppedNotification) plainTextEmail() *channels.EmailMessage {
	body := fmt.Sprintf("Queue workers stopped for site '%s' on server '%s'.\n\nStopped workers:\n- %s\n\nReview the worker logs and configuration, restart the worker, then sync its status again.", n.SiteAddress, n.ServerName, strings.Join(n.QueueNames, "\n- "))
	if n.QueuesURL != "" {
		body += "\n\nManage Queue Workers: " + n.QueuesURL
	}
	return &channels.EmailMessage{Subject: "Queue Worker Stopped", Body: body, IsHTML: false}
}

// ToSlack returns the Slack message content.
func (n *QueueStoppedNotification) ToSlack() string {
	return fmt.Sprintf("*Queue Worker Stopped*\n\nSite: %s\nServer: %s\nStopped workers: %s\n\nManage queues: %s", n.SiteAddress, n.ServerName, strings.Join(n.QueueNames, ", "), n.QueuesURL)
}

// ToDiscord returns the Discord message content.
func (n *QueueStoppedNotification) ToDiscord() string { return n.ToSlack() }

// ToTelegram returns the Telegram message content.
func (n *QueueStoppedNotification) ToTelegram() string {
	return fmt.Sprintf("<b>Queue Worker Stopped</b>\n\nSite: %s\nServer: %s\nStopped workers: %s\n\nManage queues: %s", n.SiteAddress, n.ServerName, strings.Join(n.QueueNames, ", "), n.QueuesURL)
}
