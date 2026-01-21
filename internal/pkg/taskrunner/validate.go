package taskrunner

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ScriptValidationResult holds the results of script validation
type ScriptValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// ValidateScript runs shellcheck on a script and returns validation results.
// Requires shellcheck to be installed on the system.
//
// Example usage:
//
//	result, err := taskrunner.ValidateScript(script)
//	if err != nil {
//	    // shellcheck not available or other error
//	}
//	if !result.Valid {
//	    // script has errors
//	}
func ValidateScript(script string) (*ScriptValidationResult, error) {
	return ValidateScriptWithSeverity(script, "error")
}

// ValidateScriptWithSeverity validates a script with a minimum severity level.
// Severity levels: error, warning, info, style
func ValidateScriptWithSeverity(script string, minSeverity string) (*ScriptValidationResult, error) {
	// Check if shellcheck is available
	if _, err := exec.LookPath("shellcheck"); err != nil {
		return nil, fmt.Errorf("shellcheck not found in PATH: %w", err)
	}

	// Run shellcheck
	cmd := exec.Command("shellcheck", "-s", "bash", "--severity", minSeverity, "-f", "gcc", "-")
	cmd.Stdin = strings.NewReader(script)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &ScriptValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Parse output
	output := stdout.String()
	if output != "" {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			if strings.Contains(line, "error:") {
				result.Errors = append(result.Errors, line)
				result.Valid = false
			} else if strings.Contains(line, "warning:") {
				result.Warnings = append(result.Warnings, line)
			}
		}
	}

	// Exit code 1 means issues found, other non-zero means shellcheck error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				// Issues found (already parsed above)
				return result, nil
			}
		}
		return nil, fmt.Errorf("shellcheck failed: %w\nstderr: %s", err, stderr.String())
	}

	return result, nil
}

// MustValidateScript validates a script and panics if invalid.
// Use during development or testing.
func MustValidateScript(script string) {
	result, err := ValidateScript(script)
	if err != nil {
		panic(fmt.Sprintf("script validation error: %v", err))
	}
	if !result.Valid {
		panic(fmt.Sprintf("script validation failed:\n%s", strings.Join(result.Errors, "\n")))
	}
}

// IsShellcheckAvailable checks if shellcheck is installed
func IsShellcheckAvailable() bool {
	_, err := exec.LookPath("shellcheck")
	return err == nil
}

// ValidateScriptQuick performs a quick syntax check without shellcheck.
// Uses bash -n for basic syntax validation.
func ValidateScriptQuick(script string) error {
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(script)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bash syntax error: %s", stderr.String())
	}

	return nil
}
