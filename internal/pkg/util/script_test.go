package util

import (
	"strings"
	"testing"
)

func TestWriteConfig_WithAllOptions(t *testing.T) {
	params := WriteConfigParams{
		FilePath:     "/etc/supervisor/conf.d/worker.conf",
		Content:      "[program:worker]\ncommand=/usr/bin/php worker.php",
		User:         "deploy",
		LogPath:      "/home/deploy/.launch/worker.log",
		ErrorLogPath: "/home/deploy/.launch/worker-error.log",
		Message:      "Worker config uploaded successfully",
	}

	result := WriteConfig(params)

	expectedParts := []string{
		`#!/bin/bash`,
		`set -euo pipefail`,
		`cat > "/etc/supervisor/conf.d/worker.conf" << 'CONFIGEOF'`,
		`[program:worker]`,
		`command=/usr/bin/php worker.php`,
		`CONFIGEOF`,
		`chmod 644 "/etc/supervisor/conf.d/worker.conf"`,
		`LOG_DIR=$(dirname "/home/deploy/.launch/worker.log")`,
		`mkdir -p "$LOG_DIR"`,
		`touch "/home/deploy/.launch/worker.log"`,
		`chown deploy:deploy "/home/deploy/.launch/worker.log"`,
		`chmod 644 "/home/deploy/.launch/worker.log"`,
		`touch "/home/deploy/.launch/worker-error.log"`,
		`chown deploy:deploy "/home/deploy/.launch/worker-error.log"`,
		`chmod 644 "/home/deploy/.launch/worker-error.log"`,
		`chown deploy:deploy "$LOG_DIR"`,
		`echo "Worker config uploaded successfully"`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("expected script to contain %q, but it didn't\nScript:\n%s", part, result)
		}
	}
}

func TestWriteConfig_WithLogPathOnly(t *testing.T) {
	params := WriteConfigParams{
		FilePath: "/etc/cron.d/backup",
		Content:  "0 * * * * /usr/bin/backup.sh",
		User:     "root",
		LogPath:  "/var/log/backup.log",
		Message:  "Cron file uploaded successfully",
	}

	result := WriteConfig(params)

	if !strings.Contains(result, `touch "/var/log/backup.log"`) {
		t.Error("expected script to create log file")
	}

	if strings.Contains(result, "worker-error.log") {
		t.Error("expected script to NOT contain error log references")
	}
}

func TestWriteConfig_WithErrorLogPathOnly(t *testing.T) {
	params := WriteConfigParams{
		FilePath:     "/etc/supervisor/conf.d/app.conf",
		Content:      "[program:app]",
		User:         "www-data",
		ErrorLogPath: "/var/log/app-error.log",
		Message:      "Config uploaded",
	}

	result := WriteConfig(params)

	if !strings.Contains(result, `LOG_DIR=$(dirname "/var/log/app-error.log")`) {
		t.Error("expected script to use error log path for directory")
	}

	if !strings.Contains(result, `touch "/var/log/app-error.log"`) {
		t.Error("expected script to create error log file")
	}
}

func TestWriteConfig_WithoutLogPaths(t *testing.T) {
	params := WriteConfigParams{
		FilePath: "/etc/nginx/sites-available/default",
		Content:  "server { listen 80; }",
		User:     "www-data",
		Message:  "Nginx config uploaded",
	}

	result := WriteConfig(params)

	if strings.Contains(result, "LOG_DIR") {
		t.Error("expected script to NOT contain log directory setup")
	}

	if strings.Contains(result, "mkdir -p") {
		t.Error("expected script to NOT contain mkdir command")
	}

	if !strings.Contains(result, `chmod 644 "/etc/nginx/sites-available/default"`) {
		t.Error("expected script to chmod the config file")
	}

	if !strings.Contains(result, `echo "Nginx config uploaded"`) {
		t.Error("expected script to echo success message")
	}
}
