package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	ws "github.com/kkz6/launch-go/internal/websocket"
)

// MetricsHandler handles WebSocket metrics streaming connections
type MetricsHandler struct {
	db        *gorm.DB
	jwtSecret string
	logger    zerolog.Logger
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(db *gorm.DB, jwtSecret string, logger zerolog.Logger) *MetricsHandler {
	return &MetricsHandler{
		db:        db,
		jwtSecret: jwtSecret,
		logger:    logger.With().Str("component", "metrics_stream").Logger(),
	}
}

// Handler returns a Fiber handler for metrics streaming WebSocket connections
func (h *MetricsHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		serverID := c.Query("serverId")
		intervalStr := c.Query("interval", "2")

		interval, _ := strconv.Atoi(intervalStr)
		if interval < 1 {
			interval = 1
		}
		if interval > 60 {
			interval = 60
		}

		if serverID == "" {
			h.sendError(c, "Missing serverId parameter")
			return
		}

		// Authenticate using centralized auth
		claims, err := ws.AuthenticateWebSocket(c, h.jwtSecret)
		if err != nil {
			h.logger.Warn().Err(err).Msg("Authentication failed")
			h.sendError(c, "Authentication failed")
			return
		}

		// Fetch server from database
		var server serverModels.Server
		if err := h.db.Where("id = ? AND team_id = ?", serverID, claims.TeamID).First(&server).Error; err != nil {
			h.logger.Error().Err(err).Str("server_id", serverID).Msg("Server not found")
			h.sendError(c, "Server not found")
			return
		}

		h.logger.Info().
			Str("server_id", serverID).
			Str("server_name", server.Name).
			Int("interval", interval).
			Msg("Metrics streaming requested")

		// Stream metrics via SSH
		h.streamMetrics(c, &server, interval)
	})
}

func (h *MetricsHandler) sendError(c *websocket.Conn, msg string) {
	c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"event":"error","message":"%s"}`, msg)))
	c.Close()
}

func (h *MetricsHandler) streamMetrics(c *websocket.Conn, server *serverModels.Server, interval int) {
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
		h.logger.Error().Err(err).Msg("Failed to parse private key")
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
		h.logger.Error().Err(err).Str("addr", addr).Msg("Failed to connect to SSH")
		h.sendError(c, fmt.Sprintf("SSH connection failed: %s", err.Error()))
		return
	}
	defer conn.Close()

	// Create SSH session
	session, err := conn.NewSession()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to create SSH session")
		h.sendError(c, "Failed to create session")
		return
	}
	defer session.Close()

	// Bash script that outputs JSON metrics at specified interval
	// Uses standard Linux tools: /proc/stat, free, df, /proc/loadavg
	// Uses stdbuf to disable output buffering for real-time streaming
	command := fmt.Sprintf(`stdbuf -oL bash -c '
while true; do
  # CPU - read two samples to calculate usage
  CPU1=$(head -1 /proc/stat | awk "{print \$2+\$3+\$4+\$5+\$6+\$7+\$8}")
  IDLE1=$(head -1 /proc/stat | awk "{print \$5}")
  sleep 0.2
  CPU2=$(head -1 /proc/stat | awk "{print \$2+\$3+\$4+\$5+\$6+\$7+\$8}")
  IDLE2=$(head -1 /proc/stat | awk "{print \$5}")
  CPU_DIFF=$((CPU2-CPU1))
  IDLE_DIFF=$((IDLE2-IDLE1))
  if [ $CPU_DIFF -gt 0 ]; then
    CPU_PERCENT=$(awk "BEGIN {printf \"%%.1f\", (1-$IDLE_DIFF/$CPU_DIFF)*100}")
  else
    CPU_PERCENT="0.0"
  fi

  # Memory
  MEM=$(free -b | awk "/^Mem:/ {printf \"\\\"total\\\":%%s,\\\"used\\\":%%s,\\\"free\\\":%%s,\\\"percent\\\":%%.1f\", \$2, \$3, \$4, (\$3/\$2)*100}")

  # Disk
  DISK=$(df -B1 / | awk "NR==2 {gsub(\"%%\",\"\"); printf \"\\\"total\\\":%%s,\\\"used\\\":%%s,\\\"free\\\":%%s,\\\"percent\\\":%%s\", \$2, \$3, \$4, \$5}")

  # Load
  read load1 load5 load15 rest < /proc/loadavg
  LOAD="$load1,$load5,$load15"

  # Timestamp
  TS=$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)

  # Output JSON
  echo "{\"event\":\"metrics\",\"timestamp\":\"$TS\",\"cpu\":$CPU_PERCENT,\"load\":[$LOAD],\"memory\":{$MEM},\"disk\":{$DISK}}"

  sleep %d
done
'`, interval)

	h.logger.Info().Int("interval", interval).Msg("Executing metrics stream command")

	// Get stdout pipe
	stdout, err := session.StdoutPipe()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get stdout pipe")
		h.sendError(c, "Failed to setup output stream")
		return
	}

	// Get stderr pipe for error handling
	stderr, err := session.StderrPipe()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get stderr pipe")
		h.sendError(c, "Failed to setup error stream")
		return
	}

	// Start command
	if err := session.Start(command); err != nil {
		h.logger.Error().Err(err).Msg("Failed to start command")
		h.sendError(c, "Failed to start metrics streaming")
		return
	}

	// Send connected event
	c.WriteMessage(websocket.TextMessage, []byte(`{"event":"connected"}`))

	// Stream output to WebSocket
	done := make(chan struct{})

	// Read stdout (metrics data)
	go func() {
		buf := make([]byte, 4096)
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

	// Read stderr (errors)
	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-done:
				return
			default:
				n, err := stderr.Read(buf)
				if err != nil {
					return
				}
				if n > 0 {
					h.logger.Warn().Str("stderr", string(buf[:n])).Msg("Metrics stream stderr")
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
	session.Signal(ssh.SIGTERM)
	session.Close()
	h.logger.Info().Str("server_id", server.ID).Msg("Metrics streaming ended")
}
