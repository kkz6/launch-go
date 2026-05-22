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
	return `# Wait for any apt/dpkg/unattended-upgrade process to exit and for all
# the package-manager lock files to be released. Loop indefinitely — we'd
# rather hang the provision (visible in logs) than race the lock and
# surface a confusing failure to the customer.
#
# Note on process matching: unattended-upgrades runs as a python script
# named "unattended-upgr" (truncated to 15 chars in /proc/comm), so we
# use pgrep -f to match the full command line and catch it.
function waitForAptUnlock() {
    while pgrep -f '(^|/)(apt|apt-get|dpkg|unattended-upgrade)' >/dev/null 2>&1; do
        echo "apt, apt-get, dpkg, or unattended-upgrade is running..."
        sleep 5
    done

    while sudo fuser /var/lib/dpkg/lock /var/lib/dpkg/lock-frontend \
                     /var/lib/apt/lists/lock /var/cache/apt/archives/lock \
                     >/dev/null 2>&1; do
        echo "Waiting: apt is locked..."
        sleep 5
    done
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

# Belt-and-suspenders for apt commands: noninteractive frontend + a
# server-side dpkg lock wait so apt will retry the lock for up to 120s
# instead of failing immediately if anything still races us.
export DEBIAN_FRONTEND=noninteractive
export APT_LISTCHANGES_FRONTEND=none
export APT_LOCK_WAIT_OPTS='-o DPkg::Lock::Timeout=120'`
}

// PhpPpaFunctions returns the PHP PPA installation function for Ubuntu.
func PhpPpaFunctions() string {
	return `function ensurePhpPpaInstalled() {
    if ! grep -q "ondrej/php" /etc/apt/sources.list.d/*.list 2>/dev/null && ! grep -q "ondrej/php" /etc/apt/sources.list.d/*.sources 2>/dev/null; then
        echo "Adding ondrej/php PPA..."
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive apt-get install -y software-properties-common
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive add-apt-repository ppa:ondrej/php -y
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
        echo "ondrej/php PPA installed successfully"
    else
        echo "ondrej/php PPA already installed, refreshing package lists..."
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
    fi
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
