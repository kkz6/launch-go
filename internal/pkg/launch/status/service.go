package status

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ServiceStatus represents the status of a service running on a server.
type ServiceStatus struct {
	ID       string `json:"id"`
	Software string `json:"software"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	IsActive bool   `json:"is_active"`
	Memory   string `json:"memory,omitempty"`
	Uptime   string `json:"uptime,omitempty"`
	PID      int    `json:"pid,omitempty"`
	Version  string `json:"version,omitempty"`
	Error    string `json:"error,omitempty"`
}

// VersionCommand returns the shell command that prints the installed
// version of a service, or "" if we don't know how to probe it. The
// command runs over SSH as root on the target server. Used to replace
// the placeholder/static version stored at install time with the real
// running version, for EVERY service — not just self-versioning agents.
//
// Each tool formats `--version` differently; ParseVersion handles the
// variety with a single semver regex, so this only needs the right
// invocation per tool. Container-based services exec into their
// container (Traefik runs as the launch-traefik container).
func VersionCommand(software string) string {
	switch software {
	case "launch_agent":
		return "launch-agent --version 2>/dev/null"
	case "docker":
		return "docker --version 2>/dev/null"
	case "traefik":
		return "docker exec launch-traefik traefik version 2>/dev/null"
	case "supervisor":
		return "supervisord --version 2>/dev/null"
	case "redis":
		return "redis-server --version 2>/dev/null"
	case "mysql80":
		return "mysqld --version 2>/dev/null"
	case "postgresql16":
		return "psql --version 2>/dev/null"
	case "caddy2", "caddy2_lb":
		return "caddy version 2>/dev/null"
	case "composer2":
		return "composer --version 2>/dev/null"
	case "node21":
		return "node --version 2>/dev/null"
	case "bun":
		return "bun --version 2>/dev/null"
	default:
		// PHP: software key is "phpXY" → binary "phpX.Y" (php82 → php8.2).
		return phpVersionCommand(software)
	}
}

// phpVersionCommand maps a "phpXY" software key to its versioned binary's
// --version invocation, or "" if the key isn't a PHP version.
func phpVersionCommand(software string) string {
	if !strings.HasPrefix(software, "php") {
		return ""
	}
	digits := strings.TrimPrefix(software, "php")
	if len(digits) < 2 {
		return ""
	}
	// Last digit is the minor, the rest is the major: php82 → 8.2, php74 → 7.4.
	major, minor := digits[:len(digits)-1], digits[len(digits)-1:]
	return fmt.Sprintf("php%s.%s --version 2>/dev/null", major, minor)
}

// versionRe matches the first semver-ish token (MAJOR.MINOR[.PATCH]) in a
// tool's version output. Deliberately liberal so one parser covers every
// tool's format: "Docker version 27.3.1, build …", "Version: 3.1.2",
// "Redis server v=7.0.11 …", "psql (PostgreSQL) 16.2", "v21.7.3", etc.
var versionRe = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)

// ParseVersion extracts a version string from arbitrary `--version`
// output by returning the first MAJOR.MINOR[.PATCH] token. Returns ""
// when nothing usable is found (caller keeps the stored value).
func ParseVersion(output string) string {
	return versionRe.FindString(strings.TrimSpace(output))
}

// AgentVersionCommand is retained for callers/tests; it now delegates to
// the generalised VersionCommand.
func AgentVersionCommand(software string) string { return VersionCommand(software) }

// ParseAgentVersion is retained for callers/tests; it now delegates to
// the generalised ParseVersion.
func ParseAgentVersion(output string) string { return ParseVersion(output) }

// ServiceState constants for service status
const (
	StateRunning   = "running"
	StateStopped   = "stopped"
	StateFailed    = "failed"
	StateUnknown   = "unknown"
	StateInstalled = "installed"
)

// CalculateUptimeFromTimestamp calculates uptime from a timestamp string.
// Supports formats like "Mon 2006-01-02 15:04:05 UTC" and RFC3339.
// Returns an empty string if parsing fails.
func CalculateUptimeFromTimestamp(timeStr string) string {
	if timeStr == "" || timeStr == "n/a" {
		return ""
	}

	layouts := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 UTC",
		time.RFC3339,
	}

	var t time.Time
	var err error
	for _, layout := range layouts {
		t, err = time.Parse(layout, timeStr)
		if err == nil {
			break
		}
	}
	if err != nil {
		return ""
	}

	duration := time.Since(t)
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// IsDaemonService returns true if the software runs as a daemon/systemd service.
// CLI tools like Composer, Node, Bun return false as they don't have a running status.
func IsDaemonService(software string) bool {
	// Check for versioned prefixes (e.g., node21, bun1)
	nonDaemonPrefixes := []string{"node", "bun", "composer"}
	for _, prefix := range nonDaemonPrefixes {
		if strings.HasPrefix(software, prefix) {
			return false
		}
	}

	// Non-daemon services (CLI tools, package managers)
	nonDaemonServices := map[string]bool{
		"launch_agent": true,
		"git":          true,
	}

	return !nonDaemonServices[software]
}

// GetSystemdServiceName maps software identifiers to their systemd service names.
// Returns an empty string if the software is not recognized or is not a daemon service.
func GetSystemdServiceName(software string) string {
	switch {
	case strings.HasPrefix(software, "php"):
		// Extract version: php82 -> php8.2-fpm
		re := regexp.MustCompile(`php(\d)(\d)`)
		matches := re.FindStringSubmatch(software)
		if len(matches) == 3 {
			return fmt.Sprintf("php%s.%s-fpm", matches[1], matches[2])
		}
		return ""
	case software == "mysql80" || software == "mysql":
		return "mysql"
	case software == "postgresql16" || software == "postgresql":
		return "postgresql"
	case software == "redis":
		return "redis-server"
	case software == "caddy2" || software == "caddy":
		return "caddy"
	case software == "supervisor":
		return "supervisor"
	case software == "memcached":
		return "memcached"
	case software == "docker":
		// The Docker daemon runs as a real systemd unit (docker.service),
		// so `systemctl is-active docker` is the correct probe. Without
		// this case it fell through to "" and reported "Unknown".
		return "docker"
	default:
		return ""
	}
}

// GetContainerName maps software identifiers that run as Docker
// containers (rather than systemd units) to their container name, so the
// status probe can use `docker inspect` instead of `systemctl`. Returns
// "" for software that is not container-based.
//
// Traefik on a Launch docker server runs as the container `launch-traefik`
// (see server/tasks/docker_constants.go TraefikContainerName); systemctl
// can't see it, which is why it previously reported "Unknown".
func GetContainerName(software string) string {
	switch software {
	case "traefik":
		return "launch-traefik"
	default:
		return ""
	}
}

// ParseContainerState maps the output of
// `docker inspect -f '{{.State.Status}}' <container>` to our service
// state vocabulary. Docker reports one of: created, running, paused,
// restarting, removing, exited, dead (or empty when the container/daemon
// can't be inspected).
func ParseContainerState(state string) (status string, isActive bool) {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "running":
		return StateRunning, true
	case "restarting":
		// A container perpetually restarting is crash-looping — surface it
		// as failed rather than a healthy "running".
		return StateFailed, false
	case "paused", "created", "exited", "dead", "removing":
		return StateStopped, false
	default:
		// Empty output (container absent / not inspectable) or an
		// unrecognised state.
		return StateUnknown, false
	}
}

// ParseSystemctlActiveState parses the is-active result from systemctl.
// Returns the status string and isActive boolean.
func ParseSystemctlActiveState(activeResult string) (status string, isActive bool) {
	switch strings.TrimSpace(strings.ToLower(activeResult)) {
	case "active":
		return StateRunning, true
	case "inactive":
		return StateStopped, false
	case "failed":
		return StateFailed, false
	default:
		return StateUnknown, false
	}
}
