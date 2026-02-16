package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for site file operations
const (
	DeleteSiteFilesTaskType = "site:delete_site_files"
)

// DeleteSiteFilesConfig holds configuration for deleting a site's files
type DeleteSiteFilesConfig struct {
	SitePath string
}

// DeleteSiteFiles creates a task to delete all files for a site
func DeleteSiteFiles(config DeleteSiteFilesConfig) *taskrunner.BaseTask {
	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

# Remove the site directory
if [ -d "%s" ]; then
    rm -rf "%s"
    echo "Site directory deleted"
else
    echo "Site directory not found, skipping"
fi
`, config.SitePath, config.SitePath)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Site Files"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}
