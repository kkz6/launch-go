package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// CheckDaemonStatus creates a task to check the status of all supervisor daemons
// The output is in a format that can be parsed to extract daemon status information
func CheckDaemonStatus() *taskrunner.BaseTask {
	// This script outputs JSON for each daemon program
	// Format: {"name": "daemon-xxx", "status": "RUNNING", "pid": "12345", "uptime_seconds": 3600, "description": "..."}
	script := `#!/bin/bash
set -euo pipefail

# Get supervisorctl status and parse it
supervisorctl status 2>/dev/null | while read -r line; do
    if [ -z "$line" ]; then
        continue
    fi

    # Parse the line: program_name STATUS description
    # Example: daemon-01abc123:daemon-01abc123_00   RUNNING   pid 12345, uptime 2:30:15

    # Extract program name (first field before spaces)
    program_name=$(echo "$line" | awk '{print $1}')

    # Extract daemon ID from program name (format: daemon-{id}:daemon-{id}_00)
    daemon_id=$(echo "$program_name" | sed 's/daemon-\([^:]*\).*/\1/')

    # Extract status (RUNNING, STOPPED, STARTING, etc.)
    status=$(echo "$line" | awk '{print $2}')

    # Extract description (everything after status)
    description=$(echo "$line" | sed 's/^[^ ]*[ ]*[^ ]*[ ]*//')

    # Extract PID if running
    pid=""
    uptime_seconds=0
    error=""

    if [ "$status" = "RUNNING" ]; then
        # Description format: pid 12345, uptime 2:30:15
        pid=$(echo "$description" | grep -oP 'pid \K[0-9]+' || echo "")

        # Parse uptime (format: H:MM:SS or D days, H:MM:SS)
        uptime_str=$(echo "$description" | grep -oP 'uptime \K[0-9:, days]+' || echo "0:0:0")

        # Convert uptime to seconds
        if [[ "$uptime_str" =~ ([0-9]+)" days, "([0-9]+):([0-9]+):([0-9]+) ]]; then
            days=${BASH_REMATCH[1]}
            hours=${BASH_REMATCH[2]}
            mins=${BASH_REMATCH[3]}
            secs=${BASH_REMATCH[4]}
            uptime_seconds=$((days * 86400 + hours * 3600 + mins * 60 + secs))
        elif [[ "$uptime_str" =~ ([0-9]+):([0-9]+):([0-9]+) ]]; then
            hours=${BASH_REMATCH[1]}
            mins=${BASH_REMATCH[2]}
            secs=${BASH_REMATCH[3]}
            uptime_seconds=$((hours * 3600 + mins * 60 + secs))
        fi
    else
        # For non-running states, description might contain error info
        error=$(echo "$description" | sed 's/^[ ]*//')
    fi

    # Output JSON (one per line)
    printf '{"daemon_id":"%s","status":"%s","pid":"%s","uptime_seconds":%d,"description":"%s","error":"%s"}\n' \
        "$daemon_id" "$status" "$pid" "$uptime_seconds" "$description" "$error"
done

echo "===STATUS_CHECK_COMPLETE==="
`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check Daemon Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}
