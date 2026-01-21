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
	Error    string `json:"error,omitempty"`
}

// ServiceState constants for service status
const (
	StateRunning = "running"
	StateStopped = "stopped"
	StateFailed  = "failed"
	StateUnknown = "unknown"
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

// GetSystemdServiceName maps software identifiers to their systemd service names.
// Returns an empty string if the software is not recognized.
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
	default:
		return ""
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
