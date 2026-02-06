package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewPhpExtensionUninstallFailedNotification(t *testing.T) {
	notif := NewPhpExtensionUninstallFailedNotification("my-server", "redis", "8.2", "apt-get output", "removal error")

	require.NotNil(t, notif)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, "redis", notif.ExtensionName)
	assert.Equal(t, "8.2", notif.PhpVersion)
	assert.Equal(t, "apt-get output", notif.Output)
	assert.Equal(t, "removal error", notif.ErrorMessage)
	assert.Equal(t, notificationtypes.NotificationTypePhpExtensionUninstallFailed, notif.Type())
	assert.Contains(t, notif.RawText(), "redis")
	assert.Contains(t, notif.RawText(), "my-server")
	assert.Contains(t, notif.RawText(), "removal")
}

func TestPhpExtensionUninstallFailedNotification_ToEmail(t *testing.T) {
	tests := []struct {
		name          string
		serverName    string
		extensionName string
		phpVersion    string
		output        string
		errorMessage  string
		wantInBody    []string
	}{
		{
			name:          "with output and error",
			serverName:    "my-server",
			extensionName: "redis",
			phpVersion:    "8.2",
			output:        "apt-get remove failed",
			errorMessage:  "package in use",
			wantInBody:    []string{"my-server", "redis", "8.2", "apt-get remove failed", "package in use"},
		},
		{
			name:          "without output",
			serverName:    "my-server",
			extensionName: "imagick",
			phpVersion:    "8.1",
			output:        "",
			errorMessage:  "removal failed",
			wantInBody:    []string{"my-server", "imagick", "8.1", "removal failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notif := NewPhpExtensionUninstallFailedNotification(tt.serverName, tt.extensionName, tt.phpVersion, tt.output, tt.errorMessage)
			email := notif.ToEmail()

			require.NotNil(t, email)
			assert.Equal(t, "PHP Extension Removal Failed", email.Subject)
			assert.True(t, email.IsHTML)

			for _, want := range tt.wantInBody {
				assert.Contains(t, email.Body, want)
			}
		})
	}
}

func TestPhpExtensionUninstallFailedNotification_ToSlack(t *testing.T) {
	notif := NewPhpExtensionUninstallFailedNotification("my-server", "redis", "8.2", "output", "error")

	message := notif.ToSlack()

	assert.Contains(t, message, "PHP Extension Removal Failed")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "redis")
	assert.Contains(t, message, "8.2")
	assert.Contains(t, message, "output")
	assert.Contains(t, message, "error")
}

func TestPhpExtensionUninstallFailedNotification_ToDiscord(t *testing.T) {
	notif := NewPhpExtensionUninstallFailedNotification("my-server", "redis", "8.2", "output", "error")

	message := notif.ToDiscord()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "redis")
}

func TestPhpExtensionUninstallFailedNotification_ToTelegram(t *testing.T) {
	notif := NewPhpExtensionUninstallFailedNotification("my-server", "redis", "8.2", "output", "error")

	message := notif.ToTelegram()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "redis")
}
