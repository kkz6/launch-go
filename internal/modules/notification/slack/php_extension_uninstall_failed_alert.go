package slack

import (
	"context"
	"fmt"
	"time"
)

// PhpExtensionUninstallFailedAdminAlert sends an admin alert when PHP extension removal fails
type PhpExtensionUninstallFailedAdminAlert struct {
	alerter              *AdminAlerter
	server               ServerInfo
	extensionName        string
	phpVersion           string
	user                 *UserInfo
	output               string
	errorMessage         string
	outputRetrievalError string
}

// NewPhpExtensionUninstallFailedAdminAlert creates a new admin alert
func NewPhpExtensionUninstallFailedAdminAlert(alerter *AdminAlerter, server ServerInfo, extensionName, phpVersion string) *PhpExtensionUninstallFailedAdminAlert {
	return &PhpExtensionUninstallFailedAdminAlert{
		alerter:       alerter,
		server:        server,
		extensionName: extensionName,
		phpVersion:    phpVersion,
	}
}

// WithUser sets the user who requested the removal
func (a *PhpExtensionUninstallFailedAdminAlert) WithUser(user *UserInfo) *PhpExtensionUninstallFailedAdminAlert {
	a.user = user
	return a
}

// WithOutput sets the task output
func (a *PhpExtensionUninstallFailedAdminAlert) WithOutput(output string) *PhpExtensionUninstallFailedAdminAlert {
	a.output = output
	return a
}

// WithErrorMessage sets the error message
func (a *PhpExtensionUninstallFailedAdminAlert) WithErrorMessage(errorMessage string) *PhpExtensionUninstallFailedAdminAlert {
	a.errorMessage = errorMessage
	return a
}

// WithOutputRetrievalError sets an error that occurred while retrieving output
func (a *PhpExtensionUninstallFailedAdminAlert) WithOutputRetrievalError(err string) *PhpExtensionUninstallFailedAdminAlert {
	a.outputRetrievalError = err
	return a
}

// Send sends the alert
func (a *PhpExtensionUninstallFailedAdminAlert) Send(ctx context.Context) error {
	if !a.alerter.IsConfigured() {
		return nil
	}

	message := a.buildMessage()
	return a.alerter.Send(ctx, message)
}

func (a *PhpExtensionUninstallFailedAdminAlert) buildMessage() *BlockKitMessage {
	message := NewBlockKitMessage(fmt.Sprintf("PHP extension '%s' removal failed on '%s'", a.extensionName, a.server.Name))

	message.AddHeader("PHP Extension Removal Failed")
	message.AddSection(fmt.Sprintf("PHP extension '%s' removal failed and requires attention.", a.extensionName))
	message.AddDivider()

	// Server details
	message.AddFieldsSection(
		fmt.Sprintf("*Server Name:*\n%s", a.server.Name),
		fmt.Sprintf("*Team:*\n%s", a.server.TeamName),
	)

	message.AddFieldsSection(
		fmt.Sprintf("*Extension:*\n%s", a.extensionName),
		fmt.Sprintf("*PHP Version:*\n%s", a.phpVersion),
	)

	ipAddress := a.server.PublicIPv4
	if ipAddress == "" {
		ipAddress = "Not assigned"
	}
	os := a.server.OperatingSystem
	if os == "" {
		os = "N/A"
	}
	message.AddFieldsSection(
		fmt.Sprintf("*IP Address:*\n%s", ipAddress),
		fmt.Sprintf("*Operating System:*\n%s", os),
	)

	message.AddDivider()

	// User details
	userName := "Unknown"
	userEmail := "N/A"
	if a.user != nil {
		if a.user.Name != "" {
			userName = a.user.Name
		}
		if a.user.Email != "" {
			userEmail = a.user.Email
		}
	}
	message.AddFieldsSection(
		fmt.Sprintf("*Requested By:*\n%s", userName),
		fmt.Sprintf("*User Email:*\n%s", userEmail),
	)

	// Output retrieval error
	if a.outputRetrievalError != "" {
		message.AddDivider()
		message.AddSection(fmt.Sprintf("⚠️ *Could not retrieve full logs:* %s", a.outputRetrievalError))
	}

	// Output
	if a.output != "" {
		message.AddDivider()
		message.AddSection("*Last 30 Lines of Output:*")
		message.AddSection(fmt.Sprintf("```\n%s\n```", TruncateOutput(a.output, 2900)))
	}

	// Error message
	if a.errorMessage != "" {
		message.AddDivider()
		message.AddSection("*Error Message:*")
		message.AddSection(fmt.Sprintf("```\n%s\n```", a.errorMessage))
	}

	// Context footer
	message.AddDivider()
	message.AddContext(fmt.Sprintf("Server ID: %s | Team ID: %s | %s",
		a.server.ID, a.server.TeamID, time.Now().Format("2006-01-02 15:04:05 MST")))

	return message
}
