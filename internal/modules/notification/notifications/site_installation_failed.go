package notifications

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// SiteInstallationFailedNotification is sent when initial site installation fails
type SiteInstallationFailedNotification struct {
	*models.BaseNotification
	SiteAddress   string
	ServerName    string
	GitHash       string
	CommitMessage string
	TriggeredBy   string
	Output        string
	InstallTime   time.Time
	SiteURL       string
}

// NewSiteInstallationFailedNotification creates a new site installation failed notification
func NewSiteInstallationFailedNotification(siteAddress, serverName string) *SiteInstallationFailedNotification {
	rawText := fmt.Sprintf("Site installation failed for '%s' on server '%s'.", siteAddress, serverName)
	return &SiteInstallationFailedNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypeSiteInstallationFailed, rawText),
		SiteAddress:      siteAddress,
		ServerName:       serverName,
		InstallTime:      time.Now(),
	}
}

// WithGitInfo adds git information to the notification
func (n *SiteInstallationFailedNotification) WithGitInfo(hash, message string) *SiteInstallationFailedNotification {
	n.GitHash = hash
	n.CommitMessage = message
	return n
}

// WithTriggeredBy adds the user who triggered the installation
func (n *SiteInstallationFailedNotification) WithTriggeredBy(userName string) *SiteInstallationFailedNotification {
	n.TriggeredBy = userName
	return n
}

// WithOutput adds the task output
func (n *SiteInstallationFailedNotification) WithOutput(output string) *SiteInstallationFailedNotification {
	n.Output = output
	return n
}

// WithSiteURL sets the site URL for the action button
func (n *SiteInstallationFailedNotification) WithSiteURL(url string) *SiteInstallationFailedNotification {
	n.SiteURL = url
	return n
}

// ShortGitHash returns a shortened version of the git hash
func (n *SiteInstallationFailedNotification) ShortGitHash() string {
	if len(n.GitHash) > 7 {
		return n.GitHash[:7]
	}
	return n.GitHash
}

// ToEmail returns the email message content
func (n *SiteInstallationFailedNotification) ToEmail() *channels.EmailMessage {
	builder := templates.NewEmail().
		WithGreeting("Site Installation Failed").
		WithIntro(fmt.Sprintf("Site installation failed for **%s** on server **%s**.", n.SiteAddress, n.ServerName))

	// Build details panel
	var details string
	if n.GitHash != "" {
		details += fmt.Sprintf("**Commit:** `%s`\n\n", n.ShortGitHash())
	}

	if n.CommitMessage != "" {
		details += fmt.Sprintf("**Commit Message:** %s\n\n", n.CommitMessage)
	}

	if n.TriggeredBy != "" {
		details += fmt.Sprintf("**Triggered by:** %s\n\n", n.TriggeredBy)
	}

	details += fmt.Sprintf("**Time:** %s", n.InstallTime.Format(time.RFC1123))

	if details != "" {
		builder.WithPanel(details)
	}

	if n.Output != "" {
		builder.WithIntro("**Last lines of output:**")
		builder.WithPanel("```\n" + n.Output + "\n```")
	}

	if n.SiteURL != "" {
		builder.WithAction("View Site", n.SiteURL, "error")
	}

	html, err := builder.Build()
	if err != nil {
		return n.plainTextEmail()
	}

	return &channels.EmailMessage{
		Subject: "Site Installation Failed",
		Body:    html,
		IsHTML:  true,
	}
}

func (n *SiteInstallationFailedNotification) plainTextEmail() *channels.EmailMessage {
	body := fmt.Sprintf("Site installation failed for '%s' on server '%s'.\n\n", n.SiteAddress, n.ServerName)

	if n.GitHash != "" {
		body += fmt.Sprintf("Commit: %s\n", n.ShortGitHash())
	}

	if n.CommitMessage != "" {
		body += fmt.Sprintf("Commit Message: %s\n", n.CommitMessage)
	}

	if n.TriggeredBy != "" {
		body += fmt.Sprintf("Triggered by: %s\n", n.TriggeredBy)
	}

	body += fmt.Sprintf("Time: %s\n", n.InstallTime.Format(time.RFC1123))

	if n.Output != "" {
		body += fmt.Sprintf("\nLast lines of output:\n\n%s", n.Output)
	}

	return &channels.EmailMessage{
		Subject: "Site Installation Failed",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *SiteInstallationFailedNotification) ToSlack() string {
	return n.formatWithLogs()
}

// ToDiscord returns the Discord message content
func (n *SiteInstallationFailedNotification) ToDiscord() string {
	return n.formatWithLogs()
}

// ToTelegram returns the Telegram message content
func (n *SiteInstallationFailedNotification) ToTelegram() string {
	return n.formatWithLogs()
}

func (n *SiteInstallationFailedNotification) formatWithLogs() string {
	message := "*🚨 Site Installation Failed*\n\n"
	message += fmt.Sprintf("Site installation failed for '%s' on server '%s'.", n.SiteAddress, n.ServerName)

	message += "\n\n*Details:*\n"

	if n.GitHash != "" {
		message += fmt.Sprintf("• Commit: `%s`\n", n.ShortGitHash())
	}

	if n.CommitMessage != "" {
		message += fmt.Sprintf("• Commit Message: %s\n", n.CommitMessage)
	}

	if n.TriggeredBy != "" {
		message += fmt.Sprintf("• Triggered by: %s\n", n.TriggeredBy)
	}

	message += fmt.Sprintf("• Time: %s\n", n.InstallTime.Format(time.RFC1123))

	if n.Output != "" {
		message += fmt.Sprintf("\n*Last lines of output:*\n```\n%s\n```", n.Output)
	}

	return message
}
