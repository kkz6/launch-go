package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/notification/models"
	"github.com/kkz6/launch-go/internal/modules/notification/slack"
)

// Notifier provides a simple interface for sending notifications
type Notifier struct {
	channelService *NotificationChannelService
	adminAlerter   *slack.AdminAlerter
}

// NewNotifier creates a new notifier
func NewNotifier(channelService *NotificationChannelService, adminWebhookURL string) *Notifier {
	return &Notifier{
		channelService: channelService,
		adminAlerter:   slack.NewAdminAlerter(adminWebhookURL),
	}
}

// SendToTeam sends a notification to all connected channels for a team
func (n *Notifier) SendToTeam(ctx context.Context, teamID string, notification models.Notification) error {
	return n.channelService.SendToTeam(ctx, teamID, notification)
}

// SendToChannel sends a notification to a specific channel
func (n *Notifier) SendToChannel(ctx context.Context, channelID string, notification models.Notification) error {
	return n.channelService.SendToChannel(ctx, channelID, notification)
}

// AdminAlerter returns the admin alerter for sending admin Slack alerts
func (n *Notifier) AdminAlerter() *slack.AdminAlerter {
	return n.adminAlerter
}

// SendServerProvisioningFailedAdminAlert sends an admin alert for server provisioning failure
func (n *Notifier) SendServerProvisioningFailedAdminAlert(ctx context.Context, server slack.ServerInfo, user *slack.UserInfo, output, errorMessage string) error {
	alert := slack.NewServerProvisioningFailedAdminAlert(n.adminAlerter, server).
		WithUser(user).
		WithOutput(output).
		WithErrorMessage(errorMessage)

	return alert.Send(ctx)
}

// SendPhpInstallationFailedAdminAlert sends an admin alert for PHP installation failure
func (n *Notifier) SendPhpInstallationFailedAdminAlert(ctx context.Context, server slack.ServerInfo, phpVersion string, user *slack.UserInfo, output, errorMessage string) error {
	alert := slack.NewPhpInstallationFailedAdminAlert(n.adminAlerter, server, phpVersion).
		WithUser(user).
		WithOutput(output).
		WithErrorMessage(errorMessage)

	return alert.Send(ctx)
}

// SendPhpExtensionInstallFailedAdminAlert sends an admin alert for PHP extension installation failure
func (n *Notifier) SendPhpExtensionInstallFailedAdminAlert(ctx context.Context, server slack.ServerInfo, extensionName, phpVersion string, user *slack.UserInfo, output, errorMessage string) error {
	alert := slack.NewPhpExtensionInstallFailedAdminAlert(n.adminAlerter, server, extensionName, phpVersion).
		WithUser(user).
		WithOutput(output).
		WithErrorMessage(errorMessage)

	return alert.Send(ctx)
}

// SendPhpExtensionUninstallFailedAdminAlert sends an admin alert for PHP extension removal failure
func (n *Notifier) SendPhpExtensionUninstallFailedAdminAlert(ctx context.Context, server slack.ServerInfo, extensionName, phpVersion string, user *slack.UserInfo, output, errorMessage string) error {
	alert := slack.NewPhpExtensionUninstallFailedAdminAlert(n.adminAlerter, server, extensionName, phpVersion).
		WithUser(user).
		WithOutput(output).
		WithErrorMessage(errorMessage)

	return alert.Send(ctx)
}
