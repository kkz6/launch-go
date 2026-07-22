package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/launch/status"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ServiceStatusMessage is the WebSocket message format
type ServiceStatusMessage struct {
	Event    string                 `json:"event"`
	Services []status.ServiceStatus `json:"services"`
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
func NewServiceStatusHandlerWithDeps(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *launchcache.TeamMembershipCache) *ServiceStatusHandler {
	return NewServiceStatusHandler(NewBase(db, jwtSecret, logger, membershipCache))
}

// Handler returns a Fiber handler for service status WebSocket connections
func (h *ServiceStatusHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Get parameters
		serverID := c.Query("serverId")
		serviceID := c.Query("serviceId") // Optional: monitor specific service
		interval := fiberutil.ParseIntValue(c.Query("interval", "5"), 5, 2, 60)

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
	_ = SendErrorEvent(c, msg)
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

	conn, err := sshConfig.Dial(10 * time.Second)
	if err != nil {
		h.LogError(err, "Failed to connect to SSH", "server_id", server.ID)
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
			if !isSSHConnectionHealthy(conn) {
				_ = conn.Close()
				conn, err = sshConfig.Dial(10 * time.Second)
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

func (h *ServiceStatusHandler) checkAndSendStatus(c *websocket.Conn, conn *taskrunner.SSHClient, services []serverModels.InstalledService) {
	statuses := make([]status.ServiceStatus, 0, len(services))

	for _, svc := range services {
		svcStatus := h.getServiceStatus(conn, &svc)
		statuses = append(statuses, svcStatus)
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

func (h *ServiceStatusHandler) getServiceStatus(conn *taskrunner.SSHClient, svc *serverModels.InstalledService) status.ServiceStatus {
	svcStatus := status.ServiceStatus{
		ID:       svc.ID,
		Software: svc.Software,
		Name:     svc.Name,
		Status:   status.StateUnknown,
		IsActive: false,
	}

	// Probe the real installed version for ANY service that exposes a
	// version command — runs regardless of how status is determined below
	// (CLI / container / systemd). Self-heals the install-time placeholder
	// (e.g. supervisor/redis stored as "latest", or a stale static default
	// like docker "27.3") with what's actually on the box.
	if vcmd := status.VersionCommand(svc.Software); vcmd != "" {
		h.probeAndPersistVersion(conn, svc, &svcStatus, vcmd)
	}

	// Check if this is a non-daemon service (CLI tool, package manager)
	if !status.IsDaemonService(svc.Software) {
		svcStatus.Status = status.StateInstalled
		return svcStatus
	}

	// Container-based services (e.g. Traefik) run as Docker containers,
	// not systemd units, so probe them with `docker inspect`. Without
	// this branch they fell through to GetSystemdServiceName=="" and
	// always reported "Unknown".
	if containerName := status.GetContainerName(svc.Software); containerName != "" {
		return h.getContainerServiceStatus(conn, svcStatus, containerName)
	}

	// Determine the systemd service name based on software
	serviceName := status.GetSystemdServiceName(svc.Software)
	if serviceName == "" {
		svcStatus.Status = status.StateUnknown
		svcStatus.Error = "Unknown service type"
		return svcStatus
	}

	// Create SSH session
	session, err := conn.NewSession()
	if err != nil {
		svcStatus.Error = "Failed to create SSH session"
		return svcStatus
	}
	defer session.Close()

	// Run systemctl command to get service status
	command := fmt.Sprintf(`systemctl is-active %s 2>/dev/null; echo "---"; systemctl show %s --property=ActiveState,SubState,MainPID,MemoryCurrent,ActiveEnterTimestamp 2>/dev/null`, serviceName, serviceName)

	output, err := session.CombinedOutput(command)
	if err != nil {
		// Check if it's just because the service is not active - only set error otherwise
		if !strings.Contains(string(output), "inactive") && !strings.Contains(string(output), "failed") {
			svcStatus.Error = fmt.Sprintf("Command failed: %v", err)
		}
	}

	// Parse the output
	h.parseServiceOutput(string(output), &svcStatus)

	return svcStatus
}

// probeAndPersistVersion runs a binary's `--version` over SSH, sets the
// live status's Version, and persists it to the service row when it
// differs from the stored value (self-healing the install-time
// "latest" placeholder). Best-effort: failures are logged, never fatal
// to the status stream.
func (h *ServiceStatusHandler) probeAndPersistVersion(
	conn *taskrunner.SSHClient, svc *serverModels.InstalledService, svcStatus *status.ServiceStatus, versionCmd string,
) {
	session, err := conn.NewSession()
	if err != nil {
		return
	}
	defer session.Close()

	output, _ := session.CombinedOutput(versionCmd)
	v := status.ParseAgentVersion(string(output))
	if v == "" {
		return
	}
	svcStatus.Version = v

	if svc.Version != v {
		if err := h.DB.Model(&serverModels.InstalledService{}).
			Where("id = ?", svc.ID).
			Update("version", v).Error; err != nil {
			h.Logger.Warn().Err(err).Str("service_id", svc.ID).
				Msg("failed to persist agent version")
		}
	}
}

// getContainerServiceStatus probes a container-based service (Traefik)
// via `docker inspect`. The status + uptime come from the container's
// State, not systemd. The launch SSH user already has docker access
// (deploys run docker commands over the same connection).
func (h *ServiceStatusHandler) getContainerServiceStatus(
	conn *taskrunner.SSHClient, svcStatus status.ServiceStatus, containerName string,
) status.ServiceStatus {
	session, err := conn.NewSession()
	if err != nil {
		svcStatus.Error = "Failed to create SSH session"
		return svcStatus
	}
	defer session.Close()

	// State.Status → running/exited/... ; State.StartedAt for uptime.
	command := fmt.Sprintf(
		`docker inspect -f '{{.State.Status}}|{{.State.StartedAt}}|{{.State.Pid}}' %s 2>/dev/null`,
		containerName,
	)
	output, _ := session.CombinedOutput(command)

	fields := strings.SplitN(strings.TrimSpace(string(output)), "|", 3)
	state := ""
	if len(fields) > 0 {
		state = fields[0]
	}
	svcStatus.Status, svcStatus.IsActive = status.ParseContainerState(state)

	if svcStatus.IsActive && len(fields) >= 2 {
		svcStatus.Uptime = status.CalculateUptimeFromTimestamp(strings.TrimSpace(fields[1]))
	}
	if len(fields) >= 3 {
		if pid, perr := strconv.Atoi(strings.TrimSpace(fields[2])); perr == nil && pid > 0 {
			svcStatus.PID = pid
		}
	}
	return svcStatus
}

func isSSHConnectionHealthy(conn *taskrunner.SSHClient) bool {
	session, err := conn.NewSession()
	if err != nil {
		return false
	}
	return session.Close() == nil
}

func (h *ServiceStatusHandler) parseServiceOutput(output string, svcStatus *status.ServiceStatus) {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return
	}

	// First line is the is-active result
	activeResult := strings.TrimSpace(lines[0])
	svcStatus.Status, svcStatus.IsActive = status.ParseSystemctlActiveState(activeResult)

	// Parse the property lines
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MainPID=") {
			pidStr := strings.TrimPrefix(line, "MainPID=")
			if pid, err := strconv.Atoi(pidStr); err == nil {
				svcStatus.PID = pid
			}
		} else if strings.HasPrefix(line, "MemoryCurrent=") {
			memStr := strings.TrimPrefix(line, "MemoryCurrent=")
			if memStr != "[not set]" && memStr != "" {
				svcStatus.Memory = status.ParseBytesString(memStr)
			}
		} else if strings.HasPrefix(line, "ActiveEnterTimestamp=") {
			timeStr := strings.TrimPrefix(line, "ActiveEnterTimestamp=")
			if timeStr != "" && timeStr != "n/a" {
				svcStatus.Uptime = status.CalculateUptimeFromTimestamp(timeStr)
			}
		}
	}
}
