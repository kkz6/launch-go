package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseBackupEmailsUseTimeline(t *testing.T) {
	templates.Initialize("Launch", "https://launchctl.io")

	succeeded := NewDatabaseBackupSucceededNotification("app", "project", "server", "postgres").
		WithObjectKey("backups/app.sql.gz").
		WithDashboardURL("https://launchctl.io/backups")
	succeeded.FinishedAt = time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	successEmail := succeeded.ToEmail()
	require.True(t, successEmail.IsHTML)
	assert.Contains(t, successEmail.Body, "lctl / database")
	assert.Contains(t, successEmail.Body, "BACKUP COMPLETE")

	failed := NewDatabaseBackupFailedNotification("app", "project", "server", "postgres").
		WithError(`<script>alert("x")</script>`).
		WithDashboardURL("https://launchctl.io/backups")
	failureEmail := failed.ToEmail()
	require.True(t, failureEmail.IsHTML)
	assert.Contains(t, failureEmail.Body, "BACKUP FAILED")
	assert.NotContains(t, failureEmail.Body, "<script>alert")
}

func TestGHAPermissionsEmailUsesTimeline(t *testing.T) {
	templates.Initialize("Launch", "https://launchctl.io")

	message := NewGHAPermissionsMissingNotification("application", "api", "project", "server").
		WithRepository("launch/api").
		WithAppSettingsURL("https://github.com/settings/apps/launch").
		WithDashboardURL("https://launchctl.io/gha").
		ToEmail()

	require.True(t, message.IsHTML)
	assert.Contains(t, message.Body, "lctl / github")
	assert.Contains(t, message.Body, "ACTION REQUIRED")
	assert.Contains(t, strings.ToLower(message.Body), "required repository permissions")
}
