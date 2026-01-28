// Package markers provides output marker parsing for SSH-streamed task output.
// Markers are used to communicate structured events (progress, step completion, etc.)
// from bash scripts back to the Go application through stdout.
//
// Marker Format: ::LAUNCH::<type>::<value>
//
// Example:
//
//	echo "::LAUNCH::step_completed::configure_swap"
//	echo "::LAUNCH::progress::50"
package markers

import (
	"strconv"
	"strings"
)

// Marker prefix that identifies a line as a marker
const Prefix = "::LAUNCH::"

// Marker types
const (
	// StepCompleted indicates a provision step has finished
	// Value: step name (e.g., "configure_swap", "setup_firewall")
	StepCompleted = "step_completed"

	// SoftwareInstalled indicates software installation is complete
	// Value: software identifier (e.g., "php83", "mysql80", "caddy2")
	SoftwareInstalled = "software_installed"

	// Progress indicates overall task progress
	// Value: integer 0-100
	Progress = "progress"

	// Status provides a human-readable status message
	// Value: status message for display
	Status = "status"

	// Error indicates a non-fatal error occurred
	// Value: error message
	Error = "error"

	// ExitCode indicates the script's exit code (used at end of script)
	// Value: integer exit code
	ExitCode = "exit_code"
)

// Marker represents a parsed output marker
type Marker struct {
	Type  string
	Value string
}

// Parse extracts a marker from an output line.
// Returns nil if the line is not a marker.
func Parse(line string) *Marker {
	line = strings.TrimSpace(line)

	if !strings.HasPrefix(line, Prefix) {
		return nil
	}

	remainder := strings.TrimPrefix(line, Prefix)
	parts := strings.SplitN(remainder, "::", 2)

	if len(parts) != 2 {
		return nil
	}

	return &Marker{
		Type:  parts[0],
		Value: parts[1],
	}
}

// IsMarker checks if a line contains a marker
func IsMarker(line string) bool {
	return strings.Contains(line, Prefix)
}

// ProgressValue returns the progress as an integer.
// Returns 0 if the marker is not a progress marker or value is invalid.
func (m *Marker) ProgressValue() int {
	if m.Type != Progress {
		return 0
	}

	val, err := strconv.Atoi(m.Value)
	if err != nil {
		return 0
	}

	if val < 0 {
		return 0
	}
	if val > 100 {
		return 100
	}

	return val
}

// ExitCodeValue returns the exit code as an integer.
// Returns -1 if the marker is not an exit code marker or value is invalid.
func (m *Marker) ExitCodeValue() int {
	if m.Type != ExitCode {
		return -1
	}

	val, err := strconv.Atoi(m.Value)
	if err != nil {
		return -1
	}

	return val
}

// Format creates a marker string for embedding in bash scripts
func Format(markerType, value string) string {
	return Prefix + markerType + "::" + value
}

// FormatProgress creates a progress marker string
func FormatProgress(percent int) string {
	return Format(Progress, strconv.Itoa(percent))
}

// FormatStepCompleted creates a step completed marker string
func FormatStepCompleted(step string) string {
	return Format(StepCompleted, step)
}

// FormatSoftwareInstalled creates a software installed marker string
func FormatSoftwareInstalled(software string) string {
	return Format(SoftwareInstalled, software)
}

// FormatStatus creates a status marker string
func FormatStatus(message string) string {
	return Format(Status, message)
}

// FormatError creates an error marker string
func FormatError(message string) string {
	return Format(Error, message)
}

// FormatExitCode creates an exit code marker string
func FormatExitCode(code int) string {
	return Format(ExitCode, strconv.Itoa(code))
}

// BashEcho returns a bash echo command that outputs this marker
func BashEcho(markerType, value string) string {
	return `echo "` + Format(markerType, value) + `"`
}

// BashEchoProgress returns a bash echo command for a progress marker
func BashEchoProgress(percent int) string {
	return `echo "` + FormatProgress(percent) + `"`
}

// BashEchoStepCompleted returns a bash echo command for a step completed marker
func BashEchoStepCompleted(step string) string {
	return `echo "` + FormatStepCompleted(step) + `"`
}

// BashEchoSoftwareInstalled returns a bash echo command for a software installed marker
func BashEchoSoftwareInstalled(software string) string {
	return `echo "` + FormatSoftwareInstalled(software) + `"`
}

// BashEchoStatus returns a bash echo command for a status marker
func BashEchoStatus(message string) string {
	return `echo "` + FormatStatus(message) + `"`
}
