package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UploadCronConfig holds configuration for uploading a cron file
type UploadCronConfig struct {
	Path     string
	Contents string
}

// UploadCron creates a task to upload a cron file to the server
func UploadCron(config UploadCronConfig) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

cat > "` + config.Path + `" << 'CRONEOF'
` + config.Contents + `
CRONEOF

chmod 644 "` + config.Path + `"
echo "Cron file uploaded successfully"
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Upload Cron File"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(30),
	)
}

// DeleteCronConfig holds configuration for deleting a cron file
type DeleteCronConfig struct {
	Path string
}

// DeleteCron creates a task to delete a cron file from the server
func DeleteCron(config DeleteCronConfig) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail

if [ -f "` + config.Path + `" ]; then
    rm -f "` + config.Path + `"
    echo "Cron file deleted successfully"
else
    echo "Cron file does not exist, skipping"
fi
`
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Cron File"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(30),
	)
}
