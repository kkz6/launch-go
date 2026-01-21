package status

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const CheckDaemonStatusTaskType = "common:check_daemon_status"

var daemonStatusScript = `#!/bin/bash
set -euo pipefail

# Get supervisorctl status and parse it
supervisorctl status 2>/dev/null | while read -r line; do
    if [ -z "$line" ]; then
        continue
    fi

    # Parse the line: program_name STATUS description
    # Example: daemon-01jvcrnjtmqpxgjasd485tjqym:daemon-01jvcrnjtmqpxgjasd485tjqym_00   RUNNING   pid 12345, uptime 2:30:15
    # Or: 01jvcrnjtmqpxgjasd485tjqym:01jvcrnjtmqpxgjasd485tjqym_00   RUNNING   pid 12345, uptime 2:30:15

    # Extract program name (first field before spaces)
    program_name=$(echo "$line" | awk '{print $1}')

    # Extract daemon ID from program name (format: {id}:{id}_00 or daemon-{id}:daemon-{id}_00)
    # The ID is everything before the colon
    daemon_id=$(echo "$program_name" | cut -d':' -f1)

    # Extract status (RUNNING, STOPPED, STARTING, FATAL, etc.)
    status=$(echo "$line" | awk '{print $2}')

    # Extract description (everything after status)
    description=$(echo "$line" | sed 's/^[^ ]*[ ]*[^ ]*[ ]*//')

    # Extract PID if running
    pid=""
    uptime_seconds=0
    error=""

    if [ "$status" = "RUNNING" ]; then
        # Description format: pid 12345, uptime 2:30:15
        pid=$(echo "$description" | grep -oP 'pid \K[0-9]+' 2>/dev/null || echo "")

        # Parse uptime (format: H:MM:SS or D days, H:MM:SS or D day, H:MM:SS)
        uptime_str=$(echo "$description" | grep -oP 'uptime \K.*' 2>/dev/null || echo "0:0:0")

        # Convert uptime to seconds
        # Handle "X day(s), H:MM:SS" format
        if [[ "$uptime_str" =~ ([0-9]+)\ day.*,\ *([0-9]+):([0-9]+):([0-9]+) ]]; then
            days=${BASH_REMATCH[1]}
            hours=${BASH_REMATCH[2]}
            mins=${BASH_REMATCH[3]}
            secs=${BASH_REMATCH[4]}
            uptime_seconds=$((days * 86400 + hours * 3600 + mins * 60 + secs))
        # Handle "H:MM:SS" format
        elif [[ "$uptime_str" =~ ^([0-9]+):([0-9]+):([0-9]+)$ ]]; then
            hours=${BASH_REMATCH[1]}
            mins=${BASH_REMATCH[2]}
            secs=${BASH_REMATCH[3]}
            uptime_seconds=$((hours * 3600 + mins * 60 + secs))
        fi
    else
        # For non-running states, description might contain error info
        error=$(echo "$description" | sed 's/^[ ]*//')
    fi

    # Escape special characters for JSON
    description_escaped=$(echo "$description" | sed 's/"/\\"/g' | sed 's/\\/\\\\/g')
    error_escaped=$(echo "$error" | sed 's/"/\\"/g' | sed 's/\\/\\\\/g')

    # Output JSON (one per line)
    printf '{"daemon_id":"%s","status":"%s","pid":"%s","uptime_seconds":%d,"description":"%s","error":"%s"}\n' \
        "$daemon_id" "$status" "$pid" "$uptime_seconds" "$description_escaped" "$error_escaped"
done

echo "===STATUS_CHECK_COMPLETE==="`

// CheckDaemonStatus creates a task to check the status of all supervisor daemons.
// The output is JSON lines that can be parsed with ParseDaemonStatusOutput.
func CheckDaemonStatus() *taskrunner.BaseTask {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check Daemon Status"),
		taskrunner.WithScript(daemonStatusScript),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// DaemonStatus represents a single daemon's status from supervisorctl
type DaemonStatus struct {
	DaemonID      string `json:"daemon_id"`
	Status        string `json:"status"`
	PID           string `json:"pid"`
	UptimeSeconds int    `json:"uptime_seconds"`
	Description   string `json:"description"`
	Error         string `json:"error"`
}

// ParseDaemonStatusOutput parses the JSON output from CheckDaemonStatus task
func ParseDaemonStatusOutput(output string) []DaemonStatus {
	var results []DaemonStatus

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and the completion marker
		if line == "" || line == "===STATUS_CHECK_COMPLETE===" {
			continue
		}

		// Try to parse as JSON
		if strings.HasPrefix(line, "{") {
			var status DaemonStatus
			if err := json.Unmarshal([]byte(line), &status); err == nil {
				results = append(results, status)
			}
		}
	}

	return results
}

// FormatDaemonUptime converts seconds to a human-readable uptime string for daemon status.
// Returns verbose format like "1 day, 2 hours" instead of compact "1d 2h".
func FormatDaemonUptime(seconds int) string {
	if seconds <= 0 {
		return ""
	}

	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	var parts []string
	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}
	if secs > 0 && len(parts) < 2 {
		// Only show seconds if we have less than 2 parts (for brevity)
		if secs == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", secs))
		}
	}

	if len(parts) == 0 {
		return "0 seconds"
	}

	return strings.Join(parts, ", ")
}
