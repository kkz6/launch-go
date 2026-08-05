// Package templates provides common bash template functions and utilities
// for script generation across all modules.
package templates

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

// CommonFuncMap provides template functions available to all modules
var CommonFuncMap = template.FuncMap{
	"shellDefaults":        ShellDefaults,
	"shellDefaultsLenient": ShellDefaultsLenient,
	"aptFunctions":         AptFunctions,
	"commonFuncs":          CommonFunctions,
	"phpPpaFunctions":      PhpPpaFunctions,
	"taskMarkerFuncs":      TaskMarkerFunctions,
	"caddyReloadFunc":      CaddyReloadFunction,
	"dirName":              filepath.Dir,
	"join":                 strings.Join,
	"contains":             strings.Contains,
	"lower":                strings.ToLower,
	"upper":                strings.ToUpper,
	"trim":                 strings.TrimSpace,
	"replace":              strings.ReplaceAll,
	"quote": func(s string) string {
		return fmt.Sprintf("%q", s)
	},
	"escape": func(s string) string {
		return strings.ReplaceAll(s, "'", "'\"'\"'")
	},
	"default": func(defaultVal, val interface{}) interface{} {
		if val == nil || val == "" {
			return defaultVal
		}
		return val
	},
}

// ShellDefaults returns the standard shell script header with strict error handling.
// Uses set -euo pipefail for:
//   - -e: Exit on first error
//   - -u: Exit on undefined variable
//   - -o pipefail: Exit if any command in a pipeline fails
//
// Also traps SIGPIPE to prevent scripts from dying when running over SSH
// with output piped through tee. This is necessary because SSH session
// pipe closure can send SIGPIPE to the running script.
func ShellDefaults() string {
	return `set -euo pipefail
trap '' PIPE
export DEBIAN_FRONTEND=noninteractive`
}

// ShellDefaultsLenient returns a shell script header that's more lenient.
// Uses set -eu (no pipefail) for compatibility with Laravel's behavior.
// Also traps SIGPIPE to prevent scripts from dying when running over SSH.
func ShellDefaultsLenient() string {
	return `set -eu
trap '' PIPE
export DEBIAN_FRONTEND=noninteractive`
}

// CommonFunctions returns common bash helper functions.
func CommonFunctions() string {
	return `# Send a POST request to the given URL, ignoring the response and errors
function httpPostSilently() {
    if [ -z "${2:-}" ]; then
        (curl -X POST --silent --max-time 15 --output /dev/null $1 || true)
    else
        (curl -X POST --silent --max-time 15 --output /dev/null $1 -H 'Content-Type: application/json' --data "$2" || true)
    fi
}

function httpPostRawSilently() {
    (curl -X POST --silent --max-time 15 --output /dev/null $1 --data "$2" || true)
}`
}

// AptFunctions returns apt-related bash functions for Ubuntu/Debian.
//
// Why this is more than a simple "is apt running" check:
//
//  1. cloud-init on fresh DigitalOcean/Hetzner/Vultr/Contabo boxes kicks off
//     unattended-upgrades on first boot and holds the dpkg lock for several
//     minutes. `cloud-init status --wait` only works on images that ship
//     cloud-init (Contabo's minimal image doesn't), so we treat it as a
//     best-effort hint.
//
//  2. Even after the initial apt is done, the apt/dpkg locks can be grabbed
//     by `unattended-upgrades` via its systemd timer at any time. There's a
//     race between waitForAptUnlock returning and our apt-get install
//     starting where the timer can fire and grab the lock.
//
//  3. The fix used by Forge/Cleavr/etc is to stop AND mask
//     unattended-upgrades for the duration of provisioning, then re-enable
//     it at the end. We mirror that pattern here.
func AptFunctions() string {
	return `# Matched against full command lines by pgrep -f, because unattended-upgrades
# runs as a python script whose /proc/comm is truncated to "unattended-upgr".
# Anchored at both ends so it can't also match unattended-upgrade-shutdown —
# the helper unattended-upgrades.service keeps running permanently, which
# wedged every wait below until its task timed out.
APT_PROCESS_PATTERN='(^|/)(apt|apt-get|aptitude|apt\.systemd\.daily|dpkg|dpkg-deb|unattended-upgrades?)( |$)'

# Covers cloud-init's first-boot apt run without burning a whole task timeout.
APT_WAIT_TIMEOUT_SECONDS=600

function aptLockHolders() {
    pgrep -af "${APT_PROCESS_PATTERN}" 2>/dev/null || true
    sudo fuser -v /var/lib/dpkg/lock /var/lib/dpkg/lock-frontend \
                  /var/lib/apt/lists/lock /var/cache/apt/archives/lock 2>&1 || true
}

# Non-zero once the timeout is exhausted, so under set -e the script aborts
# naming the culprit instead of hanging until the task timeout kills it.
function waitForAptUnlock() {
    local waited=0
    local reason=""

    while [ "${waited}" -lt "${APT_WAIT_TIMEOUT_SECONDS}" ]; do
        if pgrep -f "${APT_PROCESS_PATTERN}" >/dev/null 2>&1; then
            reason="apt, apt-get, dpkg, or unattended-upgrade is running"
        elif sudo fuser /var/lib/dpkg/lock /var/lib/dpkg/lock-frontend \
                        /var/lib/apt/lists/lock /var/cache/apt/archives/lock \
                        >/dev/null 2>&1; then
            reason="apt lock files are still held"
        else
            if [ -n "${reason}" ]; then
                echo "apt is available after ${waited}s, continuing..."
            fi
            return 0
        fi

        if [ "${waited}" -eq 0 ]; then
            echo "Waiting for apt: ${reason}. Currently holding apt:"
            aptLockHolders
        elif [ "$((waited % 60))" -eq 0 ]; then
            echo "Still waiting for apt after ${waited}s: ${reason}"
        fi

        sleep 5
        waited=$((waited + 5))
    done

    echo "ERROR: Giving up after ${APT_WAIT_TIMEOUT_SECONDS}s waiting for apt: ${reason}." >&2
    echo "Still holding apt:" >&2
    aptLockHolders >&2
    return 1
}

# Bring the box into a state where apt-get cannot be hijacked by cloud-init
# or the unattended-upgrades systemd timer during provisioning. Called once
# at the very top of every provision script.
#
# Background:
#   - On a fresh DigitalOcean/Hetzner/Vultr Ubuntu image, cloud-init runs
#     apt update + apt upgrade as part of its first-boot work. This holds
#     the dpkg lock for several minutes.
#   - Even after cloud-init finishes, the apt-daily.timer and
#     unattended-upgrades.timer systemd units can fire at any time and
#     grab the lock mid-provision.
#
# We removed the historical "apt update + upgrade" provision step (it
# slowed every provision by 5-15 minutes and broke build reproducibility);
# the side-effect was that subsequent provision steps now hit apt-install
# much earlier in the box's lifecycle, exposing the race. This function
# is the proper fix: wait for cloud-init, then disable the timers for the
# duration of provisioning. restoreUnattendedUpgrades() puts them back
# at the end.
function quiesceAptForProvisioning() {
    # 1. Wait for cloud-init's first-boot run to finish. Silent on images
    #    that don't ship cloud-init (e.g. some Contabo minimal images).
    if command -v cloud-init >/dev/null 2>&1; then
        echo "Waiting for cloud-init to finish..."
        sudo cloud-init status --wait >/dev/null 2>&1 || true
    fi

    # 2. Stop and mask the timer-triggered apt jobs so they can't fire
    #    while we provision. We do not kill running apt processes —
    #    interrupting an in-flight unattended-upgrade can leave dpkg in
    #    an inconsistent state. waitForAptUnlock below handles that case.
    for unit in unattended-upgrades.service unattended-upgrades.timer \
                apt-daily.service apt-daily.timer \
                apt-daily-upgrade.service apt-daily-upgrade.timer; do
        sudo systemctl stop "${unit}" 2>/dev/null || true
        sudo systemctl mask "${unit}" 2>/dev/null || true
    done

    # 3. Wait for any lock-holding process that was already running to
    #    finish gracefully.
    waitForAptUnlock
}

# Re-enable the apt timer units that quiesceAptForProvisioning masked.
# Critical: only start the TIMER units, never the service units.
# Starting unattended-upgrades.service directly runs the full upgrade
# synchronously and adds 3-10 minutes to provisioning. The timer fires
# the service on its normal schedule (within 24h) — that is what we want.
function restoreUnattendedUpgrades() {
    # Unmask everything so the timers can launch their services later.
    for unit in unattended-upgrades.service unattended-upgrades.timer \
                apt-daily.service apt-daily.timer \
                apt-daily-upgrade.service apt-daily-upgrade.timer; do
        sudo systemctl unmask "${unit}" 2>/dev/null || true
    done

    # Enable + start the timers only. Each timer knows when to trigger
    # its corresponding service.
    for timer in unattended-upgrades.timer apt-daily.timer apt-daily-upgrade.timer; do
        sudo systemctl enable "${timer}" 2>/dev/null || true
        sudo systemctl start  "${timer}" 2>/dev/null || true
    done
}

export DEBIAN_FRONTEND=noninteractive
export APT_LISTCHANGES_FRONTEND=none

# Array, not an exported string: an exported string has to be expanded
# unquoted to word-split into two arguments, which silently degrades to a
# single "-o" the moment anyone quotes it.
APT_LOCK_WAIT_OPTS=(-o DPkg::Lock::Timeout=120)

# Every apt-get call in every template goes through this. waitForAptUnlock
# only closes the window before we start; the lock timeout covers a timer
# firing mid-run, which otherwise fails the task outright with "Could not
# get lock /var/lib/dpkg/lock-frontend".
function aptGet() {
    sudo DEBIAN_FRONTEND=noninteractive apt-get "${APT_LOCK_WAIT_OPTS[@]}" "$@"
}`
}

// PhpPpaFunctions returns the function that ensures the upstream PHP
// apt repo is configured. Branches on /etc/os-release ID:
//
//   - Ubuntu uses Ondrej Surý's launchpad PPA (ppa:ondrej/php). The
//     add-apt-repository tool understands this URL form natively.
//   - Debian uses the same maintainer's sury.org repo (Launchpad PPAs
//     don't work on Debian — different package archive system). We
//     wire it up manually with apt-transport-https + a fetched signing
//     key + a deb entry pointing at the box's codename.
//
// Falls back to bailing with a clear error on anything else so a
// customer running an unsupported distro sees "we don't support X"
// rather than a confusing apt failure mid-install.
func PhpPpaFunctions() string {
	return `function ensurePhpPpaInstalled() {
    . /etc/os-release
    case "${ID}" in
        ubuntu)
            if ! grep -q "ondrej/php" /etc/apt/sources.list.d/*.list 2>/dev/null && ! grep -q "ondrej/php" /etc/apt/sources.list.d/*.sources 2>/dev/null; then
                echo "Adding ondrej/php PPA (Ubuntu)..."
                waitForAptUnlock
                aptGet install -y software-properties-common
                waitForAptUnlock
                sudo DEBIAN_FRONTEND=noninteractive add-apt-repository ppa:ondrej/php -y
                waitForAptUnlock
                aptGet update -y
                echo "ondrej/php PPA installed successfully"
            else
                echo "ondrej/php PPA already installed, refreshing package lists..."
                waitForAptUnlock
                aptGet update -y
            fi
            ;;
        debian)
            if [ ! -f /etc/apt/sources.list.d/sury-php.list ]; then
                echo "Adding sury.org PHP repo (Debian)..."
                waitForAptUnlock
                aptGet install -y \
                    apt-transport-https lsb-release ca-certificates curl gnupg
                sudo install -m 0755 -d /etc/apt/keyrings
                if [ ! -f /etc/apt/keyrings/sury-php.gpg ]; then
                    curl -fsSL https://packages.sury.org/php/apt.gpg \
                        | sudo gpg --dearmor -o /etc/apt/keyrings/sury-php.gpg
                    sudo chmod a+r /etc/apt/keyrings/sury-php.gpg
                fi
                echo "deb [signed-by=/etc/apt/keyrings/sury-php.gpg] https://packages.sury.org/php/ ${VERSION_CODENAME} main" \
                    | sudo tee /etc/apt/sources.list.d/sury-php.list >/dev/null
                waitForAptUnlock
                aptGet update -y
                echo "sury.org PHP repo installed successfully"
            else
                echo "sury.org PHP repo already installed, refreshing package lists..."
                waitForAptUnlock
                aptGet update -y
            fi
            ;;
        *)
            echo "ERROR: PHP install supports only Ubuntu or Debian." >&2
            echo "  /etc/os-release reports ID=${ID:-<unset>}" >&2
            exit 1
            ;;
    esac
}`
}

// TaskMarkerFunctions returns functions for task status markers.
// These are used by the StreamMonitor to detect task status in SSH output.
// Marker format: ::LAUNCH::<type>::<value>
func TaskMarkerFunctions() string {
	return `# Task output markers for SSH streaming
# Format: ::LAUNCH::<type>::<value>

function taskExitCode() {
    local exit_code=${1:-0}
    echo "::LAUNCH::exit_code::${exit_code}"
}

function taskProgress() {
    local progress=${1:-0}
    echo "::LAUNCH::progress::${progress}"
}

function taskStatus() {
    local message="$1"
    echo "::LAUNCH::status::${message}"
}

function taskStepCompleted() {
    local step="$1"
    echo "::LAUNCH::step_completed::${step}"
}

function taskSoftwareInstalled() {
    local software="$1"
    echo "::LAUNCH::software_installed::${software}"
}

function taskError() {
    local message="$1"
    echo "::LAUNCH::error::${message}"
}`
}

// CaddyReloadFunction returns a bash function that safely reloads Caddy.
// It detects if Caddy is stuck in "reloading" state and restarts it instead.
// The reload itself is wrapped with a timeout to prevent hanging.
func CaddyReloadFunction() string {
	return `# Safely reload Caddy with stuck-state detection and timeout
function reloadCaddy() {
    # Check if Caddy is stuck in "reloading" state from a previous failed reload
    if systemctl is-active --quiet caddy && systemctl show caddy --property=ActiveState --value 2>/dev/null | grep -q "reloading"; then
        echo "Caddy is stuck in reloading state, restarting instead..."
        sudo systemctl restart caddy
        return $?
    fi

    # Attempt reload with a 30-second timeout
    if timeout 30 sudo /usr/sbin/service caddy reload 2>&1; then
        return 0
    fi

    local exit_code=$?
    echo "Caddy reload failed (exit code: $exit_code), restarting Caddy..."
    sudo systemctl restart caddy
}`
}
