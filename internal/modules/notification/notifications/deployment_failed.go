package notifications

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

// DeploymentStatus represents the status of a deployment
type DeploymentStatus string

const (
	DeploymentStatusFailed  DeploymentStatus = "failed"
	DeploymentStatusTimeout DeploymentStatus = "timeout"
)

// DeploymentFailedNotification is sent when a deployment fails
type DeploymentFailedNotification struct {
	*models.BaseNotification
	SiteAddress    string
	ServerName     string
	Status         DeploymentStatus
	GitHash        string
	CommitMessage  string
	CommitAuthor   string
	TriggeredBy    string
	Output         string
	DeploymentTime time.Time
}

// NewDeploymentFailedNotification creates a new deployment failed notification
func NewDeploymentFailedNotification(siteAddress, serverName string, status DeploymentStatus) *DeploymentFailedNotification {
	statusLabel := "failed"
	if status == DeploymentStatusTimeout {
		statusLabel = "timed out"
	}

	rawText := fmt.Sprintf("Deployment %s for site '%s' on server '%s'.", statusLabel, siteAddress, serverName)
	return &DeploymentFailedNotification{
		BaseNotification: models.NewBaseNotification(enums.NotificationTypeDeploymentFailed, rawText),
		SiteAddress:      siteAddress,
		ServerName:       serverName,
		Status:           status,
		DeploymentTime:   time.Now(),
	}
}

// WithGitInfo adds git information to the notification
func (n *DeploymentFailedNotification) WithGitInfo(hash, message, author string) *DeploymentFailedNotification {
	n.GitHash = hash
	n.CommitMessage = message
	n.CommitAuthor = author
	return n
}

// WithTriggeredBy adds the user who triggered the deployment
func (n *DeploymentFailedNotification) WithTriggeredBy(userName string) *DeploymentFailedNotification {
	n.TriggeredBy = userName
	return n
}

// WithOutput adds the task output
func (n *DeploymentFailedNotification) WithOutput(output string) *DeploymentFailedNotification {
	n.Output = output
	return n
}

// WithDeploymentTime adds the deployment time
func (n *DeploymentFailedNotification) WithDeploymentTime(t time.Time) *DeploymentFailedNotification {
	n.DeploymentTime = t
	return n
}

// ShortGitHash returns a shortened version of the git hash
func (n *DeploymentFailedNotification) ShortGitHash() string {
	if len(n.GitHash) > 7 {
		return n.GitHash[:7]
	}
	return n.GitHash
}

// StatusLabel returns a human-readable status label
func (n *DeploymentFailedNotification) StatusLabel() string {
	if n.Status == DeploymentStatusTimeout {
		return "Timed Out"
	}
	return "Failed"
}

// ToEmail returns the email message content
func (n *DeploymentFailedNotification) ToEmail() *channels.EmailMessage {
	statusLabel := n.StatusLabel()

	body := fmt.Sprintf("Deployment %s for site '%s' on server '%s'.\n\n", statusLabel, n.SiteAddress, n.ServerName)

	if n.GitHash != "" {
		body += fmt.Sprintf("Commit: %s\n", n.ShortGitHash())
	}

	if n.CommitMessage != "" {
		body += fmt.Sprintf("Commit Message: %s\n", n.CommitMessage)
	}

	if n.CommitAuthor != "" {
		body += fmt.Sprintf("Author: %s\n", n.CommitAuthor)
	}

	if n.TriggeredBy != "" {
		body += fmt.Sprintf("Triggered by: %s\n", n.TriggeredBy)
	}

	body += fmt.Sprintf("Time: %s\n", n.DeploymentTime.Format(time.RFC1123))

	if n.Output != "" {
		body += fmt.Sprintf("\nLast lines of output:\n\n%s", n.Output)
	}

	return &channels.EmailMessage{
		Subject: fmt.Sprintf("Deployment %s", statusLabel),
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *DeploymentFailedNotification) ToSlack() string {
	return n.formatWithLogs()
}

// ToDiscord returns the Discord message content
func (n *DeploymentFailedNotification) ToDiscord() string {
	return n.formatWithLogs()
}

// ToTelegram returns the Telegram message content
func (n *DeploymentFailedNotification) ToTelegram() string {
	return n.formatWithLogs()
}

func (n *DeploymentFailedNotification) formatWithLogs() string {
	statusEmoji := "🚨"
	if n.Status == DeploymentStatusTimeout {
		statusEmoji = "⏱️"
	}

	statusLabel := n.StatusLabel()

	message := fmt.Sprintf("*%s Deployment %s*\n\n", statusEmoji, statusLabel)
	message += fmt.Sprintf("Deployment %s for site '%s' on server '%s'.", statusLabel, n.SiteAddress, n.ServerName)

	message += "\n\n*Details:*\n"

	if n.GitHash != "" {
		message += fmt.Sprintf("• Commit: `%s`\n", n.ShortGitHash())
	}

	if n.CommitMessage != "" {
		message += fmt.Sprintf("• Commit Message: %s\n", n.CommitMessage)
	}

	if n.CommitAuthor != "" {
		message += fmt.Sprintf("• Author: %s\n", n.CommitAuthor)
	}

	if n.TriggeredBy != "" {
		message += fmt.Sprintf("• Triggered by: %s\n", n.TriggeredBy)
	}

	message += fmt.Sprintf("• Time: %s\n", n.DeploymentTime.Format(time.RFC1123))

	if n.Output != "" {
		message += fmt.Sprintf("\n*Last lines of output:*\n```\n%s\n```", n.Output)
	}

	return message
}
