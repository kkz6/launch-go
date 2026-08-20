package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	siteModels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/support"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type logStreamSocket interface {
	WriteMessage(int, []byte) error
	ReadMessage() (int, []byte, error)
	SetWriteDeadline(time.Time) error
	Close() error
}

type logStreamSession interface {
	StdoutPipe() (io.Reader, error)
	Start(string) error
	Wait() error
	Signal(ssh.Signal) error
	Close() error
}

type logStreamClient interface {
	NewSession() (logStreamSession, error)
	Close() error
}

type taskrunnerLogStreamClient struct {
	client *taskrunner.SSHClient
}

func (c *taskrunnerLogStreamClient) NewSession() (logStreamSession, error) {
	return c.client.NewSession()
}

func (c *taskrunnerLogStreamClient) Close() error {
	return c.client.Close()
}

type logStreamDialer func(*taskrunner.Connection) (logStreamClient, error)

type exitStatusError interface {
	ExitStatus() int
}

const (
	defaultLogStreamWriteTimeout    = 10 * time.Second
	defaultLogStreamShutdownTimeout = 5 * time.Second
)

// LogsHandler handles WebSocket log streaming connections
type LogsHandler struct {
	Base
	dialLogStream            logStreamDialer
	logStreamWriteTimeout    time.Duration
	logStreamShutdownTimeout time.Duration
}

// NewLogsHandler creates a new logs handler
func NewLogsHandler(base Base) *LogsHandler {
	return &LogsHandler{
		Base:                     base.WithComponent("logs"),
		dialLogStream:            defaultLogStreamDialer,
		logStreamWriteTimeout:    defaultLogStreamWriteTimeout,
		logStreamShutdownTimeout: defaultLogStreamShutdownTimeout,
	}
}

func defaultLogStreamDialer(conn *taskrunner.Connection) (logStreamClient, error) {
	client, err := conn.Dial()
	if err != nil {
		return nil, err
	}
	return &taskrunnerLogStreamClient{client: client}, nil
}

// NewLogsHandlerWithDeps creates a new logs handler with individual dependencies (legacy).
// Deprecated: Use NewLogsHandler with Base instead.
func NewLogsHandlerWithDeps(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *launchcache.TeamMembershipCache) *LogsHandler {
	return NewLogsHandler(NewBase(db, jwtSecret, logger, membershipCache))
}

// Handler returns a Fiber handler for log streaming WebSocket connections
func (h *LogsHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Get parameters
		serverID := c.Query("serverId")
		entity := c.Query("entity", "server")
		entityID := c.Query("entityId")
		software := c.Query("software")
		route := c.Query("route") // encrypted file route for site logs
		tail := fiberutil.ParseTailValue(c.Query("tail", "100"))
		search := c.Query("search")
		logType := c.Query("type", "output")

		if serverID == "" {
			h.SendError(c, "Missing server ID")
			c.Close()
			return
		}

		// Authenticate using centralized auth (validates team membership)
		claims, err := h.Authenticate(c)
		if err != nil {
			h.LogError(err, "Authentication failed")
			h.SendError(c, "Authentication failed")
			c.Close()
			return
		}

		// Fetch server from database
		var server serverModels.Server
		if err := h.DB.Where("id = ? AND team_id = ?", serverID, claims.TeamID).First(&server).Error; err != nil {
			h.LogError(err, "Server not found", "server_id", serverID)
			h.SendError(c, "Server not found")
			c.Close()
			return
		}

		// Determine log file path
		var logFilePath string

		switch entity {
		case "server":
			// Server-level logs - use route parameter if provided
			if route != "" {
				// Decode the encrypted route parameter to get the file path
				routeData, err := support.DecodeFileRouteParam(route)
				if err != nil {
					h.LogError(err, "Failed to decode route parameter")
					h.SendError(c, "Invalid route parameter")
					c.Close()
					return
				}
				logFilePath = routeData.Path
			} else if software != "" {
				// Fallback to old behavior using software parameter
				sw := types.Software(software)
				if !sw.HasLogPath() {
					h.SendError(c, "Unknown software type")
					c.Close()
					return
				}
				logFilePath = sw.LogPath()
			} else {
				h.SendError(c, "Missing route or software parameter")
				c.Close()
				return
			}

		case "site":
			// Site-level logs - use route parameter if provided
			if route != "" {
				// Decode the encrypted route parameter to get the file path
				routeData, err := support.DecodeFileRouteParam(route)
				if err != nil {
					h.LogError(err, "Failed to decode route parameter")
					h.SendError(c, "Invalid route parameter")
					c.Close()
					return
				}
				logFilePath = routeData.Path
			} else {
				// Fallback to old behavior using software parameter
				var site siteModels.Site
				if err := h.DB.Where("id = ? AND server_id = ?", entityID, serverID).First(&site).Error; err != nil {
					h.SendError(c, "Site not found")
					c.Close()
					return
				}
				logFilePath = h.getSiteLogPath(&site, software, logType)
			}

		case "queue", "daemon", "cron":
			// These entities have their own log paths stored in the database
			logFilePath, err = h.getEntityLogPath(entity, entityID, serverID, logType)
			if err != nil {
				h.SendError(c, "Entity not found")
				c.Close()
				return
			}

		case "task":
			// Tasks store their output in the database
			h.streamTaskOutput(c, entityID, serverID, tail)
			return

		default:
			h.SendError(c, "Unknown entity type")
			c.Close()
			return
		}

		if logFilePath == "" {
			h.SendError(c, "Log file path not found")
			c.Close()
			return
		}

		h.LogInfo("Log streaming requested",
			"server_id", serverID,
			"entity", entity,
			"software", software,
			"log_path", logFilePath,
		)

		// Stream logs via SSH
		h.streamLogs(c, &server, logFilePath, tail, search)
	})
}

func (h *LogsHandler) getSiteLogPath(site *siteModels.Site, software, logType string) string {
	logsDir := site.GetLogsDirectory()

	switch software {
	case "laravel":
		return fmt.Sprintf("%s/laravel.log", logsDir)
	case "caddy_log":
		return fmt.Sprintf("%s/caddy.log", logsDir)
	default:
		return fmt.Sprintf("%s/caddy.log", logsDir)
	}
}

func (h *LogsHandler) getEntityLogPath(entity, entityID, serverID, logType string) (string, error) {
	switch entity {
	case "queue":
		var queue siteModels.Queue
		if err := h.DB.Where("id = ? AND server_id = ?", entityID, serverID).First(&queue).Error; err != nil {
			return "", err
		}
		if logType == "error" {
			return queue.GetErrorLogPath(), nil
		}
		return queue.GetLogPath(), nil

	case "daemon":
		var daemon serverModels.Daemon
		if err := h.DB.Where("id = ? AND server_id = ?", entityID, serverID).First(&daemon).Error; err != nil {
			return "", err
		}
		if logType == "error" {
			return daemon.GetErrorLogPath(), nil
		}
		return daemon.GetLogPath(), nil

	case "cron":
		var cron serverModels.Cron
		if err := h.DB.Where("id = ? AND server_id = ?", entityID, serverID).First(&cron).Error; err != nil {
			return "", err
		}
		return cron.GetLogPath(), nil
	}

	return "", fmt.Errorf("unknown entity type")
}

func (h *LogsHandler) streamTaskOutput(c logStreamSocket, taskID, serverID string, tail int) {
	var task serverModels.Task
	// Preload Server to get RootUsername for path calculation
	if err := h.DB.Preload("Server").Where("id = ? AND server_id = ?", taskID, serverID).First(&task).Error; err != nil {
		_ = h.sendLogStreamErrorEvent(c, "Task not found")
		_ = c.Close()
		return
	}

	// If task is finished or failed, send the stored output and close
	if task.Status == "finished" || task.Status == "failed" || task.Status == "timeout" {
		if !task.Output.IsEmpty() {
			_ = h.writeLogStreamMessage(c, []byte(task.Output.String()))
		}
		_ = c.Close()
		return
	}

	// For running/pending tasks, stream from the log file on the server
	// Use the Task's GetLogPath method which handles all path logic
	logFilePath := task.GetLogPath()

	h.LogInfo("Streaming task output",
		"task_id", taskID,
		"task_status", task.Status,
		"log_path", logFilePath,
	)

	// Stream logs via SSH (same as other entities)
	h.streamTaskLogs(c, task.Server, logFilePath, tail)
}

func buildTailCommand(logFilePath string, tail int, waitForFile bool) string {
	quotedPath := shellQuote(logFilePath)
	if !waitForFile {
		return fmt.Sprintf("tail -n %d -f %s 2>&1", tail, quotedPath)
	}

	return fmt.Sprintf(
		`attempt=0; while [ "$attempt" -lt 60 ] && [ ! -r %s ]; do attempt=$((attempt + 1)); sleep 0.5; done; if [ ! -r %s ]; then exit 42; fi; tail -n %d -f %s 2>&1`,
		quotedPath,
		quotedPath,
		tail,
		quotedPath,
	)
}

func buildLogStreamCommand(logFilePath string, tail int, search string, waitForFile bool) string {
	command := buildTailCommand(logFilePath, tail, waitForFile)
	if search == "" {
		return command
	}

	escapedSearch := ""
	for _, ch := range search {
		if ch == '\'' {
			escapedSearch += "'\\''"
		} else {
			escapedSearch += string(ch)
		}
	}
	return fmt.Sprintf("%s | grep --line-buffered -iF '%s'", command, escapedSearch)
}

func (h *LogsHandler) streamLogs(c logStreamSocket, server *serverModels.Server, logFilePath string, tail int, search string) {
	h.streamLogsWithWait(c, server, logFilePath, tail, search, false)
}

func (h *LogsHandler) streamTaskLogs(c logStreamSocket, server *serverModels.Server, logFilePath string, tail int) {
	h.streamLogsWithWait(c, server, logFilePath, tail, "", true)
}

func (h *LogsHandler) streamLogsWithWait(c logStreamSocket, server *serverModels.Server, logFilePath string, tail int, search string, waitForFile bool) {
	// Get SSH connection config using the server's connection method
	conn := server.ConnectionAsRoot()
	if conn.Host == "" {
		_ = h.sendLogStreamErrorEvent(c, "Server has no public IP")
		return
	}

	if conn.PrivateKey == "" {
		_ = h.sendLogStreamErrorEvent(c, "No SSH key configured")
		return
	}

	// Create SSH client using taskrunner
	sshClient, err := h.dialLogStream(conn)
	if err != nil {
		h.LogError(err, "Failed to connect to SSH", "host", conn.Host, "port", conn.Port)
		h.sendLogStreamError(c, "SSH connection failed: %s", err.Error())
		return
	}
	defer func() { _ = sshClient.Close() }()

	// Create SSH session
	session, err := sshClient.NewSession()
	if err != nil {
		h.LogError(err, "Failed to create SSH session")
		h.sendLogStreamError(c, "Failed to create session")
		return
	}
	command := buildLogStreamCommand(logFilePath, tail, search, waitForFile)

	h.LogInfo("Executing log tail command", "command", command)
	h.streamLogSession(c, session, command)
}

func (h *LogsHandler) streamLogSession(c logStreamSocket, session logStreamSession, command string) {
	var closeSessionOnce sync.Once
	closeSession := func() {
		closeSessionOnce.Do(func() { _ = session.Close() })
	}
	defer closeSession()

	stdout, err := session.StdoutPipe()
	if err != nil {
		h.LogError(err, "Failed to get stdout pipe")
		return
	}

	// Start command
	if err := session.Start(command); err != nil {
		h.LogError(err, "Failed to start command")
		h.sendLogStreamError(c, "Failed to start log streaming")
		return
	}

	// Stream output to WebSocket. The command completion channel is important:
	// tail exits immediately for a missing/unreadable file, and waiting only for
	// a client read would leave the UI in a permanent loading state.
	done := make(chan struct{})
	streamDone := make(chan struct{})
	var stopOnce sync.Once
	stop := func(terminate bool) {
		stopOnce.Do(func() {
			close(done)
			if terminate {
				_ = session.Signal(ssh.SIGTERM)
			}
		})
	}

	go func() {
		defer close(streamDone)
		buf := make([]byte, 8192)
		for {
			select {
			case <-done:
				return
			default:
				n, err := stdout.Read(buf)
				if err != nil {
					return
				}
				if n > 0 {
					if err := h.writeLogStreamMessage(c, buf[:n]); err != nil {
						return
					}
				}
			}
		}
	}()

	commandDone := make(chan error, 1)
	go func() {
		commandDone <- session.Wait()
	}()
	clientDone := make(chan struct{})
	go func() {
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				close(clientDone)
				return
			}
		}
	}()

	select {
	case err := <-commandDone:
		stop(false)
		closeSession()
		socketOpen := true
		if _, stopped := waitForLogStream(streamDone, h.logStreamShutdownTimeout); !stopped {
			socketOpen = false
			_ = c.Close()
			_, _ = waitForLogStream(streamDone, h.logStreamShutdownTimeout)
		}
		if err != nil && socketOpen {
			if exitErr, ok := err.(exitStatusError); ok && exitErr.ExitStatus() == 42 {
				_ = h.sendLogStreamErrorEvent(c, "Task log file is not available on the server")
			} else {
				_ = h.sendLogStreamErrorEvent(c, "Log stream ended unexpectedly")
			}
		}
		_ = c.Close()
		_, _ = waitForLogStream(clientDone, h.logStreamShutdownTimeout)
	case <-clientDone:
		stop(true)
		closeSession()
		if _, stopped := waitForLogStream(streamDone, h.logStreamShutdownTimeout); !stopped {
			_ = c.Close()
			_, _ = waitForLogStream(streamDone, h.logStreamShutdownTimeout)
		}
		_, _ = waitForLogStream(commandDone, h.logStreamShutdownTimeout)
	}
	h.LogInfo("Log streaming ended")
}

func (h *LogsHandler) sendLogStreamError(c logStreamSocket, message string, args ...any) {
	h.LogWarn(fmt.Sprintf(message, args...))
	_ = h.sendLogStreamErrorEvent(c, message, args...)
}

func (h *LogsHandler) sendLogStreamErrorEvent(c logStreamSocket, message string, args ...any) error {
	translated := fmt.Sprintf(message, args...)
	if conn, ok := c.(*websocket.Conn); ok {
		translated = h.Translate(conn, message, args...)
	}
	payload, _ := json.Marshal(WSMessage{
		Event: "error",
		Data:  map[string]string{"message": translated},
	})
	return h.writeLogStreamMessage(c, payload)
}

func (h *LogsHandler) writeLogStreamMessage(c logStreamSocket, payload []byte) error {
	if err := c.SetWriteDeadline(time.Now().Add(h.logStreamWriteTimeout)); err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, payload)
}

func waitForLogStream[T any](done <-chan T, timeout time.Duration) (T, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-done:
		return result, true
	case <-timer.C:
		var zero T
		return zero, false
	}
}
