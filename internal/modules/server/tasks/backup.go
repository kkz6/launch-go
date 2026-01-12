package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RunBackupConfig holds configuration for running a backup
type RunBackupConfig struct {
	BackupID string
}

// RunBackup creates a task to run a backup via the Launch Agent
func RunBackup(config RunBackupConfig) *taskrunner.BaseTask {
	script := fmt.Sprintf("launch-agent perform -m %s", config.BackupID)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Run Backup"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(3600), // 1 hour for backups
	)
}

// DeleteBackupConfig holds configuration for deleting a backup
type DeleteBackupConfig struct {
	BackupID string
}

// DeleteBackup creates a task to delete a backup via the Launch Agent
func DeleteBackup(config DeleteBackupConfig) *taskrunner.BaseTask {
	script := fmt.Sprintf("launch-agent delete-backup -m %s", config.BackupID)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Backup"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// SyncLaunchConfigConfig holds configuration for syncing Launch Agent config
type SyncLaunchConfigConfig struct {
	ConfigPath string
	ConfigJSON string
}

// SyncLaunchConfig creates a task to sync the Launch Agent configuration
func SyncLaunchConfig(config SyncLaunchConfigConfig) *taskrunner.BaseTask {
	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

# Write the configuration to the Launch Agent config file
cat > %s <<'EOF'
%s
EOF

# Restart the Launch Agent to pick up the new configuration
sudo systemctl restart launch-agent

echo "Launch Agent configuration synced successfully"
`, config.ConfigPath, config.ConfigJSON)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Sync Launch Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(120),
	)
}
