package slack

import (
	"context"
	"fmt"
	"time"
)

// ServerInfo contains server details for alerts
type ServerInfo struct {
	ID              string
	Name            string
	TeamID          string
	TeamName        string
	Provider        string
	PublicIPv4      string
	ServerType      string
	OperatingSystem string
	MemoryInMB      int
	CPUCores        int
}

// UserInfo contains user details for alerts
type UserInfo struct {
	Name  string
	Email string
}

// ServerProvisioningFailedAdminAlert sends an admin alert when server provisioning fails
type ServerProvisioningFailedAdminAlert struct {
	alerter              *AdminAlerter
	server               ServerInfo
	user                 *UserInfo
	output               string
	errorMessage         string
	outputRetrievalError string
}

// NewServerProvisioningFailedAdminAlert creates a new admin alert
func NewServerProvisioningFailedAdminAlert(alerter *AdminAlerter, server ServerInfo) *ServerProvisioningFailedAdminAlert {
	return &ServerProvisioningFailedAdminAlert{
		alerter: alerter,
		server:  server,
	}
}

// WithUser sets the user who provisioned the server
func (a *ServerProvisioningFailedAdminAlert) WithUser(user *UserInfo) *ServerProvisioningFailedAdminAlert {
	a.user = user
	return a
}

// WithOutput sets the task output
func (a *ServerProvisioningFailedAdminAlert) WithOutput(output string) *ServerProvisioningFailedAdminAlert {
	a.output = output
	return a
}

// WithErrorMessage sets the error message
func (a *ServerProvisioningFailedAdminAlert) WithErrorMessage(errorMessage string) *ServerProvisioningFailedAdminAlert {
	a.errorMessage = errorMessage
	return a
}

// WithOutputRetrievalError sets an error that occurred while retrieving output
func (a *ServerProvisioningFailedAdminAlert) WithOutputRetrievalError(err string) *ServerProvisioningFailedAdminAlert {
	a.outputRetrievalError = err
	return a
}

// Send sends the alert
func (a *ServerProvisioningFailedAdminAlert) Send(ctx context.Context) error {
	if !a.alerter.IsConfigured() {
		return nil
	}

	message := a.buildMessage()
	return a.alerter.Send(ctx, message)
}

func (a *ServerProvisioningFailedAdminAlert) buildMessage() *BlockKitMessage {
	message := NewBlockKitMessage(fmt.Sprintf("Server '%s' failed to provision", a.server.Name))

	message.AddHeader("🚨 Server Provisioning Failed")
	message.AddSection("A server failed to provision and requires attention.")
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
		fmt.Sprintf("*Provider:*\n%s", a.server.Provider),
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

	memory := "N/A"
	if a.server.MemoryInMB > 0 {
		memory = fmt.Sprintf("%d MB", a.server.MemoryInMB)
	}
	cpuCores := "N/A"
	if a.server.CPUCores > 0 {
		cpuCores = fmt.Sprintf("%d", a.server.CPUCores)
	}
	message.AddFieldsSection(
		fmt.Sprintf("*Memory:*\n%s", memory),
		fmt.Sprintf("*CPU Cores:*\n%s", cpuCores),
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
		fmt.Sprintf("*Provisioned By:*\n%s", userName),
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
