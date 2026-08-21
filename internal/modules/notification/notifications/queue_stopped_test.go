package notifications

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

func TestQueueStoppedNotification(t *testing.T) {
	notification := NewQueueStoppedNotification("example.com", "Production", []string{"default", "emails"}).
		WithQueuesURL("https://launch.test/servers/server-1/sites/site-1?tab=queues")

	assert.Equal(t, notificationtypes.NotificationTypeQueueStopped, notification.Type())
	assert.Contains(t, notification.RawText(), "2 queue workers stopped")

	email := notification.ToEmail()
	require.NotNil(t, email)
	assert.Equal(t, "Queue Worker Stopped", email.Subject)
	assert.True(t, email.IsHTML)
	assert.Contains(t, email.Body, "default")
	assert.Contains(t, email.Body, "emails")
	assert.Contains(t, email.Body, "tab=queues")
	assert.Contains(t, notification.ToSlack(), "default, emails")
	assert.Equal(t, notification.ToSlack(), notification.ToDiscord())
	assert.Contains(t, notification.ToTelegram(), "Queue Worker Stopped")
}

func TestQueueStoppedNotificationSingularRawText(t *testing.T) {
	notification := NewQueueStoppedNotification("example.com", "Production", []string{"default"})
	assert.Contains(t, notification.RawText(), "1 queue worker stopped")
	assert.False(t, strings.Contains(notification.RawText(), "workers stopped"))
	assert.NotContains(t, notification.ToEmail().Body, "Manage Queue Workers")
}

func TestQueueStoppedNotificationFallsBackToPlainText(t *testing.T) {
	originalBuilder := buildQueueStoppedHTML
	buildQueueStoppedHTML = func(_ *templates.EmailBuilder) (string, error) {
		return "", errors.New("render failed")
	}
	t.Cleanup(func() { buildQueueStoppedHTML = originalBuilder })

	notification := NewQueueStoppedNotification("example.com", "Production", []string{"default"}).
		WithQueuesURL("https://launch.test/queues")
	email := notification.ToEmail()
	assert.False(t, email.IsHTML)
	assert.Contains(t, email.Body, "Manage Queue Workers: https://launch.test/queues")
}
