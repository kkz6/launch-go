package notifications

import "fmt"

// FailureNotification interface for notifications with output/error
type FailureNotification interface {
	GetOutput() string
	GetErrorMessage() string
}

// FormatFailureMessage builds a standard failure notification message
func FormatFailureMessage(header string, mainMessage string, output string, errorMessage string) string {
	message := header + "\n\n" + mainMessage

	if output != "" {
		message += fmt.Sprintf("\n\n*Last lines of output:*\n```\n%s\n```", output)
	}

	if errorMessage != "" {
		message += fmt.Sprintf("\n\n*Error message:*\n```\n%s\n```", errorMessage)
	}

	return message
}
