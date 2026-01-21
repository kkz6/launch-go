package util

import (
	"strings"
)

// WriteConfigParams contains parameters for writing a config file with proper permissions.
type WriteConfigParams struct {
	FilePath     string // Path where the config file will be written
	Content      string // Content to write to the file
	User         string // User to own log files
	LogPath      string // Optional - creates log file if provided
	ErrorLogPath string // Optional - creates error log file if provided
	Message      string // Success message to echo
}

// WriteConfig generates a bash script that writes a config file with proper permissions.
// The generated script will:
// 1. Write content to FilePath using heredoc
// 2. chmod 644 the file
// 3. If LogPath provided: mkdir -p, touch, chown, chmod the log file
// 4. If ErrorLogPath provided: same for error log
// 5. Echo the success message
//
// Example:
//
//	script := WriteConfig(WriteConfigParams{
//	    FilePath:     "/etc/supervisor/conf.d/worker.conf",
//	    Content:      "[program:worker]\ncommand=/usr/bin/php worker.php",
//	    User:         "deploy",
//	    LogPath:      "/home/deploy/.launch/worker.log",
//	    ErrorLogPath: "/home/deploy/.launch/worker-error.log",
//	    Message:      "Worker config uploaded successfully",
//	})
func WriteConfig(params WriteConfigParams) string {
	var b strings.Builder

	b.WriteString(`#!/bin/bash
set -euo pipefail

cat > "`)
	b.WriteString(params.FilePath)
	b.WriteString(`" << 'CONFIGEOF'
`)
	b.WriteString(params.Content)
	b.WriteString(`
CONFIGEOF

chmod 644 "`)
	b.WriteString(params.FilePath)
	b.WriteString(`"
`)

	if params.LogPath != "" || params.ErrorLogPath != "" {
		b.WriteString(`
# Create .launch directory and log files if they don't exist
`)

		logPath := params.LogPath
		if logPath == "" {
			logPath = params.ErrorLogPath
		}

		b.WriteString(`LOG_DIR=$(dirname "`)
		b.WriteString(logPath)
		b.WriteString(`")
mkdir -p "$LOG_DIR"
`)

		if params.LogPath != "" {
			b.WriteString(`
if [ ! -f "`)
			b.WriteString(params.LogPath)
			b.WriteString(`" ]; then
    touch "`)
			b.WriteString(params.LogPath)
			b.WriteString(`"
    chown `)
			b.WriteString(params.User)
			b.WriteString(`:`)
			b.WriteString(params.User)
			b.WriteString(` "`)
			b.WriteString(params.LogPath)
			b.WriteString(`"
    chmod 644 "`)
			b.WriteString(params.LogPath)
			b.WriteString(`"
fi
`)
		}

		if params.ErrorLogPath != "" {
			b.WriteString(`
if [ ! -f "`)
			b.WriteString(params.ErrorLogPath)
			b.WriteString(`" ]; then
    touch "`)
			b.WriteString(params.ErrorLogPath)
			b.WriteString(`"
    chown `)
			b.WriteString(params.User)
			b.WriteString(`:`)
			b.WriteString(params.User)
			b.WriteString(` "`)
			b.WriteString(params.ErrorLogPath)
			b.WriteString(`"
    chmod 644 "`)
			b.WriteString(params.ErrorLogPath)
			b.WriteString(`"
fi
`)
		}

		b.WriteString(`
# Ensure directory ownership
chown `)
		b.WriteString(params.User)
		b.WriteString(`:`)
		b.WriteString(params.User)
		b.WriteString(` "$LOG_DIR"
`)
	}

	b.WriteString(`
echo "`)
	b.WriteString(params.Message)
	b.WriteString(`"
`)

	return b.String()
}
