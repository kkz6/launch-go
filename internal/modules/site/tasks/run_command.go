package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RunCommandConfig holds configuration for running a command on a site
type RunCommandConfig struct {
	Command string
	Site    *models.Site
}

// RunCommand creates a task to run a command on a site
func RunCommand(config RunCommandConfig) *taskrunner.BaseTask {
	// Get the application directory where the command should run
	appDir := config.Site.GetApplicationDirectory()

	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

cd %s

%s

echo "Done!"
`, appDir, config.Command)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Run Command"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300), // 5 minutes default timeout
	)
}
