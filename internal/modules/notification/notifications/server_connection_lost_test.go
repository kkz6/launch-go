package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewServerConnectionLostNotification(t *testing.T) {
	notif := NewServerConnectionLostNotification("my-server", "192.168.1.100")

	require.NotNil(t, notif)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, "192.168.1.100", notif.ServerIP)
	assert.Equal(t, notificationtypes.NotificationTypeServerConnectionLost, notif.Type())
	assert.Contains(t, notif.RawText(), "my-server")
	assert.Contains(t, notif.RawText(), "Connection lost")
}

func TestServerConnectionLostNotification_ToEmail(t *testing.T) {
	notif := NewServerConnectionLostNotification("my-server", "192.168.1.100")

	email := notif.ToEmail()

	require.NotNil(t, email)
	assert.Equal(t, "Server Connection Lost", email.Subject)
	assert.Contains(t, email.Body, "my-server")
	assert.Contains(t, email.Body, "192.168.1.100")
	assert.Contains(t, email.Body, "SSH")
	assert.Contains(t, email.Body, "offline")
	assert.True(t, email.IsHTML)
}

func TestServerConnectionLostNotification_ToSlack(t *testing.T) {
	notif := NewServerConnectionLostNotification("my-server", "192.168.1.100")

	message := notif.ToSlack()

	assert.Contains(t, message, "⚠️")
	assert.Contains(t, message, "Server Connection Lost")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "192.168.1.100")
	assert.Contains(t, message, "SSH")
}

func TestServerConnectionLostNotification_ToDiscord(t *testing.T) {
	notif := NewServerConnectionLostNotification("my-server", "192.168.1.100")

	message := notif.ToDiscord()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "192.168.1.100")
}

func TestServerConnectionLostNotification_ToTelegram(t *testing.T) {
	notif := NewServerConnectionLostNotification("my-server", "192.168.1.100")

	message := notif.ToTelegram()

	assert.Contains(t, message, "<b>")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "192.168.1.100")
}
