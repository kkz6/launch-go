package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	siteModels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	ws "github.com/kkz6/launch-go/internal/websocket"
)

// TerminalResizeMessage represents a terminal resize request
type TerminalResizeMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// TerminalHandler handles WebSocket SSH terminal connections
type TerminalHandler struct {
	db              *gorm.DB
	jwtSecret       string
	logger          zerolog.Logger
	membershipCache *cache.TeamMembershipCache
}

// NewTerminalHandler creates a new terminal handler
func NewTerminalHandler(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *cache.TeamMembershipCache) *TerminalHandler {
	return &TerminalHandler{
		db:              db,
		jwtSecret:       jwtSecret,
		logger:          logger.With().Str("component", "terminal").Logger(),
		membershipCache: membershipCache,
	}
}

// Handler returns a Fiber handler for terminal WebSocket connections
func (h *TerminalHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Get parameters
		serverID := c.Query("serverId")
		siteID := c.Query("siteId")
		username := c.Query("username", "launcher")

		if serverID == "" {
			h.logger.Warn().Msg("Missing serverId")
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ Missing server ID\x1b[0m\r\n"))
			c.Close()
			return
		}

		// Authenticate using centralized auth (validates team membership)
		claims, err := ws.AuthenticateWebSocket(c, h.jwtSecret, h.membershipCache)
		if err != nil {
			h.logger.Warn().Err(err).Msg("Authentication failed")
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ Authentication failed\x1b[0m\r\n"))
			c.Close()
			return
		}

		h.logger.Info().
			Str("user_id", claims.UserID).
			Str("server_id", serverID).
			Str("site_id", siteID).
			Str("username", username).
			Msg("Terminal connection requested")

		// Fetch server from database
		var server serverModels.Server
		if err := h.db.Where("id = ? AND team_id = ?", serverID, claims.TeamID).First(&server).Error; err != nil {
			h.logger.Error().Err(err).Str("server_id", serverID).Msg("Server not found")
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ Server not found\x1b[0m\r\n"))
			c.Close()
			return
		}

		// Fetch site if siteId is provided
		var sitePath string
		if siteID != "" {
			var site siteModels.Site
			if err := h.db.Where("id = ? AND server_id = ?", siteID, serverID).First(&site).Error; err != nil {
				h.logger.Error().Err(err).Str("site_id", siteID).Msg("Site not found")
				c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ Site not found\x1b[0m\r\n"))
				c.Close()
				return
			}
			sitePath = site.GetApplicationDirectory()
		}

		// Check if server is connected
		if !server.Connected {
			h.logger.Warn().Str("server_id", serverID).Msg("Server not connected")
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[33m⚠️ Server is not connected\x1b[0m\r\n"))
			c.Close()
			return
		}

		// Get SSH connection using the server's connection method
		var conn *taskrunner.Connection
		if username == "root" {
			conn = server.ConnectionAsRoot()
		} else {
			conn = server.ConnectionAsUser()
		}

		if conn.Host == "" {
			h.logger.Error().Str("server_id", serverID).Msg("Server has no public IP")
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ Server has no public IP\x1b[0m\r\n"))
			c.Close()
			return
		}

		if conn.PrivateKey == "" {
			h.logger.Error().Str("server_id", serverID).Msg("Server has no SSH key")
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ No SSH key configured\x1b[0m\r\n"))
			c.Close()
			return
		}

		// Establish SSH connection
		h.handleSSHConnection(c, conn.Host, conn.Port, conn.User, conn.PrivateKey, server.Name, sitePath)
	})
}

func (h *TerminalHandler) handleSSHConnection(wsConn *websocket.Conn, host string, port int, username, privateKey, serverName, sitePath string) {
	// Parse private key
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse private key")
		wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ Invalid SSH key\x1b[0m\r\n"))
		return
	}

	// SSH client config
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key verification
		Timeout:         10 * time.Second,
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", host, port)
	h.logger.Info().Str("addr", addr).Str("username", username).Msg("Connecting to SSH")

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		h.logger.Error().Err(err).Str("addr", addr).Msg("Failed to connect to SSH")
		wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31m❌ SSH connection failed: %s\x1b[0m\r\n", err.Error())))
		return
	}
	defer conn.Close()

	// Create SSH session
	session, err := conn.NewSession()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to create SSH session")
		wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ Failed to create session\x1b[0m\r\n"))
		return
	}
	defer session.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	// Request pseudo-terminal
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		h.logger.Error().Err(err).Msg("Failed to request pty")
		wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ Failed to allocate terminal\x1b[0m\r\n"))
		return
	}

	// Get stdin/stdout pipes
	stdin, err := session.StdinPipe()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get stdin pipe")
		return
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get stdout pipe")
		return
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get stderr pipe")
		return
	}

	// Start shell
	if err := session.Shell(); err != nil {
		h.logger.Error().Err(err).Msg("Failed to start shell")
		wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m❌ Failed to start shell\x1b[0m\r\n"))
		return
	}

	h.logger.Info().Str("server", serverName).Str("username", username).Str("sitePath", sitePath).Msg("SSH session started")

	// Send a clear screen to the terminal first
	wsConn.WriteMessage(websocket.TextMessage, []byte("\x1bc"))

	// If site path is provided, cd into it and clear the screen
	if sitePath != "" {
		initCmd := fmt.Sprintf("cd \"%s\" && export DEBIAN_FRONTEND=noninteractive && clear\n", sitePath)
		stdin.Write([]byte(initCmd))
	} else {
		stdin.Write([]byte("export DEBIAN_FRONTEND=noninteractive && clear\n"))
	}

	// Bridge WebSocket and SSH
	var wg sync.WaitGroup
	done := make(chan struct{})

	// SSH stdout -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.copyToWebSocket(wsConn, stdout, done)
	}()

	// SSH stderr -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.copyToWebSocket(wsConn, stderr, done)
	}()

	// WebSocket -> SSH stdin
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.handleWebSocketInput(wsConn, stdin, session, done)
	}()

	// Wait for session to end
	session.Wait()
	close(done)
	wg.Wait()

	h.logger.Info().Str("server", serverName).Msg("SSH session ended")
}

func (h *TerminalHandler) copyToWebSocket(wsConn *websocket.Conn, reader io.Reader, done chan struct{}) {
	buf := make([]byte, 8192)
	for {
		select {
		case <-done:
			return
		default:
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					h.logger.Debug().Err(err).Msg("SSH read error")
				}
				return
			}

			if n > 0 {
				if err := wsConn.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
					h.logger.Debug().Err(err).Msg("WebSocket write error")
					return
				}
			}
		}
	}
}

func (h *TerminalHandler) handleWebSocketInput(wsConn *websocket.Conn, stdin io.WriteCloser, session *ssh.Session, done chan struct{}) {
	for {
		select {
		case <-done:
			return
		default:
			msgType, msg, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					h.logger.Debug().Msg("WebSocket closed normally")
				} else if !isConnectionClosed(err) {
					h.logger.Debug().Err(err).Msg("WebSocket read error")
				}
				return
			}

			if msgType == websocket.TextMessage {
				// Check if it's a resize message
				var resizeMsg TerminalResizeMessage
				if err := json.Unmarshal(msg, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
					if resizeMsg.Cols > 0 && resizeMsg.Rows > 0 {
						if err := session.WindowChange(resizeMsg.Rows, resizeMsg.Cols); err != nil {
							h.logger.Debug().Err(err).Msg("Failed to resize window")
						}
					}
					continue
				}

				// Regular input - send to SSH stdin
				if _, err := stdin.Write(msg); err != nil {
					h.logger.Debug().Err(err).Msg("SSH stdin write error")
					return
				}
			}
		}
	}
}

// isConnectionClosed checks if the error is a closed connection error
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*net.OpError); ok {
		return true
	}
	return err.Error() == "use of closed network connection"
}
