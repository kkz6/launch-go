package notifications

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewDeploymentFailedNotification(t *testing.T) {
	tests := []struct {
		name        string
		siteAddress string
		serverName  string
		status      DeploymentStatus
		wantLabel   string
	}{
		{
			name:        "deployment failed",
			siteAddress: "mysite.com",
			serverName:  "my-server",
			status:      DeploymentStatusFailed,
			wantLabel:   "failed",
		},
		{
			name:        "deployment timeout",
			siteAddress: "mysite.com",
			serverName:  "my-server",
			status:      DeploymentStatusTimeout,
			wantLabel:   "timed out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notif := NewDeploymentFailedNotification(tt.siteAddress, tt.serverName, tt.status)

			require.NotNil(t, notif)
			assert.Equal(t, tt.siteAddress, notif.SiteAddress)
			assert.Equal(t, tt.serverName, notif.ServerName)
			assert.Equal(t, tt.status, notif.Status)
			assert.Equal(t, notificationtypes.NotificationTypeDeploymentFailed, notif.Type())
			assert.Contains(t, notif.RawText(), tt.wantLabel)
		})
	}
}

func TestDeploymentFailedNotification_WithGitInfo(t *testing.T) {
	notif := NewDeploymentFailedNotification("mysite.com", "my-server", DeploymentStatusFailed)

	result := notif.WithGitInfo("abc123def456", "Fix bug", "John Doe")

	assert.Same(t, notif, result)
	assert.Equal(t, "abc123def456", notif.GitHash)
	assert.Equal(t, "Fix bug", notif.CommitMessage)
	assert.Equal(t, "John Doe", notif.CommitAuthor)
}

func TestDeploymentFailedNotification_WithTriggeredBy(t *testing.T) {
	notif := NewDeploymentFailedNotification("mysite.com", "my-server", DeploymentStatusFailed)

	result := notif.WithTriggeredBy("jane@example.com")

	assert.Same(t, notif, result)
	assert.Equal(t, "jane@example.com", notif.TriggeredBy)
}

func TestDeploymentFailedNotification_WithOutput(t *testing.T) {
	notif := NewDeploymentFailedNotification("mysite.com", "my-server", DeploymentStatusFailed)

	result := notif.WithOutput("error: composer install failed")

	assert.Same(t, notif, result)
	assert.Equal(t, "error: composer install failed", notif.Output)
}

func TestDeploymentFailedNotification_WithDeploymentTime(t *testing.T) {
	notif := NewDeploymentFailedNotification("mysite.com", "my-server", DeploymentStatusFailed)
	deployTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	result := notif.WithDeploymentTime(deployTime)

	assert.Same(t, notif, result)
	assert.Equal(t, deployTime, notif.DeploymentTime)
}

func TestDeploymentFailedNotification_ShortGitHash(t *testing.T) {
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
			name:     "exactly 7 chars",
			gitHash:  "abc123d",
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
			notif := NewDeploymentFailedNotification("mysite.com", "my-server", DeploymentStatusFailed)
			notif.GitHash = tt.gitHash

			assert.Equal(t, tt.expected, notif.ShortGitHash())
		})
	}
}

func TestDeploymentFailedNotification_StatusLabel(t *testing.T) {
	tests := []struct {
		status   DeploymentStatus
		expected string
	}{
		{DeploymentStatusFailed, "Failed"},
		{DeploymentStatusTimeout, "Timed Out"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			notif := NewDeploymentFailedNotification("mysite.com", "my-server", tt.status)
			assert.Equal(t, tt.expected, notif.StatusLabel())
		})
	}
}

func TestDeploymentFailedNotification_ToEmail(t *testing.T) {
	notif := NewDeploymentFailedNotification("mysite.com", "my-server", DeploymentStatusFailed)
	notif.WithGitInfo("abc123def456", "Fix bug", "John Doe")
	notif.WithTriggeredBy("jane@example.com")
	notif.WithOutput("error output")

	email := notif.ToEmail()

	require.NotNil(t, email)
	assert.Contains(t, email.Subject, "Failed")
	assert.Contains(t, email.Body, "mysite.com")
	assert.Contains(t, email.Body, "my-server")
	assert.Contains(t, email.Body, "abc123d")
	assert.Contains(t, email.Body, "Fix bug")
	assert.Contains(t, email.Body, "John Doe")
	assert.Contains(t, email.Body, "jane@example.com")
	assert.Contains(t, email.Body, "error output")
	assert.True(t, email.IsHTML)
}

func TestDeploymentFailedNotification_ToSlack(t *testing.T) {
	notif := NewDeploymentFailedNotification("mysite.com", "my-server", DeploymentStatusFailed)
	notif.WithGitInfo("abc123def456", "Fix bug", "John Doe")
	notif.WithOutput("error output")

	message := notif.ToSlack()

	assert.Contains(t, message, "🚨")
	assert.Contains(t, message, "mysite.com")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "abc123d")
	assert.Contains(t, message, "error output")
}

func TestDeploymentFailedNotification_ToSlack_Timeout(t *testing.T) {
	notif := NewDeploymentFailedNotification("mysite.com", "my-server", DeploymentStatusTimeout)

	message := notif.ToSlack()

	assert.Contains(t, message, "⏱️")
	assert.Contains(t, message, "Timed Out")
}

func TestDeploymentFailedNotification_ToTelegram(t *testing.T) {
	notif := NewDeploymentFailedNotification("mysite.com", "my-server", DeploymentStatusFailed)

	message := notif.ToTelegram()

	assert.Contains(t, message, "mysite.com")
	assert.Contains(t, message, "my-server")
}
