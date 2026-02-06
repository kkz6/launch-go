package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewPhpInstallationFailedNotification(t *testing.T) {
	notif := NewPhpInstallationFailedNotification("my-server", "8.2", "apt-get output", "package not found")

	require.NotNil(t, notif)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, "8.2", notif.PhpVersion)
	assert.Equal(t, "apt-get output", notif.Output)
	assert.Equal(t, "package not found", notif.ErrorMessage)
	assert.Equal(t, notificationtypes.NotificationTypePhpInstallationFailed, notif.Type())
	assert.Contains(t, notif.RawText(), "PHP 8.2")
	assert.Contains(t, notif.RawText(), "my-server")
}

func TestPhpInstallationFailedNotification_ToEmail(t *testing.T) {
	tests := []struct {
		name         string
		serverName   string
		phpVersion   string
		output       string
		errorMessage string
		wantInBody   []string
	}{
		{
			name:         "with output and error",
			serverName:   "my-server",
			phpVersion:   "8.2",
			output:       "apt-get failed",
			errorMessage: "package not found",
			wantInBody:   []string{"my-server", "8.2", "apt-get failed", "package not found"},
		},
		{
			name:         "without output",
			serverName:   "my-server",
			phpVersion:   "8.1",
			output:       "",
			errorMessage: "some error",
			wantInBody:   []string{"my-server", "8.1", "some error"},
		},
		{
			name:         "without error",
			serverName:   "my-server",
			phpVersion:   "8.3",
			output:       "output text",
			errorMessage: "",
			wantInBody:   []string{"my-server", "8.3", "output text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notif := NewPhpInstallationFailedNotification(tt.serverName, tt.phpVersion, tt.output, tt.errorMessage)
			email := notif.ToEmail()

			require.NotNil(t, email)
			assert.Equal(t, "PHP Installation Failed", email.Subject)
			assert.True(t, email.IsHTML)

			for _, want := range tt.wantInBody {
				assert.Contains(t, email.Body, want)
			}
		})
	}
}

func TestPhpInstallationFailedNotification_ToSlack(t *testing.T) {
	notif := NewPhpInstallationFailedNotification("my-server", "8.2", "output", "error")

	message := notif.ToSlack()

	assert.Contains(t, message, "🚨")
	assert.Contains(t, message, "PHP Installation Failed")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "8.2")
	assert.Contains(t, message, "output")
	assert.Contains(t, message, "error")
}

func TestPhpInstallationFailedNotification_ToDiscord(t *testing.T) {
	notif := NewPhpInstallationFailedNotification("my-server", "8.2", "output", "error")

	message := notif.ToDiscord()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "8.2")
}

func TestPhpInstallationFailedNotification_ToTelegram(t *testing.T) {
	notif := NewPhpInstallationFailedNotification("my-server", "8.2", "output", "error")

	message := notif.ToTelegram()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "8.2")
}
