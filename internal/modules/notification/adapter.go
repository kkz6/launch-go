package notification

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Ensure NotifierAdapter satisfies the taskrunner interface at compile time.
var _ taskrunner.NotifierService = (*NotifierAdapter)(nil)

// NotifierAdapter adapts the notification module's Notifier to the
// taskrunner.NotifierService interface. It wraps minimal taskrunner.Notification
// values into full models.Notification before forwarding to the real Notifier.
type NotifierAdapter struct {
	notifier *services.Notifier
}

// NewNotifierAdapter creates a new adapter around the notification module's Notifier.
func NewNotifierAdapter(notifier *services.Notifier) *NotifierAdapter {
	return &NotifierAdapter{notifier: notifier}
}

// SendToTeam sends a notification to all connected channels for a team.
func (a *NotifierAdapter) SendToTeam(ctx context.Context, teamID string, notification taskrunner.Notification) error {
	return a.notifier.SendToTeam(ctx, teamID, wrapNotification(notification))
}

// SendToChannel sends a notification to a specific channel.
func (a *NotifierAdapter) SendToChannel(ctx context.Context, channelID string, notification taskrunner.Notification) error {
	return a.notifier.SendToChannel(ctx, channelID, wrapNotification(notification))
}

// wrapNotification converts a taskrunner.Notification into a models.Notification.
// If the notification already satisfies models.Notification it is returned as-is;
// otherwise it is wrapped in a taskrunnerNotification adapter.
func wrapNotification(n taskrunner.Notification) models.Notification {
	if full, ok := n.(models.Notification); ok {
		return full
	}
	return &taskrunnerNotification{notification: n}
}

// taskrunnerNotification wraps a taskrunner.Notification to satisfy models.Notification.
// All channel-specific methods fall back to the plain RawText value.
type taskrunnerNotification struct {
	notification taskrunner.Notification
}

var _ models.Notification = (*taskrunnerNotification)(nil)

func (n *taskrunnerNotification) Type() notificationtypes.NotificationType {
	return notificationtypes.NotificationType("task")
}

func (n *taskrunnerNotification) RawText() string {
	return n.notification.RawText()
}

func (n *taskrunnerNotification) ToEmail() *channels.EmailMessage {
	return &channels.EmailMessage{
		Subject: "Launch Notification",
		Body:    n.notification.RawText(),
		IsHTML:  false,
	}
}

func (n *taskrunnerNotification) ToSlack() string {
	return n.notification.RawText()
}

func (n *taskrunnerNotification) ToDiscord() string {
	return n.notification.RawText()
}

func (n *taskrunnerNotification) ToTelegram() string {
	return n.notification.RawText()
}
