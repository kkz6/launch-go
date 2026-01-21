package slack

import (
	"fmt"
)

// BaseAlert contains common fields and methods for admin alerts
type BaseAlert struct {
	Alerter              *AdminAlerter
	Server               ServerInfo
	User                 *UserInfo
	Output               string
	ErrorMessage         string
	OutputRetrievalError string
}

// NewBaseAlert creates a new BaseAlert with the required fields
func NewBaseAlert(alerter *AdminAlerter, server ServerInfo) BaseAlert {
	return BaseAlert{
		Alerter: alerter,
		Server:  server,
	}
}

// SetUser sets the user info
func (a *BaseAlert) SetUser(user *UserInfo) {
	a.User = user
}

// SetOutput sets the command output
func (a *BaseAlert) SetOutput(output string) {
	a.Output = output
}

// SetErrorMessage sets the error message
func (a *BaseAlert) SetErrorMessage(errorMessage string) {
	a.ErrorMessage = errorMessage
}

// SetOutputRetrievalError sets the output retrieval error
func (a *BaseAlert) SetOutputRetrievalError(err string) {
	a.OutputRetrievalError = err
}

// BuildOutputSection adds the common output/error sections to a message
func (a *BaseAlert) BuildOutputSection(message *BlockKitMessage) {
	// Output retrieval error
	if a.OutputRetrievalError != "" {
		message.AddDivider()
		message.AddSection(fmt.Sprintf("Warning: *Could not retrieve full logs:* %s", a.OutputRetrievalError))
	}

	// Output
	if a.Output != "" {
		message.AddDivider()
		message.AddSection("*Last 30 Lines of Output:*")
		message.AddSection(fmt.Sprintf("```\n%s\n```", TruncateOutput(a.Output, 2900)))
	}

	// Error message
	if a.ErrorMessage != "" {
		message.AddDivider()
		message.AddSection("*Error Message:*")
		message.AddSection(fmt.Sprintf("```\n%s\n```", a.ErrorMessage))
	}
}

// BuildUserSection builds the user details fields and returns the formatted values
func (a *BaseAlert) BuildUserSection() (userName, userEmail string) {
	userName = "Unknown"
	userEmail = "N/A"

	if a.User != nil {
		if a.User.Name != "" {
			userName = a.User.Name
		}
		if a.User.Email != "" {
			userEmail = a.User.Email
		}
	}

	return userName, userEmail
}

// BuildContextFooter adds a common context footer to the message
func (a *BaseAlert) BuildContextFooter(message *BlockKitMessage, timestamp string) {
	message.AddDivider()
	message.AddContext(fmt.Sprintf("Server ID: %s | Team ID: %s | %s",
		a.Server.ID, a.Server.TeamID, timestamp))
}
