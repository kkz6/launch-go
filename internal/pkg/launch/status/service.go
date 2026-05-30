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

// AgentVersionCommand returns the shell command that prints the installed
// version of a self-versioning binary, or "" if the software doesn't
// support a version probe. Used to replace the placeholder "latest"
// stored at install time with the real running version.
func AgentVersionCommand(software string) string {
	switch software {
	case "launch_agent":
		return "launch-agent --version 2>/dev/null"
	default:
		return ""
	}
}

// ParseAgentVersion extracts a version string from a CLI `--version`
// output. urfave/cli prints "<name> version <X.Y.Z>", so we take the
// last whitespace-delimited token and normalise a leading "v".
// Returns "" when nothing usable is found (caller keeps the stored value).
func ParseAgentVersion(output string) string {
	line := strings.TrimSpace(output)
	if line == "" {
		return ""
	}
	// Use the first non-empty line in case the binary prints extra noise.
	if idx := strings.IndexAny(line, "\r\n"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	v := fields[len(fields)-1]
	// Reject obviously-non-version tails (e.g. a bare "version" or an
	// error word) by requiring at least one digit.
	if !strings.ContainsAny(v, "0123456789") {
		return ""
	}
	return strings.TrimPrefix(v, "v")
}

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
