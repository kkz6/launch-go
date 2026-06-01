package notifications

import (
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

// GHAPermissionsMissingNotification fires when the gha:bootstrap_workflow
// job hits a GitHub 403 "Resource not accessible by integration" — the
// GitHub App is installed but lacks one of the repository permissions
// the bootstrap pipeline needs. asynq would otherwise burn ~25 retries
// against a 403 that will never succeed; we surface the failure to the
// human who can actually fix it (the App owner) so they grant the
// missing scope and re-sync.
//
// The notification routes through the existing team-channel pipe — the
// dedicated user_id-routed channel is a follow-up. The body still
// identifies the specific workload + which permissions GitHub needs,
// so a team member receiving the email can pass it to the GitHub Org
// admin if they aren't the same person.
type GHAPermissionsMissingNotification struct {
	*models.BaseNotification
	WorkloadKind string // "application" | "compose"
	WorkloadName string
	ProjectName  string
	ServerName   string
	Repository   string // owner/repo as configured on the workload
	AppSettings  string // URL to the GitHub App's permissions page
	DashboardURL string // deep link back to the GHA tab in Launch
	DetectedAt   time.Time
}

// requiredPermissions is the canonical list of repo-scoped permissions
// the bootstrap pipeline needs. Surfaced verbatim in the email body so
// the user can compare with the App's current settings without
// guessing.
var requiredPermissions = []string{
	"Contents: Read and write",
	"Actions: Write",
	"Secrets: Read and write",
	"Variables: Read and write",
	"Metadata: Read (default)",
}

// NewGHAPermissionsMissingNotification builds the notification with
// the minimum information needed to identify the failed workload.
// Optional context (repo URL, dashboard link) layers on via With…
// helpers so test rigs that don't wire those don't have to thread
// them through.
func NewGHAPermissionsMissingNotification(
	workloadKind, workloadName, projectName, serverName string,
) *GHAPermissionsMissingNotification {
	raw := fmt.Sprintf(
		"GitHub Actions bootstrap for %s '%s' (project '%s', server '%s') was blocked: "+
			"the connected GitHub App is missing required repo permissions.",
		workloadKind, workloadName, projectName, serverName,
	)
	return &GHAPermissionsMissingNotification{
		BaseNotification: models.NewBaseNotification(
			notificationtypes.NotificationTypeGHAPermissionsMissing, raw,
		),
		WorkloadKind: workloadKind,
		WorkloadName: workloadName,
		ProjectName:  projectName,
		ServerName:   serverName,
		DetectedAt:   time.Now().UTC(),
	}
}

// WithRepository records the GitHub repo (owner/repo) so the recipient
// can identify which install needs the permission update.
func (n *GHAPermissionsMissingNotification) WithRepository(repo string) *GHAPermissionsMissingNotification {
	n.Repository = repo
	return n
}

// WithAppSettingsURL adds the direct link to the GitHub App's
// permissions page so the email is one click away from the fix.
func (n *GHAPermissionsMissingNotification) WithAppSettingsURL(url string) *GHAPermissionsMissingNotification {
	n.AppSettings = url
	return n
}

// WithDashboardURL adds the deep link back into the Launch UI so the
// user lands directly on the GHA tab to click Re-sync after fixing
// permissions.
func (n *GHAPermissionsMissingNotification) WithDashboardURL(url string) *GHAPermissionsMissingNotification {
	n.DashboardURL = url
	return n
}

// ToEmail returns a plain-text email message. Includes the
// step-by-step recovery path so the recipient doesn't have to read
// docs to figure out the fix.
func (n *GHAPermissionsMissingNotification) ToEmail() *channels.EmailMessage {
	var perms strings.Builder
	for _, p := range requiredPermissions {
		perms.WriteString(fmt.Sprintf("  • %s\n", p))
	}

	body := fmt.Sprintf(`GitHub Actions builds are blocked on this %s.

Workload:    %s
Project:     %s
Server:      %s
Repository:  %s
Detected at: %s

GitHub returned 403 "Resource not accessible by integration" while
Launch tried to set up the deploy workflow. The connected GitHub App
is installed but doesn't have all of the repository permissions the
bootstrap pipeline needs.

To unblock builds:

1. Open the GitHub App's settings (see link below).
2. Set these repository permissions to Read and write where applicable:

%s
3. Save. GitHub will mark the installation as needing approval.
4. Go to the installation page and accept the new permission set.
5. Click "Re-sync workflow file" on the GHA tab in Launch.
   Launch will retry the bootstrap automatically once GitHub's
   "new permissions accepted" webhook arrives, so step 5 is
   optional — but it's the fastest way to confirm the fix.
`,
		n.WorkloadKind,
		fallback(n.WorkloadName, "—"),
		fallback(n.ProjectName, "—"),
		fallback(n.ServerName, "—"),
		fallback(n.Repository, "—"),
		n.DetectedAt.Format(time.RFC1123),
		strings.TrimRight(perms.String(), "\n"),
	)
	if n.AppSettings != "" {
		body += fmt.Sprintf("\nGitHub App settings: %s\n", n.AppSettings)
	}
	if n.DashboardURL != "" {
		body += fmt.Sprintf("Launch dashboard:    %s\n", n.DashboardURL)
	}
	return &channels.EmailMessage{
		Subject: fmt.Sprintf("Action required: GitHub App permissions for %s", n.WorkloadName),
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack-flavoured message.
func (n *GHAPermissionsMissingNotification) ToSlack() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		"*⚠️ GitHub Actions blocked: missing App permissions*\n\n"+
			"Bootstrap for %s `%s` (project `%s`, server `%s`) was rejected by GitHub.",
		n.WorkloadKind, n.WorkloadName, n.ProjectName, n.ServerName,
	))
	if n.Repository != "" {
		b.WriteString(fmt.Sprintf("\nRepository: `%s`", n.Repository))
	}
	b.WriteString("\n\n*Required permissions:*\n")
	for _, p := range requiredPermissions {
		b.WriteString(fmt.Sprintf("• %s\n", p))
	}
	if n.AppSettings != "" {
		b.WriteString(fmt.Sprintf("\n<%s|Open GitHub App settings>", n.AppSettings))
	}
	if n.DashboardURL != "" {
		b.WriteString(fmt.Sprintf("\n<%s|Open Launch GHA tab>", n.DashboardURL))
	}
	return b.String()
}

// ToDiscord shares the Slack Markdown rendering.
func (n *GHAPermissionsMissingNotification) ToDiscord() string {
	return n.ToSlack()
}

// ToTelegram returns the HTML-style Telegram message content.
func (n *GHAPermissionsMissingNotification) ToTelegram() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		"<b>⚠️ GitHub Actions blocked: missing App permissions</b>\n\n"+
			"Bootstrap for %s <code>%s</code> (project <code>%s</code>, server <code>%s</code>) "+
			"was rejected by GitHub.",
		n.WorkloadKind, n.WorkloadName, n.ProjectName, n.ServerName,
	))
	if n.Repository != "" {
		b.WriteString(fmt.Sprintf("\nRepository: <code>%s</code>", n.Repository))
	}
	b.WriteString("\n\n<b>Required permissions:</b>\n")
	for _, p := range requiredPermissions {
		b.WriteString(fmt.Sprintf("• %s\n", p))
	}
	if n.AppSettings != "" {
		b.WriteString(fmt.Sprintf("\nGitHub App settings: %s", n.AppSettings))
	}
	if n.DashboardURL != "" {
		b.WriteString(fmt.Sprintf("\nLaunch dashboard: %s", n.DashboardURL))
	}
	return b.String()
}
