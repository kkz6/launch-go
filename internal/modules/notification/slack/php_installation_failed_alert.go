package slack

import (
	"context"
	"fmt"
	"time"
)

// PhpInstallationFailedAdminAlert sends an admin alert when PHP installation fails
type PhpInstallationFailedAdminAlert struct {
	BaseAlert
	phpVersion string
}

// NewPhpInstallationFailedAdminAlert creates a new admin alert
func NewPhpInstallationFailedAdminAlert(alerter *AdminAlerter, server ServerInfo, phpVersion string) *PhpInstallationFailedAdminAlert {
	return &PhpInstallationFailedAdminAlert{
		BaseAlert:  NewBaseAlert(alerter, server),
		phpVersion: phpVersion,
	}
}

// WithUser sets the user who requested the installation
func (a *PhpInstallationFailedAdminAlert) WithUser(user *UserInfo) *PhpInstallationFailedAdminAlert {
	a.SetUser(user)
	return a
}

// WithOutput sets the task output
func (a *PhpInstallationFailedAdminAlert) WithOutput(output string) *PhpInstallationFailedAdminAlert {
	a.SetOutput(output)
	return a
}

// WithErrorMessage sets the error message
func (a *PhpInstallationFailedAdminAlert) WithErrorMessage(errorMessage string) *PhpInstallationFailedAdminAlert {
	a.SetErrorMessage(errorMessage)
	return a
}

// WithOutputRetrievalError sets an error that occurred while retrieving output
func (a *PhpInstallationFailedAdminAlert) WithOutputRetrievalError(err string) *PhpInstallationFailedAdminAlert {
	a.SetOutputRetrievalError(err)
	return a
}

// Send sends the alert
func (a *PhpInstallationFailedAdminAlert) Send(ctx context.Context) error {
	if !a.Alerter.IsConfigured() {
		return nil
	}

	message := a.buildMessage()
	return a.Alerter.Send(ctx, message)
}

func (a *PhpInstallationFailedAdminAlert) buildMessage() *BlockKitMessage {
	message := NewBlockKitMessage(fmt.Sprintf("PHP %s installation failed on '%s'", a.phpVersion, a.Server.Name))

	message.AddHeader("🚨 PHP Installation Failed")
	message.AddSection(fmt.Sprintf("PHP %s installation failed and requires attention.", a.phpVersion))
	message.AddDivider()

	// Server details
	message.AddFieldsSection(
		fmt.Sprintf("*Server Name:*\n%s", a.Server.Name),
		fmt.Sprintf("*Team:*\n%s", a.Server.TeamName),
	)

	ipAddress := a.Server.PublicIPv4
	if ipAddress == "" {
		ipAddress = "Not assigned"
	}
	message.AddFieldsSection(
		fmt.Sprintf("*PHP Version:*\n%s", a.phpVersion),
		fmt.Sprintf("*IP Address:*\n%s", ipAddress),
	)

	serverType := a.Server.ServerType
	if serverType == "" {
		serverType = "N/A"
	}
	os := a.Server.OperatingSystem
	if os == "" {
		os = "N/A"
	}
	message.AddFieldsSection(
		fmt.Sprintf("*Server Type:*\n%s", serverType),
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
