package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewSiteInstallationFailedNotification(t *testing.T) {
	notif := NewSiteInstallationFailedNotification("mysite.com", "my-server")

	require.NotNil(t, notif)
	assert.Equal(t, "mysite.com", notif.SiteAddress)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, notificationtypes.NotificationTypeSiteInstallationFailed, notif.Type())
	assert.Contains(t, notif.RawText(), "mysite.com")
	assert.Contains(t, notif.RawText(), "my-server")
	assert.NotZero(t, notif.InstallTime)
}

func TestSiteInstallationFailedNotification_WithGitInfo(t *testing.T) {
	notif := NewSiteInstallationFailedNotification("mysite.com", "my-server")

	result := notif.WithGitInfo("abc123def456", "Initial commit")

	assert.Same(t, notif, result)
	assert.Equal(t, "abc123def456", notif.GitHash)
	assert.Equal(t, "Initial commit", notif.CommitMessage)
}

func TestSiteInstallationFailedNotification_WithTriggeredBy(t *testing.T) {
	notif := NewSiteInstallationFailedNotification("mysite.com", "my-server")

	result := notif.WithTriggeredBy("john@example.com")

	assert.Same(t, notif, result)
	assert.Equal(t, "john@example.com", notif.TriggeredBy)
}

func TestSiteInstallationFailedNotification_WithOutput(t *testing.T) {
	notif := NewSiteInstallationFailedNotification("mysite.com", "my-server")

	result := notif.WithOutput("npm install failed")

	assert.Same(t, notif, result)
	assert.Equal(t, "npm install failed", notif.Output)
}

func TestSiteInstallationFailedNotification_ShortGitHash(t *testing.T) {
	tests := []struct {
		name     string
		gitHash  string
		expected string
	}{
		{
			name:     "long hash",
			gitHash:  "abc123def456789",
			expected: "abc123d",
		},
		{
			name:     "short hash",
			gitHash:  "abc",
			expected: "abc",
		},
		{
			name:     "empty hash",
			gitHash:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notif := NewSiteInstallationFailedNotification("mysite.com", "my-server")
			notif.GitHash = tt.gitHash

			assert.Equal(t, tt.expected, notif.ShortGitHash())
		})
	}
}

func TestSiteInstallationFailedNotification_ToEmail(t *testing.T) {
	notif := NewSiteInstallationFailedNotification("mysite.com", "my-server")
	notif.WithGitInfo("abc123def456", "Initial commit")
	notif.WithTriggeredBy("john@example.com")
	notif.WithOutput("error: npm failed")

	email := notif.ToEmail()

	require.NotNil(t, email)
	assert.Equal(t, "Site Installation Failed", email.Subject)
	assert.Contains(t, email.Body, "mysite.com")
	assert.Contains(t, email.Body, "my-server")
	assert.Contains(t, email.Body, "abc123d")
	assert.Contains(t, email.Body, "Initial commit")
	assert.Contains(t, email.Body, "john@example.com")
	assert.Contains(t, email.Body, "npm failed")
	assert.False(t, email.IsHTML)
}

func TestSiteInstallationFailedNotification_ToSlack(t *testing.T) {
	notif := NewSiteInstallationFailedNotification("mysite.com", "my-server")
	notif.WithGitInfo("abc123def456", "Initial commit")
	notif.WithOutput("error output")

	message := notif.ToSlack()

	assert.Contains(t, message, "🚨")
	assert.Contains(t, message, "Site Installation Failed")
	assert.Contains(t, message, "mysite.com")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "abc123d")
	assert.Contains(t, message, "error output")
}

func TestSiteInstallationFailedNotification_ToDiscord(t *testing.T) {
	notif := NewSiteInstallationFailedNotification("mysite.com", "my-server")

	message := notif.ToDiscord()

	assert.Contains(t, message, "mysite.com")
	assert.Contains(t, message, "my-server")
}

func TestSiteInstallationFailedNotification_ToTelegram(t *testing.T) {
	notif := NewSiteInstallationFailedNotification("mysite.com", "my-server")

	message := notif.ToTelegram()

	assert.Contains(t, message, "mysite.com")
	assert.Contains(t, message, "my-server")
}
