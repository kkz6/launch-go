package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewJobOnServerFailedNotification(t *testing.T) {
	notif := NewJobOnServerFailedNotification("my-server", "server-123", "Install PHP extensions")

	require.NotNil(t, notif)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, "server-123", notif.ServerID)
	assert.Equal(t, "Install PHP extensions", notif.Reference)
	assert.Equal(t, notificationtypes.NotificationTypeJobOnServerFailed, notif.Type())
	assert.Contains(t, notif.RawText(), "Job on server failed")
}

func TestJobOnServerFailedNotification_ToEmail(t *testing.T) {
	notif := NewJobOnServerFailedNotification("my-server", "server-123", "Install PHP extensions")

	email := notif.ToEmail()

	require.NotNil(t, email)
	assert.Equal(t, "Job on server failed", email.Subject)
	assert.Contains(t, email.Body, "my-server")
	assert.Contains(t, email.Body, "Install PHP extensions")
	assert.False(t, email.IsHTML)
}

func TestJobOnServerFailedNotification_ToSlack(t *testing.T) {
	notif := NewJobOnServerFailedNotification("my-server", "server-123", "Install PHP extensions")

	message := notif.ToSlack()

	assert.Contains(t, message, "🚨")
	assert.Contains(t, message, "Job on Server Failed")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "Install PHP extensions")
}

func TestJobOnServerFailedNotification_ToDiscord(t *testing.T) {
	notif := NewJobOnServerFailedNotification("my-server", "server-123", "Install PHP extensions")

	message := notif.ToDiscord()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "Install PHP extensions")
}

func TestJobOnServerFailedNotification_ToTelegram(t *testing.T) {
	notif := NewJobOnServerFailedNotification("my-server", "server-123", "Install PHP extensions")

	message := notif.ToTelegram()

	assert.Contains(t, message, "<b>")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "Install PHP extensions")
}
