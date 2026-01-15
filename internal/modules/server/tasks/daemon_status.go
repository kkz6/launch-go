package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// CheckDaemonStatus creates a task to check the status of all supervisor daemons
// The output is in a format that can be parsed to extract daemon status information
func CheckDaemonStatus() *taskrunner.BaseTask {
	// This script outputs JSON for each daemon program
	// Supervisor output format: {program_name}:{program_name}_00   RUNNING   pid 12345, uptime 2:30:15
	// Program names can be either "{id}" (for queues) or "daemon-{id}" (for daemons)
	script := `#!/bin/bash
set -euo pipefail

# Get supervisorctl status and parse it
supervisorctl status 2>/dev/null | while read -r line; do
    if [ -z "$line" ]; then
        continue
    fi

    # Parse the line: program_name STATUS description
    # Example: daemon-01jvcrnjtmqpxgjasd485tjqym:daemon-01jvcrnjtmqpxgjasd485tjqym_00   RUNNING   pid 12345, uptime 2:30:15

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

echo "===STATUS_CHECK_COMPLETE==="
`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check Daemon Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}
