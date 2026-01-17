package slack

import (
	"context"
	"fmt"
	"time"
)

// PhpInstallationFailedAdminAlert sends an admin alert when PHP installation fails
type PhpInstallationFailedAdminAlert struct {
	alerter              *AdminAlerter
	server               ServerInfo
	phpVersion           string
	user                 *UserInfo
	output               string
	errorMessage         string
	outputRetrievalError string
}

// NewPhpInstallationFailedAdminAlert creates a new admin alert
func NewPhpInstallationFailedAdminAlert(alerter *AdminAlerter, server ServerInfo, phpVersion string) *PhpInstallationFailedAdminAlert {
	return &PhpInstallationFailedAdminAlert{
		alerter:    alerter,
		server:     server,
		phpVersion: phpVersion,
	}
}

// WithUser sets the user who requested the installation
func (a *PhpInstallationFailedAdminAlert) WithUser(user *UserInfo) *PhpInstallationFailedAdminAlert {
	a.user = user
	return a
}

// WithOutput sets the task output
func (a *PhpInstallationFailedAdminAlert) WithOutput(output string) *PhpInstallationFailedAdminAlert {
	a.output = output
	return a
}

// WithErrorMessage sets the error message
func (a *PhpInstallationFailedAdminAlert) WithErrorMessage(errorMessage string) *PhpInstallationFailedAdminAlert {
	a.errorMessage = errorMessage
	return a
}

// WithOutputRetrievalError sets an error that occurred while retrieving output
func (a *PhpInstallationFailedAdminAlert) WithOutputRetrievalError(err string) *PhpInstallationFailedAdminAlert {
	a.outputRetrievalError = err
	return a
}

// Send sends the alert
func (a *PhpInstallationFailedAdminAlert) Send(ctx context.Context) error {
	if !a.alerter.IsConfigured() {
		return nil
	}

	message := a.buildMessage()
	return a.alerter.Send(ctx, message)
}

func (a *PhpInstallationFailedAdminAlert) buildMessage() *BlockKitMessage {
	message := NewBlockKitMessage(fmt.Sprintf("PHP %s installation failed on '%s'", a.phpVersion, a.server.Name))

	message.AddHeader("🚨 PHP Installation Failed")
	message.AddSection(fmt.Sprintf("PHP %s installation failed and requires attention.", a.phpVersion))
	message.AddDivider()

	// Server details
	message.AddFieldsSection(
		fmt.Sprintf("*Server Name:*\n%s", a.server.Name),
		fmt.Sprintf("*Team:*\n%s", a.server.TeamName),
	)

	ipAddress := a.server.PublicIPv4
	if ipAddress == "" {
		ipAddress = "Not assigned"
	}
	message.AddFieldsSection(
		fmt.Sprintf("*PHP Version:*\n%s", a.phpVersion),
		fmt.Sprintf("*IP Address:*\n%s", ipAddress),
	)

	serverType := a.server.ServerType
	if serverType == "" {
		serverType = "N/A"
	}
	os := a.server.OperatingSystem
	if os == "" {
		os = "N/A"
	}
	message.AddFieldsSection(
		fmt.Sprintf("*Server Type:*\n%s", serverType),
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
