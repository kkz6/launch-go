package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewServerProvisionedNotification(t *testing.T) {
	notif := NewServerProvisionedNotification("my-server", "192.168.1.100")

	require.NotNil(t, notif)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, "192.168.1.100", notif.ServerIP)
	assert.Equal(t, notificationtypes.NotificationTypeServerProvisioned, notif.Type())
	assert.Contains(t, notif.RawText(), "my-server")
	assert.Contains(t, notif.RawText(), "provisioned")
}

func TestServerProvisionedNotification_ToEmail(t *testing.T) {
	notif := NewServerProvisionedNotification("my-server", "192.168.1.100")

	email := notif.ToEmail()

	require.NotNil(t, email)
	assert.Equal(t, "Server Provisioned", email.Subject)
	assert.Contains(t, email.Body, "my-server")
	assert.Contains(t, email.Body, "192.168.1.100")
	assert.Contains(t, email.Body, "provisioned")
	assert.False(t, email.IsHTML)
}

func TestServerProvisionedNotification_ToSlack(t *testing.T) {
	notif := NewServerProvisionedNotification("my-server", "192.168.1.100")

	message := notif.ToSlack()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "192.168.1.100")
	assert.Contains(t, message, "✅")
	assert.Contains(t, message, "Server Provisioned")
}

func TestServerProvisionedNotification_ToDiscord(t *testing.T) {
	notif := NewServerProvisionedNotification("my-server", "192.168.1.100")

	message := notif.ToDiscord()

	// Discord uses the same format as Slack
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "192.168.1.100")
}

func TestServerProvisionedNotification_ToTelegram(t *testing.T) {
	notif := NewServerProvisionedNotification("my-server", "192.168.1.100")

	message := notif.ToTelegram()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "192.168.1.100")
	assert.Contains(t, message, "<b>")
	assert.Contains(t, message, "</b>")
}
