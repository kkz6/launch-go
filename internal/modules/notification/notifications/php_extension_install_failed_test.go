package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewPhpExtensionInstallFailedNotification(t *testing.T) {
	notif := NewPhpExtensionInstallFailedNotification("my-server", "redis", "8.2", "pecl output", "compilation error")

	require.NotNil(t, notif)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, "redis", notif.ExtensionName)
	assert.Equal(t, "8.2", notif.PhpVersion)
	assert.Equal(t, "pecl output", notif.Output)
	assert.Equal(t, "compilation error", notif.ErrorMessage)
	assert.Equal(t, notificationtypes.NotificationTypePhpExtensionInstallFailed, notif.Type())
	assert.Contains(t, notif.RawText(), "redis")
	assert.Contains(t, notif.RawText(), "my-server")
	assert.Contains(t, notif.RawText(), "8.2")
}

func TestPhpExtensionInstallFailedNotification_ToEmail(t *testing.T) {
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
			output:        "pecl install failed",
			errorMessage:  "compilation error",
			wantInBody:    []string{"my-server", "redis", "8.2", "pecl install failed", "compilation error"},
		},
		{
			name:          "without output",
			serverName:    "my-server",
			extensionName: "imagick",
			phpVersion:    "8.1",
			output:        "",
			errorMessage:  "missing dependencies",
			wantInBody:    []string{"my-server", "imagick", "8.1", "missing dependencies"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notif := NewPhpExtensionInstallFailedNotification(tt.serverName, tt.extensionName, tt.phpVersion, tt.output, tt.errorMessage)
			email := notif.ToEmail()

			require.NotNil(t, email)
			assert.Equal(t, "PHP Extension Installation Failed", email.Subject)
			assert.False(t, email.IsHTML)

			for _, want := range tt.wantInBody {
				assert.Contains(t, email.Body, want)
			}
		})
	}
}

func TestPhpExtensionInstallFailedNotification_ToSlack(t *testing.T) {
	notif := NewPhpExtensionInstallFailedNotification("my-server", "redis", "8.2", "output", "error")

	message := notif.ToSlack()

	assert.Contains(t, message, "PHP Extension Installation Failed")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "redis")
	assert.Contains(t, message, "8.2")
	assert.Contains(t, message, "output")
	assert.Contains(t, message, "error")
}

func TestPhpExtensionInstallFailedNotification_ToDiscord(t *testing.T) {
	notif := NewPhpExtensionInstallFailedNotification("my-server", "redis", "8.2", "output", "error")

	message := notif.ToDiscord()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "redis")
}

func TestPhpExtensionInstallFailedNotification_ToTelegram(t *testing.T) {
	notif := NewPhpExtensionInstallFailedNotification("my-server", "redis", "8.2", "output", "error")

	message := notif.ToTelegram()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "redis")
}
