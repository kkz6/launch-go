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
	BaseAlert
}

// NewServerProvisioningFailedAdminAlert creates a new admin alert
func NewServerProvisioningFailedAdminAlert(alerter *AdminAlerter, server ServerInfo) *ServerProvisioningFailedAdminAlert {
	return &ServerProvisioningFailedAdminAlert{
		BaseAlert: NewBaseAlert(alerter, server),
	}
}

// WithUser sets the user who provisioned the server
func (a *ServerProvisioningFailedAdminAlert) WithUser(user *UserInfo) *ServerProvisioningFailedAdminAlert {
	a.SetUser(user)
	return a
}

// WithOutput sets the task output
func (a *ServerProvisioningFailedAdminAlert) WithOutput(output string) *ServerProvisioningFailedAdminAlert {
	a.SetOutput(output)
	return a
}

// WithErrorMessage sets the error message
func (a *ServerProvisioningFailedAdminAlert) WithErrorMessage(errorMessage string) *ServerProvisioningFailedAdminAlert {
	a.SetErrorMessage(errorMessage)
	return a
}

// WithOutputRetrievalError sets an error that occurred while retrieving output
func (a *ServerProvisioningFailedAdminAlert) WithOutputRetrievalError(err string) *ServerProvisioningFailedAdminAlert {
	a.SetOutputRetrievalError(err)
	return a
}

// Send sends the alert
func (a *ServerProvisioningFailedAdminAlert) Send(ctx context.Context) error {
	if !a.Alerter.IsConfigured() {
		return nil
	}

	message := a.buildMessage()
	return a.Alerter.Send(ctx, message)
}

func (a *ServerProvisioningFailedAdminAlert) buildMessage() *BlockKitMessage {
	message := NewBlockKitMessage(fmt.Sprintf("Server '%s' failed to provision", a.Server.Name))

	message.AddHeader("Server Provisioning Failed")
	message.AddSection("A server failed to provision and requires attention.")
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
		fmt.Sprintf("*Provider:*\n%s", a.Server.Provider),
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

	memory := "N/A"
	if a.Server.MemoryInMB > 0 {
		memory = fmt.Sprintf("%d MB", a.Server.MemoryInMB)
	}
	cpuCores := "N/A"
	if a.Server.CPUCores > 0 {
		cpuCores = fmt.Sprintf("%d", a.Server.CPUCores)
	}
	message.AddFieldsSection(
		fmt.Sprintf("*Memory:*\n%s", memory),
		fmt.Sprintf("*CPU Cores:*\n%s", cpuCores),
	)

	message.AddDivider()

	// User details
	userName, userEmail := a.BuildUserSection()
	message.AddFieldsSection(
		fmt.Sprintf("*Provisioned By:*\n%s", userName),
		fmt.Sprintf("*User Email:*\n%s", userEmail),
	)

	// Output sections (output retrieval error, output, error message)
	a.BuildOutputSection(message)

	// Context footer
	a.BuildContextFooter(message, time.Now().Format("2006-01-02 15:04:05 MST"))

	return message
}
