package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewFailedToDeleteServerNotification(t *testing.T) {
	notif := NewFailedToDeleteServerNotification("my-server", "DigitalOcean", "API rate limit exceeded")

	require.NotNil(t, notif)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, "DigitalOcean", notif.Provider)
	assert.Equal(t, "API rate limit exceeded", notif.ErrorMessage)
	assert.Equal(t, notificationtypes.NotificationTypeFailedToDeleteServer, notif.Type())
	assert.Contains(t, notif.RawText(), "my-server")
	assert.Contains(t, notif.RawText(), "DigitalOcean")
}

func TestFailedToDeleteServerNotification_ToEmail(t *testing.T) {
	tests := []struct {
		name         string
		serverName   string
		provider     string
		errorMessage string
		wantInBody   []string
	}{
		{
			name:         "with error message",
			serverName:   "my-server",
			provider:     "DigitalOcean",
			errorMessage: "API rate limit exceeded",
			wantInBody:   []string{"my-server", "DigitalOcean", "API rate limit exceeded", "manually delete"},
		},
		{
			name:         "without error message",
			serverName:   "my-server",
			provider:     "AWS",
			errorMessage: "",
			wantInBody:   []string{"my-server", "AWS", "manually delete"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notif := NewFailedToDeleteServerNotification(tt.serverName, tt.provider, tt.errorMessage)
			email := notif.ToEmail()

			require.NotNil(t, email)
			assert.Equal(t, "Failed to Delete Server from Provider", email.Subject)
			assert.False(t, email.IsHTML)

			for _, want := range tt.wantInBody {
				assert.Contains(t, email.Body, want)
			}
		})
	}
}

func TestFailedToDeleteServerNotification_ToSlack(t *testing.T) {
	notif := NewFailedToDeleteServerNotification("my-server", "DigitalOcean", "API error")

	message := notif.ToSlack()

	assert.Contains(t, message, "⚠️")
	assert.Contains(t, message, "Failed to Delete Server")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "DigitalOcean")
	assert.Contains(t, message, "API error")
}

func TestFailedToDeleteServerNotification_ToSlack_WithoutError(t *testing.T) {
	notif := NewFailedToDeleteServerNotification("my-server", "DigitalOcean", "")

	message := notif.ToSlack()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "DigitalOcean")
	assert.NotContains(t, message, "*Error:*")
}

func TestFailedToDeleteServerNotification_ToDiscord(t *testing.T) {
	notif := NewFailedToDeleteServerNotification("my-server", "DigitalOcean", "error")

	message := notif.ToDiscord()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "DigitalOcean")
}

func TestFailedToDeleteServerNotification_ToTelegram(t *testing.T) {
	notif := NewFailedToDeleteServerNotification("my-server", "DigitalOcean", "error")

	message := notif.ToTelegram()

	assert.Contains(t, message, "<b>")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "DigitalOcean")
}
