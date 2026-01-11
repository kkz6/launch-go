{{/*
  Common shell functions for server provisioning and management scripts.
  This is a Go template file - use with text/template.
*/}}

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
