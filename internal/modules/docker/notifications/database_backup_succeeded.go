// Package notifications contains docker-module specific notifications.
// We keep these here (rather than in the global
// internal/modules/notification/notifications package) because the
// global package is for cross-module / platform-wide events; backup
// success/failure is specific to docker-managed databases and tied to
// docker-module data structures.
package notifications

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

// DatabaseBackupSucceededNotification is dispatched when a backup run
// completes successfully AND the backup config has NotifyOnSuccess=true.
// Carries everything the user needs to inspect the snapshot — bucket
// location, size, engine — without having to re-open the dashboard.
//
// Implements the taskrunner.Notification interface (via RawText on
// BaseNotification) so the notifier's adapter can route it to email,
// Slack, Discord, Telegram, etc.
type DatabaseBackupSucceededNotification struct {
	*models.BaseNotification
	DatabaseName    string
	ProjectName     string
	ServerName      string
	Engine          string
	ObjectKey       string
	SizeBytes       int64
	StorageProvider string
	FinishedAt      time.Time
	Source          string
	// DashboardURL is the deep link back into the launch UI's
	// backup page so the user can click straight into the run history.
	// Empty when we don't have the host info handy.
	DashboardURL string
}

// NewDatabaseBackupSucceededNotification constructs the notification
// with required fields. Optional metadata is layered on via With…
// helpers, matching the rest of the notification module's pattern
// (see deployment_failed.go).
func NewDatabaseBackupSucceededNotification(
	databaseName, projectName, serverName, engine string,
) *DatabaseBackupSucceededNotification {
	raw := fmt.Sprintf(
		"Backup of database '%s' (%s) on server '%s' completed successfully.",
		databaseName, engine, serverName,
	)
	return &DatabaseBackupSucceededNotification{
		BaseNotification: models.NewBaseNotification(
			notificationtypes.NotificationTypeDatabaseBackupSucceeded, raw,
		),
		DatabaseName: databaseName,
		ProjectName:  projectName,
		ServerName:   serverName,
		Engine:       engine,
		FinishedAt:   time.Now().UTC(),
	}
}

// WithObjectKey sets the S3 object key the snapshot was uploaded to.
func (n *DatabaseBackupSucceededNotification) WithObjectKey(key string) *DatabaseBackupSucceededNotification {
	n.ObjectKey = key
	return n
}

// WithSizeBytes records the gzipped dump's size for the email body.
func (n *DatabaseBackupSucceededNotification) WithSizeBytes(size int64) *DatabaseBackupSucceededNotification {
	n.SizeBytes = size
	return n
}

// WithStorageProvider records a human label for the destination so
// the email reads "uploaded to <Contabo Storage>" rather than a UUID.
func (n *DatabaseBackupSucceededNotification) WithStorageProvider(label string) *DatabaseBackupSucceededNotification {
	n.StorageProvider = label
	return n
}

// WithSource tags the run as "schedule" or "manual" so users can tell
// scheduled cron firings from someone clicking "Run now".
func (n *DatabaseBackupSucceededNotification) WithSource(source string) *DatabaseBackupSucceededNotification {
	n.Source = source
	return n
}

// WithDashboardURL adds the deep link back into the UI.
func (n *DatabaseBackupSucceededNotification) WithDashboardURL(url string) *DatabaseBackupSucceededNotification {
	n.DashboardURL = url
	return n
}

// ToEmail builds an HTML-style email. We don't have a custom template
// for backup success yet (the existing mail/templates package only
// wires server/deploy/php notifications), so we fall back to plain
// text — the rest of the platform does the same when a template's
// missing. Slack/Discord/Telegram get the same message via formatChat.
func (n *DatabaseBackupSucceededNotification) ToEmail() *channels.EmailMessage {
	body := fmt.Sprintf(`Backup of database '%s' (%s) on server '%s' completed successfully.

Details:
• Project: %s
• Engine: %s
• Storage Provider: %s
• Object: %s
• Size: %s
• Finished: %s
• Triggered by: %s
`,
		n.DatabaseName, n.Engine, n.ServerName,
		fallback(n.ProjectName, "—"),
		n.Engine,
		fallback(n.StorageProvider, "—"),
		fallback(n.ObjectKey, "—"),
		humanSize(n.SizeBytes),
		n.FinishedAt.Format(time.RFC1123),
		fallback(n.Source, "schedule"),
	)
	if n.DashboardURL != "" {
		body += fmt.Sprintf("\nDashboard: %s\n", n.DashboardURL)
	}
	return &channels.EmailMessage{
		Subject: fmt.Sprintf("Backup succeeded: %s", n.DatabaseName),
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content. Markdown-style; Discord
// is close enough to use the same format.
func (n *DatabaseBackupSucceededNotification) ToSlack() string {
	msg := fmt.Sprintf(
		"*✅ Database Backup Succeeded*\n\nBackup of `%s` (%s) on server `%s` completed.",
		n.DatabaseName, n.Engine, n.ServerName,
	)
	msg += "\n\n*Details:*\n"
	if n.ProjectName != "" {
		msg += fmt.Sprintf("• Project: %s\n", n.ProjectName)
	}
	if n.StorageProvider != "" {
		msg += fmt.Sprintf("• Storage: %s\n", n.StorageProvider)
	}
	if n.ObjectKey != "" {
		msg += fmt.Sprintf("• Object: `%s`\n", n.ObjectKey)
	}
	msg += fmt.Sprintf("• Size: %s\n", humanSize(n.SizeBytes))
	msg += fmt.Sprintf("• Finished: %s\n", n.FinishedAt.Format(time.RFC1123))
	if n.Source != "" {
		msg += fmt.Sprintf("• Triggered by: %s\n", n.Source)
	}
	return msg
}

// ToDiscord uses the same Markdown rendering as Slack — Discord
// accepts the same `*bold*` / backtick syntax.
func (n *DatabaseBackupSucceededNotification) ToDiscord() string {
	return n.ToSlack()
}

// ToTelegram returns the HTML-style Telegram message content.
func (n *DatabaseBackupSucceededNotification) ToTelegram() string {
	msg := fmt.Sprintf(
		"<b>✅ Database Backup Succeeded</b>\n\nBackup of <code>%s</code> (%s) on server <code>%s</code> completed.",
		n.DatabaseName, n.Engine, n.ServerName,
	)
	if n.SizeBytes > 0 {
		msg += fmt.Sprintf("\n<b>Size:</b> %s", humanSize(n.SizeBytes))
	}
	return msg
}
