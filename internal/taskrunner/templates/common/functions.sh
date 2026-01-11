{{/*
  Common shell functions for server provisioning and management scripts.
  This is a Go template file - use with text/template.
*/}}

# ==============================================================================
# Task Output Markers (for SSH streaming in local development)
# ==============================================================================

# These markers are parsed by the StreamMonitor to detect task status
# without requiring HTTP callbacks (useful for local development)

# Signal that task has started
function taskStarted()
{
    echo "::LAUNCH_TASK_STARTED::"
}

# Signal task completion with exit code
function taskFinished()
{
    local exit_code=${1:-0}
    echo "::LAUNCH_EXIT_CODE::${exit_code}"
    echo "::LAUNCH_TASK_FINISHED::"
}

# Signal task failure with exit code
function taskFailed()
{
    local exit_code=${1:-1}
    echo "::LAUNCH_EXIT_CODE::${exit_code}"
    echo "::LAUNCH_TASK_FAILED::"
}

# Report progress (0-100)
function taskProgress()
{
    local progress=${1:-0}
    echo "::LAUNCH_TASK_PROGRESS::${progress}"
}

# Report status message
function taskStatus()
{
    local message="$1"
    echo "::LAUNCH_TASK_STATUS::${message}"
}

# Wrapper to run a command and report result
function runAndReport()
{
    "$@"
    local exit_code=$?
    if [ $exit_code -eq 0 ]; then
        return 0
    else
        taskFailed $exit_code
        exit $exit_code
    fi
}

# ==============================================================================
# HTTP Functions
# ==============================================================================

# Send a POST request to the given URL, ignoring the response and errors
function httpPostSilently()
{
    if [ -z "${2:-}" ]; then
        (curl -X POST --silent --max-time 15 --output /dev/null $1 || true)
    else
        (curl -X POST --silent --max-time 15 --output /dev/null $1 -H 'Content-Type: application/json' --data $2 || true)
    fi
}

function httpPostRawSilently()
{
    (curl -X POST --silent --max-time 15 --output /dev/null $1 --data "$2" || true)
}

# ==============================================================================
# APT Functions
# ==============================================================================

# Wait for apt to be unlocked
function waitForAptUnlock()
{
    while ps -C apt,apt-get,dpkg >/dev/null 2>&1; do
        echo "apt, apt-get or dpkg is running..."
        sleep 5
    done

    while fuser /var/{lib/{dpkg,apt/lists},cache/apt/archives}/{lock,lock-frontend} >/dev/null 2>&1; do
        echo "Waiting: apt is locked..."
        sleep 5
    done

    if [ -f /var/log/unattended-upgrades/unattended-upgrades.log ]; then
        while fuser /var/log/unattended-upgrades/unattended-upgrades.log >/dev/null 2>&1; do
        echo "Waiting: unattended-upgrades is locked..."
        sleep 5
        done
    fi
}
