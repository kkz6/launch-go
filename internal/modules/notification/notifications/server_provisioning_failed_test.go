package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/notification/enums"
)

func TestNewServerProvisioningFailedNotification(t *testing.T) {
	notif := NewServerProvisioningFailedNotification("my-server", "some output", "connection timeout")

	require.NotNil(t, notif)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, "some output", notif.Output)
	assert.Equal(t, "connection timeout", notif.ErrorMessage)
	assert.Equal(t, enums.NotificationTypeServerProvisioningFailed, notif.Type())
	assert.Contains(t, notif.RawText(), "my-server")
	assert.Contains(t, notif.RawText(), "failed to provision")
}

func TestServerProvisioningFailedNotification_ToEmail(t *testing.T) {
	tests := []struct {
		name         string
		serverName   string
		output       string
		errorMessage string
		wantInBody   []string
	}{
		{
			name:         "with output and error",
			serverName:   "my-server",
			output:       "apt-get failed",
			errorMessage: "connection timeout",
			wantInBody:   []string{"my-server", "apt-get failed", "connection timeout"},
		},
		{
			name:         "without output",
			serverName:   "my-server",
			output:       "",
			errorMessage: "connection timeout",
			wantInBody:   []string{"my-server", "connection timeout"},
		},
		{
			name:         "without error message",
			serverName:   "my-server",
			output:       "apt-get failed",
			errorMessage: "",
			wantInBody:   []string{"my-server", "apt-get failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notif := NewServerProvisioningFailedNotification(tt.serverName, tt.output, tt.errorMessage)
			email := notif.ToEmail()

			require.NotNil(t, email)
			assert.Equal(t, "Server Provisioning Failed", email.Subject)
			assert.False(t, email.IsHTML)

			for _, want := range tt.wantInBody {
				assert.Contains(t, email.Body, want)
			}
		})
	}
}

func TestServerProvisioningFailedNotification_ToSlack(t *testing.T) {
	notif := NewServerProvisioningFailedNotification("my-server", "some output", "error msg")

	message := notif.ToSlack()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "🚨")
	assert.Contains(t, message, "Server Provisioning Failed")
	assert.Contains(t, message, "some output")
	assert.Contains(t, message, "error msg")
}

func TestServerProvisioningFailedNotification_ToDiscord(t *testing.T) {
	notif := NewServerProvisioningFailedNotification("my-server", "output", "error")

	message := notif.ToDiscord()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "output")
}

func TestServerProvisioningFailedNotification_ToTelegram(t *testing.T) {
	notif := NewServerProvisioningFailedNotification("my-server", "output", "error")

	message := notif.ToTelegram()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "output")
}
