package handlers

import (
	"fmt"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// MetricsHandler handles WebSocket metrics streaming connections
type MetricsHandler struct {
	Base
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(base Base) *MetricsHandler {
	return &MetricsHandler{
		Base: base.WithComponent("metrics_stream"),
	}
}

// NewMetricsHandlerWithDeps creates a new metrics handler with individual dependencies (legacy).
// Deprecated: Use NewMetricsHandler with Base instead.
func NewMetricsHandlerWithDeps(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *cache.TeamMembershipCache) *MetricsHandler {
	return NewMetricsHandler(NewBase(db, jwtSecret, logger, membershipCache))
}

// Handler returns a Fiber handler for metrics streaming WebSocket connections
func (h *MetricsHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		serverID := c.Query("serverId")
		interval := fiberutil.ParseIntervalValue(c.Query("interval", "2"), 2)

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

		h.LogInfo("Metrics streaming requested",
			"server_id", serverID,
			"server_name", server.Name,
			"interval", interval,
		)

		// Stream metrics via SSH
		h.streamMetrics(c, &server, interval)
	})
}

func (h *MetricsHandler) sendError(c *websocket.Conn, msg string) {
	_ = SendErrorEvent(c, msg)
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

	// Create SSH session
	session, err := conn.NewSession()
	if err != nil {
		h.LogError(err, "Failed to create SSH session")
		h.sendError(c, "Failed to create session")
		return
	}
	defer session.Close()

	// Bash script that outputs JSON metrics at specified interval
	// Uses standard Linux tools: /proc/stat, free, df, /proc/loadavg, ps, /proc/net/dev
	// Uses stdbuf to disable output buffering for real-time streaming
	command := fmt.Sprintf(`stdbuf -oL bash -c '
# Collect and send system info once at start
HOSTNAME=$(hostname)
OS_NAME=$(grep "^PRETTY_NAME=" /etc/os-release 2>/dev/null | cut -d"\"" -f2 || echo "Linux")
KERNEL=$(uname -r)
CPU_MODEL=$(grep "model name" /proc/cpuinfo 2>/dev/null | head -1 | cut -d":" -f2 | sed "s/^ //" | sed "s/\"/\\\\\"/g" || echo "Unknown")
CPU_CORES=$(nproc 2>/dev/null || grep -c "^processor" /proc/cpuinfo)
TOTAL_MEM=$(free -b | awk "/^Mem:/ {print \$2}")
UPTIME_SECS=$(awk "{print int(\$1)}" /proc/uptime)

echo "{\"event\":\"system_info\",\"hostname\":\"$HOSTNAME\",\"os\":\"$OS_NAME\",\"kernel\":\"$KERNEL\",\"cpu_model\":\"$CPU_MODEL\",\"cpu_cores\":$CPU_CORES,\"total_memory\":$TOTAL_MEM,\"uptime\":$UPTIME_SECS}"

# Initialize previous network counters
PREV_RX=0
PREV_TX=0
FIRST_RUN=1

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

  # Top 5 processes by CPU usage
  PROCS=""
  while IFS= read -r line; do
    pid=$(echo "$line" | awk "{print \$2}")
    user=$(echo "$line" | awk "{print \$1}")
    cpu=$(echo "$line" | awk "{print \$3}")
    mem=$(echo "$line" | awk "{print \$4}")
    cmd=$(echo "$line" | awk "{print \$11}" | sed "s/\"/\\\\\"/g" | cut -c1-50)
    if [ -n "$PROCS" ]; then
      PROCS="$PROCS,"
    fi
    PROCS="$PROCS{\"pid\":$pid,\"user\":\"$user\",\"cpu\":$cpu,\"mem\":$mem,\"command\":\"$cmd\"}"
  done < <(ps aux --sort=-%%cpu 2>/dev/null | head -6 | tail -5)

  # Network I/O - read from /proc/net/dev (sum all interfaces except lo)
  CURR_RX=$(awk "BEGIN{rx=0} /^[[:space:]]*(eth|ens|enp|wlan|wlp|eno)[^:]*:/{rx+=\$2} END{print rx}" /proc/net/dev)
  CURR_TX=$(awk "BEGIN{tx=0} /^[[:space:]]*(eth|ens|enp|wlan|wlp|eno)[^:]*:/{tx+=\$10} END{print tx}" /proc/net/dev)

  if [ $FIRST_RUN -eq 1 ]; then
    RX_RATE=0
    TX_RATE=0
    FIRST_RUN=0
  else
    RX_RATE=$(( (CURR_RX - PREV_RX) / %d ))
    TX_RATE=$(( (CURR_TX - PREV_TX) / %d ))
    [ $RX_RATE -lt 0 ] && RX_RATE=0
    [ $TX_RATE -lt 0 ] && TX_RATE=0
  fi
  PREV_RX=$CURR_RX
  PREV_TX=$CURR_TX

  # Timestamp
  TS=$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)

  # Output JSON
  echo "{\"event\":\"metrics\",\"timestamp\":\"$TS\",\"cpu\":$CPU_PERCENT,\"load\":[$LOAD],\"memory\":{$MEM},\"disk\":{$DISK},\"processes\":[$PROCS],\"network\":{\"rx_bytes\":$CURR_RX,\"tx_bytes\":$CURR_TX,\"rx_rate\":$RX_RATE,\"tx_rate\":$TX_RATE}}"

  sleep %d
done
'`, interval, interval, interval)

	h.LogInfo("Executing metrics stream command", "interval", interval)

	// Get stdout pipe
	stdout, err := session.StdoutPipe()
	if err != nil {
		h.LogError(err, "Failed to get stdout pipe")
		h.sendError(c, "Failed to setup output stream")
		return
	}

	// Get stderr pipe for error handling
	stderr, err := session.StderrPipe()
	if err != nil {
		h.LogError(err, "Failed to get stderr pipe")
		h.sendError(c, "Failed to setup error stream")
		return
	}

	// Start command
	if err := session.Start(command); err != nil {
		h.LogError(err, "Failed to start command")
		h.sendError(c, "Failed to start metrics streaming")
		return
	}

	// Send connected event
	_ = SendEvent(c, "connected", nil)

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
					h.LogWarn("Metrics stream stderr", "stderr", string(buf[:n]))
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
	h.LogInfo("Metrics streaming ended", "server_id", server.ID)
}
