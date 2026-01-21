package jobs

import (
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TaskBuilder provides a fluent interface for building SSH tasks.
// It simplifies the process of creating tasks with scripts, callbacks,
// and other configuration options.
//
// Example usage:
//
//	task := jobs.NewTaskBuilder("Install Cron").
//	    WithTimeout(30 * time.Second).
//	    AddScript("cat > %s << 'EOF'\n%s\nEOF", cronPath, contents).
//	    AddScript("chmod 644 %s", cronPath).
//	    Build()
type TaskBuilder struct {
	name           string
	timeout        time.Duration
	scriptParts    []string
	useShellHeader bool
	useAptWait     bool
}

// NewTaskBuilder creates a new TaskBuilder with the given task name.
func NewTaskBuilder(name string) *TaskBuilder {
	return &TaskBuilder{
		name:           name,
		timeout:        10 * time.Minute, // Default timeout
		scriptParts:    make([]string, 0),
		useShellHeader: true, // Default to including shell header
	}
}

// WithTimeout sets the task timeout duration.
func (b *TaskBuilder) WithTimeout(timeout time.Duration) *TaskBuilder {
	b.timeout = timeout
	return b
}

// WithTimeoutSeconds sets the task timeout in seconds.
func (b *TaskBuilder) WithTimeoutSeconds(seconds int) *TaskBuilder {
	b.timeout = time.Duration(seconds) * time.Second
	return b
}

// WithShellHeader includes the standard shell header (set -euo pipefail, etc.).
// This is enabled by default.
func (b *TaskBuilder) WithShellHeader() *TaskBuilder {
	b.useShellHeader = true
	return b
}

// WithoutShellHeader disables the standard shell header.
// Use this when you need custom shell settings.
func (b *TaskBuilder) WithoutShellHeader() *TaskBuilder {
	b.useShellHeader = false
	return b
}

// WithAptWait includes the apt wait function for tasks that install packages.
// This prevents apt lock issues when multiple operations run concurrently.
func (b *TaskBuilder) WithAptWait() *TaskBuilder {
	b.useAptWait = true
	return b
}

// AddScript appends a script fragment to the task.
// Supports printf-style formatting.
//
// Example:
//
//	builder.AddScript("mkdir -p %s", dirPath)
//	builder.AddScript("chmod 755 %s", dirPath)
func (b *TaskBuilder) AddScript(format string, args ...any) *TaskBuilder {
	if len(args) > 0 {
		b.scriptParts = append(b.scriptParts, fmt.Sprintf(format, args...))
	} else {
		b.scriptParts = append(b.scriptParts, format)
	}
	return b
}

// AddScriptIf conditionally appends a script fragment if the condition is true.
func (b *TaskBuilder) AddScriptIf(condition bool, format string, args ...any) *TaskBuilder {
	if condition {
		return b.AddScript(format, args...)
	}
	return b
}

// AddComment adds a comment line to the script.
func (b *TaskBuilder) AddComment(comment string) *TaskBuilder {
	b.scriptParts = append(b.scriptParts, "# "+comment)
	return b
}

// AddEcho adds an echo statement to the script.
func (b *TaskBuilder) AddEcho(message string) *TaskBuilder {
	return b.AddScript("echo %q", message)
}

// AddCD adds a cd command to change directory.
func (b *TaskBuilder) AddCD(path string) *TaskBuilder {
	return b.AddScript("cd %s", path)
}

// AddMkdir adds a mkdir -p command.
func (b *TaskBuilder) AddMkdir(path string) *TaskBuilder {
	return b.AddScript("mkdir -p %s", path)
}

// AddChown adds a chown command.
func (b *TaskBuilder) AddChown(user, path string) *TaskBuilder {
	return b.AddScript("chown %s:%s %s", user, user, path)
}

// AddChmod adds a chmod command.
func (b *TaskBuilder) AddChmod(mode, path string) *TaskBuilder {
	return b.AddScript("chmod %s %s", mode, path)
}

// AddRemove adds an rm command.
func (b *TaskBuilder) AddRemove(path string, force, recursive bool) *TaskBuilder {
	flags := ""
	if force {
		flags += "f"
	}
	if recursive {
		flags += "r"
	}
	if flags != "" {
		return b.AddScript("rm -%s %s", flags, path)
	}
	return b.AddScript("rm %s", path)
}

// AddSystemctl adds a systemctl command.
func (b *TaskBuilder) AddSystemctl(action, service string) *TaskBuilder {
	return b.AddScript("systemctl %s %s", action, service)
}

// AddHeredoc adds a heredoc to create a file with content.
// The content is safely escaped using a unique EOF marker.
func (b *TaskBuilder) AddHeredoc(path, content string) *TaskBuilder {
	return b.AddScript("cat > %s << 'HEREDOCEOF'\n%s\nHEREDOCEOF", path, content)
}

// AddHeredocAppend adds a heredoc that appends to a file.
func (b *TaskBuilder) AddHeredocAppend(path, content string) *TaskBuilder {
	return b.AddScript("cat >> %s << 'HEREDOCEOF'\n%s\nHEREDOCEOF", path, content)
}

// AddAptInstall adds apt-get install commands.
// Automatically uses apt wait if enabled.
func (b *TaskBuilder) AddAptInstall(packages ...string) *TaskBuilder {
	if b.useAptWait {
		b.AddScript("waitForAptUnlock")
	}
	return b.AddScript("apt-get install -y %s", strings.Join(packages, " "))
}

// AddAptRemove adds apt-get remove commands.
func (b *TaskBuilder) AddAptRemove(packages ...string) *TaskBuilder {
	if b.useAptWait {
		b.AddScript("waitForAptUnlock")
	}
	return b.AddScript("apt-get remove -y %s", strings.Join(packages, " "))
}

// AddConditional adds an if-then-else-fi block.
func (b *TaskBuilder) AddConditional(condition string, thenScript string, elseScript string) *TaskBuilder {
	script := fmt.Sprintf("if %s; then\n    %s", condition, thenScript)
	if elseScript != "" {
		script += fmt.Sprintf("\nelse\n    %s", elseScript)
	}
	script += "\nfi"
	b.scriptParts = append(b.scriptParts, script)
	return b
}

// AddFileExists adds a conditional that checks if a file exists.
func (b *TaskBuilder) AddFileExists(path, thenScript, elseScript string) *TaskBuilder {
	return b.AddConditional(fmt.Sprintf("[ -f %s ]", path), thenScript, elseScript)
}

// AddDirExists adds a conditional that checks if a directory exists.
func (b *TaskBuilder) AddDirExists(path, thenScript, elseScript string) *TaskBuilder {
	return b.AddConditional(fmt.Sprintf("[ -d %s ]", path), thenScript, elseScript)
}

// Build creates the BaseTask with the configured options.
func (b *TaskBuilder) Build() *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName(b.name),
		taskrunner.WithScript(b.buildScript()),
		taskrunner.WithTimeout(b.timeout),
	)
}

// buildScript assembles the final script from all parts.
func (b *TaskBuilder) buildScript() string {
	var sb strings.Builder

	// Add shebang and shell header if enabled
	if b.useShellHeader {
		_, _ = sb.WriteString("#!/bin/bash\n")
		_, _ = sb.WriteString("set -euo pipefail\n")
		_, _ = sb.WriteString("export DEBIAN_FRONTEND=noninteractive\n")
		_, _ = sb.WriteString("\n")
	}

	// Add apt wait function if enabled
	if b.useAptWait {
		_, _ = sb.WriteString(taskrunner.AptFunctions())
		_, _ = sb.WriteString("\n\n")
	}

	// Add all script parts
	for i, part := range b.scriptParts {
		_, _ = sb.WriteString(part)
		if i < len(b.scriptParts)-1 {
			_, _ = sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Script returns the built script without creating a task.
// Useful for debugging or combining with other scripts.
func (b *TaskBuilder) Script() string {
	return b.buildScript()
}

// QuickTask is a convenience function to create a simple task from a script.
func QuickTask(name string, script string, timeoutSeconds int) *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName(name),
		taskrunner.WithScript(taskrunner.WrapScript(script)),
		taskrunner.WithTimeoutSeconds(timeoutSeconds),
	)
}

// FileUploadTask creates a task that uploads content to a file on the server.
func FileUploadTask(name, path, content string, user string, mode string) *taskrunner.BaseTask {
	builder := NewTaskBuilder(name).
		WithTimeoutSeconds(30).
		AddHeredoc(path, content)

	if mode != "" {
		builder.AddChmod(mode, path)
	}

	if user != "" {
		builder.AddChown(user, path)
	}

	return builder.Build()
}

// FileDeleteTask creates a task that deletes a file from the server.
func FileDeleteTask(name, path string) *taskrunner.BaseTask {
	return NewTaskBuilder(name).
		WithTimeoutSeconds(30).
		AddFileExists(path,
			fmt.Sprintf("rm -f %s\necho \"File deleted successfully\"", path),
			"echo \"File does not exist, skipping\"",
		).
		Build()
}

// ServiceTask creates a task that performs a systemctl operation on a service.
func ServiceTask(name, action, service string) *taskrunner.BaseTask {
	return NewTaskBuilder(name).
		WithTimeoutSeconds(60).
		AddSystemctl(action, service).
		AddEcho(fmt.Sprintf("Service %s %s completed", service, action)).
		Build()
}
