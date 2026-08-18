package notifications

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	mailtemplates "github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// DatabaseBackupFailedNotification is dispatched when a backup run
// errors out AND the backup config has NotifyOnFailure=true (the
// default). The user gets enough detail in the notification body to
// triage without opening the dashboard: which database failed, on
// which server, the last few lines of error output, and a deep link
// to the run history.
type DatabaseBackupFailedNotification struct {
	*models.BaseNotification
	DatabaseName    string
	ProjectName     string
	ServerName      string
	Engine          string
	StorageProvider string
	ErrorOutput     string
	FailedAt        time.Time
	Source          string
	DashboardURL    string
}

// NewDatabaseBackupFailedNotification constructs the failure
// notification. Required fields are the minimum needed to identify
// what failed; optional context (output, link) layers on via With…
// helpers so the call site stays clean when those values aren't
// handy.
func NewDatabaseBackupFailedNotification(
	databaseName, projectName, serverName, engine string,
) *DatabaseBackupFailedNotification {
	raw := fmt.Sprintf(
		"Backup of database '%s' (%s) on server '%s' failed.",
		databaseName, engine, serverName,
	)
	return &DatabaseBackupFailedNotification{
		BaseNotification: models.NewBaseNotification(
			notificationtypes.NotificationTypeDatabaseBackupFailed, raw,
		),
		DatabaseName: databaseName,
		ProjectName:  projectName,
		ServerName:   serverName,
		Engine:       engine,
		FailedAt:     time.Now().UTC(),
	}
}

// WithError attaches the captured error output. We truncate to ~2KB
// here to keep email bodies / Slack messages readable; the full
// output is still on the run row in the DB for deep inspection.
func (n *DatabaseBackupFailedNotification) WithError(output string) *DatabaseBackupFailedNotification {
	const limit = 2000
	if len(output) > limit {
		output = output[:limit] + "\n… (truncated; see dashboard for full output)"
	}
	n.ErrorOutput = output
	return n
}

// WithStorageProvider records a human label for the destination.
func (n *DatabaseBackupFailedNotification) WithStorageProvider(label string) *DatabaseBackupFailedNotification {
	n.StorageProvider = label
	return n
}

// WithSource tags the run as "schedule" or "manual".
func (n *DatabaseBackupFailedNotification) WithSource(source string) *DatabaseBackupFailedNotification {
	n.Source = source
	return n
}

// WithDashboardURL adds the deep link back into the UI.
func (n *DatabaseBackupFailedNotification) WithDashboardURL(url string) *DatabaseBackupFailedNotification {
	n.DashboardURL = url
	return n
}

// ToEmail renders backup failure in the shared transactional timeline.
func (n *DatabaseBackupFailedNotification) ToEmail() *channels.EmailMessage {
	body := fmt.Sprintf(`Backup of database '%s' (%s) on server '%s' failed.

Details:
• Project: %s
• Engine: %s
• Storage Provider: %s
• Failed: %s
• Triggered by: %s
`,
		n.DatabaseName, n.Engine, n.ServerName,
		fallback(n.ProjectName, "—"),
		n.Engine,
		fallback(n.StorageProvider, "—"),
		n.FailedAt.Format(time.RFC1123),
		fallback(n.Source, "schedule"),
	)
	if n.ErrorOutput != "" {
		body += fmt.Sprintf("\nError output:\n\n%s\n", n.ErrorOutput)
	}
	if n.DashboardURL != "" {
		body += fmt.Sprintf("\nDashboard: %s\n", n.DashboardURL)
	}

	details := fmt.Sprintf(`**Project:** %s

**Engine:** %s

**Storage:** %s

**Failed:** %s

**Triggered by:** %s`,
		fallback(n.ProjectName, "—"),
		n.Engine,
		fallback(n.StorageProvider, "—"),
		n.FailedAt.Format(time.RFC1123),
		fallback(n.Source, "schedule"),
	)
	builder := mailtemplates.NewEmail().
		WithContext("lctl / database").
		WithState("BACKUP FAILED", "error").
		WithGreeting("Database backup failed").
		WithIntro(fmt.Sprintf("Backup of **%s** on **%s** did not complete.", n.DatabaseName, n.ServerName)).
		WithPanel(details)
	if n.ErrorOutput != "" {
		builder.WithIntro("The backup process returned this output:").
			WithPanel("```\n" + n.ErrorOutput + "\n```")
	}
	if n.DashboardURL != "" {
		builder.WithAction("Open backup history", n.DashboardURL, "error")
	}
	html, err := builder.Build()
	if err == nil {
		return &channels.EmailMessage{
			Subject: fmt.Sprintf("Backup FAILED: %s", n.DatabaseName),
			Body:    html,
			IsHTML:  true,
		}
	}
	return &channels.EmailMessage{
		Subject: fmt.Sprintf("Backup FAILED: %s", n.DatabaseName),
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content.
func (n *DatabaseBackupFailedNotification) ToSlack() string {
	msg := fmt.Sprintf(
		"*🚨 Database Backup Failed*\n\nBackup of `%s` (%s) on server `%s` failed.",
		n.DatabaseName, n.Engine, n.ServerName,
	)
	msg += "\n\n*Details:*\n"
	if n.ProjectName != "" {
		msg += fmt.Sprintf("• Project: %s\n", n.ProjectName)
	}
	if n.StorageProvider != "" {
		msg += fmt.Sprintf("• Storage: %s\n", n.StorageProvider)
	}
	msg += fmt.Sprintf("• Failed: %s\n", n.FailedAt.Format(time.RFC1123))
	if n.Source != "" {
		msg += fmt.Sprintf("• Triggered by: %s\n", n.Source)
	}
	if n.ErrorOutput != "" {
		msg += fmt.Sprintf("\n*Last lines of output:*\n```\n%s\n```", n.ErrorOutput)
	}
	return msg
}

// ToDiscord shares the Slack Markdown rendering.
func (n *DatabaseBackupFailedNotification) ToDiscord() string {
	return n.ToSlack()
}

// ToTelegram returns the HTML-style Telegram message content.
func (n *DatabaseBackupFailedNotification) ToTelegram() string {
	msg := fmt.Sprintf(
		"<b>🚨 Database Backup Failed</b>\n\nBackup of <code>%s</code> (%s) on server <code>%s</code> failed.",
		n.DatabaseName, n.Engine, n.ServerName,
	)
	if n.ErrorOutput != "" {
		msg += fmt.Sprintf("\n\n<b>Output:</b>\n<pre>%s</pre>", n.ErrorOutput)
	}
	return msg
}
