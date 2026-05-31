package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	siteModels "github.com/kkz6/launch-go/internal/modules/site/models"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const (
	// Ping interval for WebSocket keepalive
	terminalPingInterval = 30 * time.Second
	// Pong wait deadline - connection closed if no pong received within this duration
	terminalPongWait = 60 * time.Second
	// Write wait for control messages
	terminalWriteWait = 10 * time.Second
)

// TerminalResizeMessage represents a terminal resize request
type TerminalResizeMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// TerminalHandler handles WebSocket SSH terminal connections
type TerminalHandler struct {
	Base
}

// NewTerminalHandler creates a new terminal handler
func NewTerminalHandler(base Base) *TerminalHandler {
	return &TerminalHandler{
		Base: base.WithComponent("terminal"),
	}
}

// NewTerminalHandlerWithDeps creates a new terminal handler with individual dependencies (legacy).
// Deprecated: Use NewTerminalHandler with Base instead.
func NewTerminalHandlerWithDeps(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *launchcache.TeamMembershipCache) *TerminalHandler {
	return NewTerminalHandler(NewBase(db, jwtSecret, logger, membershipCache))
}

// Handler returns a Fiber handler for terminal WebSocket connections
func (h *TerminalHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Get parameters
		serverID := c.Query("serverId")
		siteID := c.Query("siteId")
		// container, when set, switches the session from a plain login
		// shell to `docker exec -it <container> sh` — used by the
		// workload detail pages so the Terminal button opens the
		// container's shell rather than the host's root shell.
		containerName := c.Query("container")
		// shell is the program to exec inside the container. We accept
		// "bash" or "sh" (mirrors dokploy's terminal modal). Empty =>
		// auto-detect (try bash, fall back to sh). The auto path wraps
		// the command in `sh -c '… bash || sh'`, which itself breaks if
		// the container has no `sh` in $PATH at all — distroless / scratch
		// images. Letting the user pick "bash" directly skips the wrapper
		// so a node:slim container that lacks /bin/sh but ships /bin/bash
		// still works.
		shell := strings.ToLower(strings.TrimSpace(c.Query("shell")))
		username := c.Query("username", config.ServerDefaults().Username)

		if serverID == "" {
			h.LogWarn("Missing serverId")
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mMissing server ID\x1b[0m\r\n"))
			c.Close()
			return
		}

		// Authenticate using centralized auth (validates team membership)
		claims, err := h.Authenticate(c)
		if err != nil {
			h.LogError(err, "Authentication failed")
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mAuthentication failed\x1b[0m\r\n"))
			c.Close()
			return
		}

		h.LogInfo("Terminal connection requested",
			"user_id", claims.UserID,
			"server_id", serverID,
			"site_id", siteID,
			"container", containerName,
			"username", username,
		)

		// Fetch server from database
		var server serverModels.Server
		if err := h.DB.Where("id = ? AND team_id = ?", serverID, claims.TeamID).First(&server).Error; err != nil {
			h.LogError(err, "Server not found", "server_id", serverID)
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mServer not found\x1b[0m\r\n"))
			c.Close()
			return
		}

		// Fetch site if siteId is provided
		var sitePath string
		if siteID != "" {
			var site siteModels.Site
			if err := h.DB.Where("id = ? AND server_id = ?", siteID, serverID).First(&site).Error; err != nil {
				h.LogError(err, "Site not found", "site_id", siteID)
				c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mSite not found\x1b[0m\r\n"))
				c.Close()
				return
			}
			sitePath = site.GetApplicationDirectory()
		}

		// Check if server is connected
		if !server.Connected {
			h.LogWarn("Server not connected", "server_id", serverID)
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[33mServer is not connected\x1b[0m\r\n"))
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
			h.LogError(nil, "Server has no public IP", "server_id", serverID)
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mServer has no public IP\x1b[0m\r\n"))
			c.Close()
			return
		}

		if conn.PrivateKey == "" {
			h.LogError(nil, "Server has no SSH key", "server_id", serverID)
			c.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mNo SSH key configured\x1b[0m\r\n"))
			c.Close()
			return
		}

		// Establish SSH connection using taskrunner SSHClient
		h.handleSSHConnection(c, conn, server.Name, sitePath, containerName, shell)
	})
}

func (h *TerminalHandler) handleSSHConnection(wsConn *websocket.Conn, conn *taskrunner.Connection, serverName, sitePath, containerName, shell string) {
	// Create SSH client using taskrunner
	h.LogInfo("Connecting to SSH", "host", conn.Host, "port", conn.Port, "username", conn.User)

	sshClient, err := conn.Dial()
	if err != nil {
		h.LogError(err, "Failed to connect to SSH", "host", conn.Host, "port", conn.Port)
		wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31mSSH connection failed: %s\x1b[0m\r\n", err.Error())))
		return
	}
	defer sshClient.Close()

	// Create SSH session
	session, err := sshClient.NewSession()
	if err != nil {
		h.LogError(err, "Failed to create SSH session")
		wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mFailed to create session\x1b[0m\r\n"))
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
		h.LogError(err, "Failed to request pty")
		wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mFailed to allocate terminal\x1b[0m\r\n"))
		return
	}

	// Get stdin/stdout pipes
	stdin, err := session.StdinPipe()
	if err != nil {
		h.LogError(err, "Failed to get stdin pipe")
		return
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		h.LogError(err, "Failed to get stdout pipe")
		return
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		h.LogError(err, "Failed to get stderr pipe")
		return
	}

	// When `container` is set, replace the host login shell with a
	// `docker exec` into that container. The PTY we already requested
	// lives on the host SSH session; `-it` re-attaches it to the
	// container's stdio, so resize events still flow through.
	//
	// We prefer bash if the image carries it, else fall back to sh.
	// Both Alpine (sh-only) and Debian-based images work.
	//
	// Safety: dockerContainerNamePattern below restricts the input to a
	// container-name-safe charset so we can't be tricked into
	// arbitrary command injection.
	if containerName != "" {
		if !isValidContainerName(containerName) {
			h.LogWarn("Rejected unsafe container name", "container", containerName)
			wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mInvalid container name\x1b[0m\r\n"))
			return
		}
		// Pick the shell. Three modes:
		//   - shell="bash" → invoke /bin/bash directly. Skips the `sh -c`
		//     wrapper so containers without /bin/sh (distroless variants
		//     that still ship bash) work.
		//   - shell="sh"   → invoke /bin/sh directly. Same reasoning for
		//     alpine-only / minimal images that don't have bash.
		//   - shell=""     → legacy auto-detect via the wrapper.
		var execCmd string
		switch shell {
		case "bash":
			execCmd = fmt.Sprintf("docker exec -it -e TERM=xterm-256color %s /bin/bash", containerName)
		case "sh":
			execCmd = fmt.Sprintf("docker exec -it -e TERM=xterm-256color %s /bin/sh", containerName)
		default:
			execCmd = fmt.Sprintf(
				"docker exec -it %s sh -c 'export TERM=xterm-256color; (command -v bash >/dev/null && exec bash) || exec sh'",
				containerName,
			)
		}
		if err := session.Start(execCmd); err != nil {
			h.LogError(err, "Failed to start docker exec", "container", containerName)
			wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31mFailed to attach to container: %s\x1b[0m\r\n", err.Error())))
			return
		}
		h.LogInfo("Docker exec session started",
			"server", serverName, "username", conn.User, "container", containerName,
		)
		wsConn.WriteMessage(websocket.TextMessage, []byte("\x1bc"))
	} else {
		// Standard host shell — the legacy path.
		if err := session.Shell(); err != nil {
			h.LogError(err, "Failed to start shell")
			wsConn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mFailed to start shell\x1b[0m\r\n"))
			return
		}

		h.LogInfo("SSH session started", "server", serverName, "username", conn.User, "sitePath", sitePath)

		// Send a clear screen to the terminal first
		wsConn.WriteMessage(websocket.TextMessage, []byte("\x1bc"))

		// If site path is provided, cd into it and clear the screen
		if sitePath != "" {
			initCmd := fmt.Sprintf("cd \"%s\" && export DEBIAN_FRONTEND=noninteractive && clear\n", sitePath)
			stdin.Write([]byte(initCmd))
		} else {
			stdin.Write([]byte("export DEBIAN_FRONTEND=noninteractive && clear\n"))
		}
	}

	// Set up WebSocket keepalive
	var writeMu sync.Mutex

	wsConn.SetPongHandler(func(string) error {
		wsConn.SetReadDeadline(time.Now().Add(terminalPongWait))
		return nil
	})
	wsConn.SetReadDeadline(time.Now().Add(terminalPongWait))

	// Bridge WebSocket and SSH
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Ping ticker to keep WebSocket alive
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(terminalPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				writeMu.Lock()
				err := wsConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(terminalWriteWait))
				writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	// SSH stdout -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.copyToWebSocket(wsConn, stdout, done, &writeMu)
	}()

	// SSH stderr -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.copyToWebSocket(wsConn, stderr, done, &writeMu)
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

	// Signal the browser that the shell has exited so the UI's status
	// indicator flips to "Disconnected" immediately. Without this
	// frame the WS sits open until the next ping fails or the read
	// deadline expires (~minutes), and the browser ends up rendering
	// "1006 abnormal closure" long after the actual exit.
	//
	// Tightening the read deadline to "now" also unblocks the
	// handleWebSocketInput goroutine's ReadMessage call so wg.Wait()
	// below returns promptly instead of stalling on the shell's exit.
	writeMu.Lock()
	_ = wsConn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shell exited"),
		time.Now().Add(terminalWriteWait),
	)
	writeMu.Unlock()
	_ = wsConn.SetReadDeadline(time.Now())

	wg.Wait()

	h.LogInfo("SSH session ended", "server", serverName)
}

func (h *TerminalHandler) copyToWebSocket(wsConn *websocket.Conn, reader io.Reader, done chan struct{}, writeMu *sync.Mutex) {
	buf := make([]byte, 8192)
	for {
		select {
		case <-done:
			return
		default:
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					h.LogDebug("SSH read error", "error", err.Error())
				}
				return
			}

			if n > 0 {
				writeMu.Lock()
				err := wsConn.WriteMessage(websocket.TextMessage, buf[:n])
				writeMu.Unlock()
				if err != nil {
					h.LogDebug("WebSocket write error", "error", err.Error())
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
					h.LogDebug("WebSocket closed normally")
				} else if !isConnectionClosed(err) {
					h.LogDebug("WebSocket read error", "error", err.Error())
				}
				return
			}

			if msgType == websocket.TextMessage {
				// Check if it's a resize message
				var resizeMsg TerminalResizeMessage
				if err := json.Unmarshal(msg, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
					if resizeMsg.Cols > 0 && resizeMsg.Rows > 0 {
						if err := session.WindowChange(resizeMsg.Rows, resizeMsg.Cols); err != nil {
							h.LogDebug("Failed to resize window", "error", err.Error())
						}
					}
					continue
				}

				// Regular input - send to SSH stdin
				if _, err := stdin.Write(msg); err != nil {
					h.LogDebug("SSH stdin write error", "error", err.Error())
					return
				}
			}
		}
	}
}

// isValidContainerName guards the `docker exec` callsite — the
// container name flows from a URL query param straight into a shell
// command, so we restrict it to docker's own naming rules
// (https://docs.docker.com/engine/reference/run/#name---name).
// Allowed: [a-zA-Z0-9][a-zA-Z0-9_.-]*, length 1..253. Rejects every
// metacharacter (`;`, `$`, backtick, space, etc.) which is what
// matters for command-injection safety.
func isValidContainerName(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	for i, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if i > 0 {
			ok = ok || r == '_' || r == '.' || r == '-'
		}
		if !ok {
			return false
		}
	}
	return true
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
