package tasks

import (
	"encoding/base64"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for file operations
const (
	UploadFileTaskType = "server:upload_file"
	DeleteFileTaskType = "server:delete_file"
	GetFileTaskType    = "server:get_file"
)

// UploadFileConfig holds configuration for uploading a file
type UploadFileConfig struct {
	Path     string
	Contents string
	Mode     string // e.g., "644", "755"
}

// UploadFile creates a task to upload a file to the server
func UploadFile(config UploadFileConfig) *taskrunner.BaseTask {
	mode := config.Mode
	if !validFileMode(mode) {
		mode = "644"
	}

	quotedPath := taskrunner.ShellQuote(config.Path)
	encodedContents := taskrunner.ShellQuote(base64.StdEncoding.EncodeToString([]byte(config.Contents)))
	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

mkdir -p "$(dirname -- %s)"

printf '%%s' %s | base64 --decode > %s

chmod %s %s
echo "File uploaded successfully"
`, quotedPath, encodedContents, quotedPath, mode, quotedPath)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Upload File"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

func validFileMode(mode string) bool {
	if len(mode) != 3 && len(mode) != 4 {
		return false
	}
	for _, digit := range mode {
		if digit < '0' || digit > '7' {
			return false
		}
	}
	return true
}

// DeleteFileConfig holds configuration for deleting a file
type DeleteFileConfig struct {
	Path string
}

// DeleteFile creates a task to delete a file from the server
func DeleteFile(config DeleteFileConfig) *taskrunner.BaseTask {
	quotedPath := taskrunner.ShellQuote(config.Path)
	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

if [ -f %s ]; then
    rm -f -- %s
    echo "File deleted successfully"
else
    echo "File does not exist, skipping"
fi
`, quotedPath, quotedPath)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete File"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// GetFileConfig holds configuration for fetching a file
type GetFileConfig struct {
	Path     string
	MaxBytes int // Maximum bytes to fetch (default 1MB)
}

// GetFile creates a task to fetch file content from a remote server.
// Uses tail -c to fetch the last N bytes (default 1MB).
func GetFile(config GetFileConfig) *taskrunner.BaseTask {
	maxBytes := config.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024 // 1MB default
	}

	script := fmt.Sprintf("tail -c %d -- %s 2>/dev/null || echo ''", maxBytes, taskrunner.ShellQuote(config.Path))

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get File"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
