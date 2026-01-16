package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ReadComposerJSON creates a task to read composer.json from a site directory
func ReadComposerJSON(appDirectory string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`#!/bin/bash
cat %s/composer.json 2>/dev/null || echo '{}'
`, appDirectory)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Read composer.json"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
