package slack

import (
	"context"
	"fmt"
	"time"
)

// PhpExtensionUninstallFailedAdminAlert sends an admin alert when PHP extension removal fails
type PhpExtensionUninstallFailedAdminAlert struct {
	BaseAlert
	extensionName string
	phpVersion    string
}

// NewPhpExtensionUninstallFailedAdminAlert creates a new admin alert
func NewPhpExtensionUninstallFailedAdminAlert(alerter *AdminAlerter, server ServerInfo, extensionName, phpVersion string) *PhpExtensionUninstallFailedAdminAlert {
	return &PhpExtensionUninstallFailedAdminAlert{
		BaseAlert:     NewBaseAlert(alerter, server),
		extensionName: extensionName,
		phpVersion:    phpVersion,
	}
}

// WithUser sets the user who requested the removal
func (a *PhpExtensionUninstallFailedAdminAlert) WithUser(user *UserInfo) *PhpExtensionUninstallFailedAdminAlert {
	a.SetUser(user)
	return a
}

// WithOutput sets the task output
func (a *PhpExtensionUninstallFailedAdminAlert) WithOutput(output string) *PhpExtensionUninstallFailedAdminAlert {
	a.SetOutput(output)
	return a
}

// WithErrorMessage sets the error message
func (a *PhpExtensionUninstallFailedAdminAlert) WithErrorMessage(errorMessage string) *PhpExtensionUninstallFailedAdminAlert {
	a.SetErrorMessage(errorMessage)
	return a
}

// WithOutputRetrievalError sets an error that occurred while retrieving output
func (a *PhpExtensionUninstallFailedAdminAlert) WithOutputRetrievalError(err string) *PhpExtensionUninstallFailedAdminAlert {
	a.SetOutputRetrievalError(err)
	return a
}

// Send sends the alert
func (a *PhpExtensionUninstallFailedAdminAlert) Send(ctx context.Context) error {
	if !a.Alerter.IsConfigured() {
		return nil
	}

	message := a.buildMessage()
	return a.Alerter.Send(ctx, message)
}

func (a *PhpExtensionUninstallFailedAdminAlert) buildMessage() *BlockKitMessage {
	message := NewBlockKitMessage(fmt.Sprintf("PHP extension '%s' removal failed on '%s'", a.extensionName, a.Server.Name))

	message.AddHeader("PHP Extension Removal Failed")
	message.AddSection(fmt.Sprintf("PHP extension '%s' removal failed and requires attention.", a.extensionName))
	message.AddDivider()

	// Server details
	message.AddFieldsSection(
		fmt.Sprintf("*Server Name:*\n%s", a.Server.Name),
		fmt.Sprintf("*Team:*\n%s", a.Server.TeamName),
	)

	message.AddFieldsSection(
		fmt.Sprintf("*Extension:*\n%s", a.extensionName),
		fmt.Sprintf("*PHP Version:*\n%s", a.phpVersion),
	)

	ipAddress := a.Server.PublicIPv4
	if ipAddress == "" {
		ipAddress = "Not assigned"
	}
	os := a.Server.OperatingSystem
	if os == "" {
		os = "N/A"
	}
	message.AddFieldsSection(
		fmt.Sprintf("*IP Address:*\n%s", ipAddress),
		fmt.Sprintf("*Operating System:*\n%s", os),
	)

	message.AddDivider()

	// User details
	userName, userEmail := a.BuildUserSection()
	message.AddFieldsSection(
		fmt.Sprintf("*Requested By:*\n%s", userName),
		fmt.Sprintf("*User Email:*\n%s", userEmail),
	)

	// Output/error sections
	a.BuildOutputSection(message)

	// Context footer
	a.BuildContextFooter(message, time.Now().Format("2006-01-02 15:04:05 MST"))

	return message
}
