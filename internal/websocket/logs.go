package websocket

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	siteModels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/support"
)

// LogsHandler handles WebSocket log streaming connections
type LogsHandler struct {
	db        *gorm.DB
	jwtSecret string
	logger    zerolog.Logger
}

// NewLogsHandler creates a new logs handler
func NewLogsHandler(db *gorm.DB, jwtSecret string, logger zerolog.Logger) *LogsHandler {
	return &LogsHandler{
		db:        db,
		jwtSecret: jwtSecret,
		logger:    logger.With().Str("component", "logs").Logger(),
	}
}

// Handler returns a Fiber handler for log streaming WebSocket connections
func (h *LogsHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Get parameters
		token := c.Query("token")
		serverID := c.Query("serverId")
		entity := c.Query("entity", "server")
		entityID := c.Query("entityId")
		software := c.Query("software")
		route := c.Query("route") // encrypted file route for site logs
		tailStr := c.Query("tail", "100")
		search := c.Query("search")
		logType := c.Query("type", "output")

		tail, _ := strconv.Atoi(tailStr)
		if tail <= 0 {
			tail = 100
		}

		if token == "" || serverID == "" {
			h.logger.Warn().Msg("Missing token or serverId")
			c.WriteMessage(websocket.TextMessage, []byte("Missing authentication or server ID"))
			c.Close()
			return
		}

		// Validate JWT
		claims, err := h.validateToken(token)
		if err != nil {
			h.logger.Warn().Err(err).Msg("Invalid token")
			c.WriteMessage(websocket.TextMessage, []byte("Authentication failed"))
			c.Close()
			return
		}

		teamID := claims["team_id"].(string)

		// Fetch server from database
		var server serverModels.Server
		if err := h.db.Where("id = ? AND team_id = ?", serverID, teamID).First(&server).Error; err != nil {
			h.logger.Error().Err(err).Str("server_id", serverID).Msg("Server not found")
			c.WriteMessage(websocket.TextMessage, []byte("Server not found"))
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
					h.logger.Error().Err(err).Msg("Failed to decode route parameter")
					c.WriteMessage(websocket.TextMessage, []byte("Invalid route parameter"))
					c.Close()
					return
				}
				logFilePath = routeData.Path
			} else if software != "" {
				// Fallback to old behavior using software parameter
				sw := enums.Software(software)
				if sw.HasLogPath() {
					logFilePath = sw.LogPath()
				} else {
					c.WriteMessage(websocket.TextMessage, []byte("Unknown software type"))
					c.Close()
					return
				}
			} else {
				c.WriteMessage(websocket.TextMessage, []byte("Missing route or software parameter"))
				c.Close()
				return
			}

		case "site":
			// Site-level logs - use route parameter if provided
			if route != "" {
				// Decode the encrypted route parameter to get the file path
				routeData, err := support.DecodeFileRouteParam(route)
				if err != nil {
					h.logger.Error().Err(err).Msg("Failed to decode route parameter")
					c.WriteMessage(websocket.TextMessage, []byte("Invalid route parameter"))
					c.Close()
					return
				}
				logFilePath = routeData.Path
			} else {
				// Fallback to old behavior using software parameter
				var site siteModels.Site
				if err := h.db.Where("id = ? AND server_id = ?", entityID, serverID).First(&site).Error; err != nil {
					c.WriteMessage(websocket.TextMessage, []byte("Site not found"))
					c.Close()
					return
				}
				logFilePath = h.getSiteLogPath(&site, software, logType)
			}

		case "queue", "daemon", "cron":
			// These entities have their own log paths stored in the database
			logFilePath, err = h.getEntityLogPath(entity, entityID, serverID, logType)
			if err != nil {
				c.WriteMessage(websocket.TextMessage, []byte("Entity not found"))
				c.Close()
				return
			}

		case "task":
			// Tasks store their output in the database
			h.streamTaskOutput(c, entityID, serverID, tail)
			return

		default:
			c.WriteMessage(websocket.TextMessage, []byte("Unknown entity type"))
			c.Close()
			return
		}

		if logFilePath == "" {
			c.WriteMessage(websocket.TextMessage, []byte("Log file path not found"))
			c.Close()
			return
		}

		h.logger.Info().
			Str("server_id", serverID).
			Str("entity", entity).
			Str("software", software).
			Str("log_path", logFilePath).
			Msg("Log streaming requested")

		// Stream logs via SSH
		h.streamLogs(c, &server, logFilePath, tail, search)
	})
}

func (h *LogsHandler) validateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(h.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func (h *LogsHandler) getSiteLogPath(site *siteModels.Site, software, logType string) string {
	logsDir := site.GetLogsDirectory()

	switch software {
	case "laravel":
		return fmt.Sprintf("%s/laravel.log", logsDir)
	case "access":
		return fmt.Sprintf("%s/access.log", logsDir)
	case "error":
		return fmt.Sprintf("%s/error.log", logsDir)
	default:
		if logType == "error" {
			return fmt.Sprintf("%s/error.log", logsDir)
		}
		return fmt.Sprintf("%s/access.log", logsDir)
	}
}

func (h *LogsHandler) getEntityLogPath(entity, entityID, serverID, logType string) (string, error) {
	switch entity {
	case "queue":
		var queue siteModels.Queue
		if err := h.db.Where("id = ? AND server_id = ?", entityID, serverID).First(&queue).Error; err != nil {
			return "", err
		}
		if logType == "error" {
			return queue.GetErrorLogPath(), nil
		}
		return queue.GetLogPath(), nil

	case "daemon":
		var daemon serverModels.Daemon
		if err := h.db.Where("id = ? AND server_id = ?", entityID, serverID).First(&daemon).Error; err != nil {
			return "", err
		}
		if logType == "error" {
			return daemon.GetErrorLogPath(), nil
		}
		return daemon.GetLogPath(), nil

	case "cron":
		var cron serverModels.Cron
		if err := h.db.Where("id = ? AND server_id = ?", entityID, serverID).First(&cron).Error; err != nil {
			return "", err
		}
		return cron.GetLogPath(), nil
	}

	return "", fmt.Errorf("unknown entity type")
}

func (h *LogsHandler) streamTaskOutput(c *websocket.Conn, taskID, serverID string, tail int) {
	var task serverModels.Task
	if err := h.db.Where("id = ? AND server_id = ?", taskID, serverID).First(&task).Error; err != nil {
		c.WriteMessage(websocket.TextMessage, []byte("Task not found"))
		c.Close()
		return
	}

	// If task is finished, just send the output and close
	if task.Status == "finished" || task.Status == "failed" {
		if !task.Output.IsEmpty() {
			c.WriteMessage(websocket.TextMessage, []byte(task.Output.String()))
		}
		c.Close()
		return
	}

	// For running tasks, we'd need to stream from the log file
	// For now, send current output
	if !task.Output.IsEmpty() {
		c.WriteMessage(websocket.TextMessage, []byte(task.Output.String()))
	}
	c.Close()
}

func (h *LogsHandler) streamLogs(c *websocket.Conn, server *serverModels.Server, logFilePath string, tail int, search string) {
	// Get SSH connection config using the server's connection method
	sshConfig := server.ConnectionAsRoot()
	if sshConfig.Host == "" {
		c.WriteMessage(websocket.TextMessage, []byte("Server has no public IP"))
		return
	}

	if sshConfig.PrivateKey == "" {
		c.WriteMessage(websocket.TextMessage, []byte("No SSH key configured"))
		return
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey([]byte(sshConfig.PrivateKey))
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse private key")
		c.WriteMessage(websocket.TextMessage, []byte("Invalid SSH key"))
		return
	}

	// SSH client config - use root for reading log files
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
		h.logger.Error().Err(err).Str("addr", addr).Msg("Failed to connect to SSH")
		c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("SSH connection failed: %s", err.Error())))
		return
	}
	defer conn.Close()

	// Create SSH session
	session, err := conn.NewSession()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to create SSH session")
		c.WriteMessage(websocket.TextMessage, []byte("Failed to create session"))
		return
	}
	defer session.Close()

	// Build the tail command
	command := fmt.Sprintf("tail -n %d -f %s 2>&1", tail, logFilePath)

	// Add grep filter if search is provided
	if search != "" {
		// Escape single quotes in search
		escapedSearch := ""
		for _, ch := range search {
			if ch == '\'' {
				escapedSearch += "'\\''"
			} else {
				escapedSearch += string(ch)
			}
		}
		command = fmt.Sprintf("%s | grep --line-buffered -iF '%s'", command, escapedSearch)
	}

	h.logger.Info().Str("command", command).Msg("Executing log tail command")

	// Get stdout pipe
	stdout, err := session.StdoutPipe()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get stdout pipe")
		return
	}

	// Start command
	if err := session.Start(command); err != nil {
		h.logger.Error().Err(err).Msg("Failed to start command")
		c.WriteMessage(websocket.TextMessage, []byte("Failed to start log streaming"))
		return
	}

	// Stream output to WebSocket
	done := make(chan struct{})

	go func() {
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
					if err := c.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
						return
					}
				}
			}
		}
	}()

	// Wait for WebSocket to close
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}

	close(done)
	session.Close()
	h.logger.Info().Msg("Log streaming ended")
}
