package slack

import (
	"context"
	"fmt"
	"time"
)

// PhpExtensionInstallFailedAdminAlert sends an admin alert when PHP extension installation fails
type PhpExtensionInstallFailedAdminAlert struct {
	BaseAlert
	extensionName string
	phpVersion    string
}

// NewPhpExtensionInstallFailedAdminAlert creates a new admin alert
func NewPhpExtensionInstallFailedAdminAlert(alerter *AdminAlerter, server ServerInfo, extensionName, phpVersion string) *PhpExtensionInstallFailedAdminAlert {
	return &PhpExtensionInstallFailedAdminAlert{
		BaseAlert:     NewBaseAlert(alerter, server),
		extensionName: extensionName,
		phpVersion:    phpVersion,
	}
}

// WithUser sets the user who requested the installation
func (a *PhpExtensionInstallFailedAdminAlert) WithUser(user *UserInfo) *PhpExtensionInstallFailedAdminAlert {
	a.SetUser(user)
	return a
}

// WithOutput sets the task output
func (a *PhpExtensionInstallFailedAdminAlert) WithOutput(output string) *PhpExtensionInstallFailedAdminAlert {
	a.SetOutput(output)
	return a
}

// WithErrorMessage sets the error message
func (a *PhpExtensionInstallFailedAdminAlert) WithErrorMessage(errorMessage string) *PhpExtensionInstallFailedAdminAlert {
	a.SetErrorMessage(errorMessage)
	return a
}

// WithOutputRetrievalError sets an error that occurred while retrieving output
func (a *PhpExtensionInstallFailedAdminAlert) WithOutputRetrievalError(err string) *PhpExtensionInstallFailedAdminAlert {
	a.SetOutputRetrievalError(err)
	return a
}

// Send sends the alert
func (a *PhpExtensionInstallFailedAdminAlert) Send(ctx context.Context) error {
	if !a.Alerter.IsConfigured() {
		return nil
	}

	message := a.buildMessage()
	return a.Alerter.Send(ctx, message)
}

func (a *PhpExtensionInstallFailedAdminAlert) buildMessage() *BlockKitMessage {
	message := NewBlockKitMessage(fmt.Sprintf("PHP extension '%s' installation failed on '%s'", a.extensionName, a.Server.Name))

	message.AddHeader("PHP Extension Installation Failed")
	message.AddSection(fmt.Sprintf("PHP extension '%s' installation failed and requires attention.", a.extensionName))
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
