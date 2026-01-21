package handlers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/cache"
)

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	ID       string `json:"id"`
	Software string `json:"software"`
	Name     string `json:"name"`
	Status   string `json:"status"` // running, stopped, failed, unknown
	IsActive bool   `json:"is_active"`
	Memory   string `json:"memory,omitempty"`
	Uptime   string `json:"uptime,omitempty"`
	PID      int    `json:"pid,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ServiceStatusMessage is the WebSocket message format
type ServiceStatusMessage struct {
	Event    string          `json:"event"`
	Services []ServiceStatus `json:"services"`
}

// ServiceStatusHandler handles WebSocket service status streaming connections
type ServiceStatusHandler struct {
	Base
}

// NewServiceStatusHandler creates a new service status handler
func NewServiceStatusHandler(base Base) *ServiceStatusHandler {
	return &ServiceStatusHandler{
		Base: base.WithComponent("service_status"),
	}
}

// NewServiceStatusHandlerWithDeps creates a new service status handler with individual dependencies (legacy).
// Deprecated: Use NewServiceStatusHandler with Base instead.
func NewServiceStatusHandlerWithDeps(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *cache.TeamMembershipCache) *ServiceStatusHandler {
	return NewServiceStatusHandler(NewBase(db, jwtSecret, logger, membershipCache))
}

// Handler returns a Fiber handler for service status WebSocket connections
func (h *ServiceStatusHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Get parameters
		serverID := c.Query("serverId")
		serviceID := c.Query("serviceId")       // Optional: monitor specific service
		intervalStr := c.Query("interval", "5") // Polling interval in seconds

		interval, _ := strconv.Atoi(intervalStr)
		if interval < 2 {
			interval = 2 // Minimum 2 seconds
		}
		if interval > 60 {
			interval = 60 // Maximum 60 seconds
		}

		if serverID == "" {
			h.sendError(c, "Missing serverId parameter")
			return
		}

		// Authenticate using centralized auth (validates team membership)
		claims, err := h.Authenticate(c)
		if err != nil {
			h.LogError(err, "Authentication failed")
			h.sendError(c, "Authentication failed")
			return
		}

		// Fetch server from database
		var server serverModels.Server
		if err := h.DB.Where("id = ? AND team_id = ?", serverID, claims.TeamID).First(&server).Error; err != nil {
			h.LogError(err, "Server not found", "server_id", serverID)
			h.sendError(c, "Server not found")
			return
		}

		// Fetch services to monitor
		var services []serverModels.InstalledService
		query := h.DB.Where("server_id = ?", serverID)
		if serviceID != "" {
			query = query.Where("id = ?", serviceID)
		}
		if err := query.Find(&services).Error; err != nil {
			h.LogError(err, "Failed to fetch services")
			h.sendError(c, "Failed to fetch services")
			return
		}

		if len(services) == 0 {
			h.sendError(c, "No services found")
			return
		}

		h.LogInfo("Service status monitoring started",
			"server_id", serverID,
			"service_id", serviceID,
			"service_count", len(services),
			"interval", interval,
		)

		// Start monitoring loop
		h.monitorServices(c, &server, services, time.Duration(interval)*time.Second)
	})
}

func (h *ServiceStatusHandler) sendError(c *websocket.Conn, msg string) {
	data, _ := json.Marshal(map[string]string{"event": "error", "message": msg})
	c.WriteMessage(websocket.TextMessage, data)
	c.Close()
}

func (h *ServiceStatusHandler) monitorServices(c *websocket.Conn, server *serverModels.Server, services []serverModels.InstalledService, interval time.Duration) {
	// Get SSH connection config
	sshConfig := server.ConnectionAsRoot()
	if sshConfig.Host == "" {
		h.sendError(c, "Server has no public IP")
		return
	}

	if sshConfig.PrivateKey == "" {
		h.sendError(c, "No SSH key configured")
		return
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey([]byte(sshConfig.PrivateKey))
	if err != nil {
		h.LogError(err, "Failed to parse private key")
		h.sendError(c, "Invalid SSH key")
		return
	}

	// SSH client config
	config := &ssh.ClientConfig{
		User: sshConfig.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", sshConfig.Host, sshConfig.Port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		h.LogError(err, "Failed to connect to SSH", "addr", addr)
		h.sendError(c, fmt.Sprintf("SSH connection failed: %s", err.Error()))
		return
	}
	defer conn.Close()

	// Send initial status
	h.checkAndSendStatus(c, conn, services)

	// Create ticker for periodic checks
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Channel to detect WebSocket close
	done := make(chan struct{})
	go func() {
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				close(done)
				return
			}
		}
	}()

	// Monitoring loop
	for {
		select {
		case <-done:
			h.LogInfo("Service status monitoring ended - client disconnected")
			return
		case <-ticker.C:
			// Check if SSH connection is still alive
			if _, _, err := conn.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				// Try to reconnect
				conn, err = ssh.Dial("tcp", addr, config)
				if err != nil {
					h.LogError(err, "Failed to reconnect SSH")
					h.sendError(c, "SSH connection lost")
					return
				}
			}
			h.checkAndSendStatus(c, conn, services)
		}
	}
}

func (h *ServiceStatusHandler) checkAndSendStatus(c *websocket.Conn, conn *ssh.Client, services []serverModels.InstalledService) {
	statuses := make([]ServiceStatus, 0, len(services))

	for _, svc := range services {
		status := h.getServiceStatus(conn, &svc)
		statuses = append(statuses, status)
	}

	msg := ServiceStatusMessage{
		Event:    "service.status",
		Services: statuses,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.LogError(err, "Failed to marshal status message")
		return
	}

	if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
		h.LogError(err, "Failed to send status message")
	}
}

func (h *ServiceStatusHandler) getServiceStatus(conn *ssh.Client, svc *serverModels.InstalledService) ServiceStatus {
	status := ServiceStatus{
		ID:       svc.ID,
		Software: svc.Software,
		Name:     svc.Name,
		Status:   "unknown",
		IsActive: false,
	}

	// Determine the systemd service name based on software
	serviceName := h.getSystemdServiceName(svc.Software)
	if serviceName == "" {
		status.Status = "unknown"
		status.Error = "Unknown service type"
		return status
	}

	// Create SSH session
	session, err := conn.NewSession()
	if err != nil {
		status.Error = "Failed to create SSH session"
		return status
	}
	defer session.Close()

	// Run systemctl command to get service status
	command := fmt.Sprintf(`systemctl is-active %s 2>/dev/null; echo "---"; systemctl show %s --property=ActiveState,SubState,MainPID,MemoryCurrent,ActiveEnterTimestamp 2>/dev/null`, serviceName, serviceName)

	output, err := session.CombinedOutput(command)
	if err != nil {
		// Check if it's just because the service is not active - only set error otherwise
		if !strings.Contains(string(output), "inactive") && !strings.Contains(string(output), "failed") {
			status.Error = fmt.Sprintf("Command failed: %v", err)
		}
	}

	// Parse the output
	h.parseServiceOutput(string(output), &status)

	return status
}

func (h *ServiceStatusHandler) parseServiceOutput(output string, status *ServiceStatus) {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return
	}

	// First line is the is-active result
	activeResult := strings.TrimSpace(lines[0])
	switch activeResult {
	case "active":
		status.Status = "running"
		status.IsActive = true
	case "inactive":
		status.Status = "stopped"
		status.IsActive = false
	case "failed":
		status.Status = "failed"
		status.IsActive = false
	default:
		status.Status = "unknown"
		status.IsActive = false
	}

	// Parse the property lines
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MainPID=") {
			pidStr := strings.TrimPrefix(line, "MainPID=")
			if pid, err := strconv.Atoi(pidStr); err == nil {
				status.PID = pid
			}
		} else if strings.HasPrefix(line, "MemoryCurrent=") {
			memStr := strings.TrimPrefix(line, "MemoryCurrent=")
			if memStr != "[not set]" && memStr != "" {
				status.Memory = formatBytes(memStr)
			}
		} else if strings.HasPrefix(line, "ActiveEnterTimestamp=") {
			timeStr := strings.TrimPrefix(line, "ActiveEnterTimestamp=")
			if timeStr != "" && timeStr != "n/a" {
				status.Uptime = calculateUptime(timeStr)
			}
		}
	}
}

func (h *ServiceStatusHandler) getSystemdServiceName(software string) string {
	// Map software to systemd service names
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

// formatBytes converts a byte string to human-readable format
func formatBytes(byteStr string) string {
	bytes, err := strconv.ParseInt(byteStr, 10, 64)
	if err != nil {
		return byteStr
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// calculateUptime calculates uptime from a timestamp string
func calculateUptime(timeStr string) string {
	// Parse timestamp like "Mon 2024-01-15 10:30:00 UTC"
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
