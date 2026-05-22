package handlers

import (
	"bufio"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/ssh"

	dockermodels "github.com/kkz6/launch-go/internal/modules/docker/models"
	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
)

// DockerLogsHandler streams `docker logs -f` from a running container
// over a WebSocket. Each line is sent as a TextMessage frame so the
// frontend can append it to a buffer without parsing JSON.
//
// Mirrors MetricsHandler in structure: auth → fetch app + server →
// open SSH → pipe stdout to the WS → wait for client to disconnect.
type DockerLogsHandler struct {
	Base
}

// NewDockerLogsHandler builds a handler from the shared websocket-handler
// Base (already carries DB, logger, JWT secret, membership cache).
func NewDockerLogsHandler(base Base) *DockerLogsHandler {
	return &DockerLogsHandler{Base: base.WithComponent("docker_logs")}
}

// Handler returns a Fiber WebSocket handler. URL parameters:
//
//	applicationId   required — the docker application to stream from.
//	tail            optional, default 200 — number of historical lines
//	                to backfill before live tailing starts.
func (h *DockerLogsHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		applicationID := c.Query("applicationId")
		tail := parseTail(c.Query("tail", "200"))

		if applicationID == "" {
			h.SendError(c, "Missing applicationId parameter")
			c.Close()
			return
		}

		claims, err := h.Authenticate(c)
		if err != nil {
			h.LogError(err, "Authentication failed")
			h.SendError(c, "Authentication failed")
			c.Close()
			return
		}

		// Resolve the application within the caller's team. Scoping by
		// team_id here is what prevents a token holder from streaming
		// another team's container logs by guessing IDs.
		var app dockermodels.Application
		if err := h.DB.Where("id = ? AND team_id = ?", applicationID, claims.TeamID).First(&app).Error; err != nil {
			h.LogError(err, "Application not found", "application_id", applicationID)
			h.SendError(c, "Application not found")
			c.Close()
			return
		}
		if app.ContainerID == nil || *app.ContainerID == "" {
			// No container_id yet means the app hasn't deployed
			// successfully. Tell the user instead of trying to docker-
			// logs an empty string (which docker would treat as "all").
			_ = SendEvent(c, "no_container", map[string]any{
				"message": "This application has not been deployed yet.",
			})
			c.Close()
			return
		}

		var server serverModels.Server
		if err := h.DB.Where("id = ? AND team_id = ?", app.ServerID, claims.TeamID).First(&server).Error; err != nil {
			h.LogError(err, "Server not found", "server_id", app.ServerID)
			h.SendError(c, "Server not found")
			c.Close()
			return
		}

		h.LogInfo("Docker logs stream requested",
			"application_id", app.ID,
			"container_id", *app.ContainerID,
			"server_id", server.ID,
			"tail", tail,
		)

		h.streamLogs(c, &server, *app.ContainerID, tail)
	})
}

// streamLogs SSHs to the docker host and pipes `docker logs -f` into
// the WebSocket. Closes cleanly when the client disconnects.
func (h *DockerLogsHandler) streamLogs(
	c *websocket.Conn, server *serverModels.Server, containerID string, tail int,
) {
	sshConfig := server.ConnectionAsRoot()
	if sshConfig.Host == "" {
		h.SendError(c, "Server has no public IP")
		return
	}
	if sshConfig.PrivateKey == "" {
		h.SendError(c, "No SSH key configured")
		return
	}

	signer, err := ssh.ParsePrivateKey([]byte(sshConfig.PrivateKey))
	if err != nil {
		h.LogError(err, "Failed to parse private key")
		h.SendError(c, "Invalid SSH key")
		return
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", sshConfig.Host, sshConfig.Port), &ssh.ClientConfig{
		User: sshConfig.User,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		// InsecureIgnoreHostKey matches the rest of the SSH connection
		// sites in this codebase (metrics, terminal). Re-evaluating this
		// is a cross-cutting hardening task, not a per-handler decision.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		h.LogError(err, "Failed to connect to SSH")
		h.SendError(c, fmt.Sprintf("SSH connection failed: %s", err.Error()))
		return
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		h.LogError(err, "Failed to create SSH session")
		h.SendError(c, "Failed to create session")
		return
	}
	defer session.Close()

	// `docker logs -f --tail <n>` writes both stdout AND stderr from the
	// container to the SSH session's stdout (docker collapses them by
	// default unless you call --details). We pull both pipes to be safe
	// for older docker versions that route stderr separately.
	command := fmt.Sprintf(
		`docker logs --follow --tail %d --timestamps %s 2>&1`,
		tail, shellQuote(containerID),
	)
	stdout, err := session.StdoutPipe()
	if err != nil {
		h.LogError(err, "Failed to get stdout pipe")
		h.SendError(c, "Failed to setup output stream")
		return
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		h.LogError(err, "Failed to get stderr pipe")
		h.SendError(c, "Failed to setup error stream")
		return
	}

	if err := session.Start(command); err != nil {
		h.LogError(err, "Failed to start docker logs command")
		h.SendError(c, "Failed to start log stream")
		return
	}

	_ = SendEvent(c, "connected", map[string]any{"container_id": containerID})

	done := make(chan struct{})

	// Stdout: every line goes to the WS. We use a generous buffer because
	// containers occasionally emit very long stack traces in one line —
	// the 64 KiB cap matches the metrics handler.
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 256*1024)
		for scanner.Scan() {
			select {
			case <-done:
				return
			default:
			}
			if err := c.WriteMessage(websocket.TextMessage, scanner.Bytes()); err != nil {
				return
			}
		}
	}()

	// Stderr from docker itself (not from the container) is logged but
	// not forwarded to the user — typically it's "Error response from
	// daemon: No such container" which we've already guarded against.
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
					h.LogWarn("docker logs stderr", "stderr", string(buf[:n]))
				}
			}
		}
	}()

	// Block until the client disconnects.
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}

	close(done)
	_ = session.Signal(ssh.SIGTERM)
	_ = session.Close()
	h.LogInfo("Docker logs stream ended", "container_id", containerID)
}

// parseTail clamps the tail value to a sane range. A user-provided
// `tail=100000` would blast a multi-MB payload to the browser on
// connect; cap it so the page stays responsive.
func parseTail(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 200
	}
	if n > 5000 {
		return 5000
	}
	return n
}

// shellQuote single-quotes a value for safe shell interpolation. We
// only ever pass container IDs (a-f0-9), but defence-in-depth: never
// concat untrusted input into a shell command without escaping. A
// rogue container ID in the DB shouldn't lead to RCE on the docker host.
func shellQuote(s string) string {
	// Standard sh-quoting: wrap in '…', escape internal ' as '\''.
	out := make([]byte, 0, len(s)+2)
	out = append(out, '\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\\', '\'', '\'')
			continue
		}
		out = append(out, s[i])
	}
	out = append(out, '\'')
	return string(out)
}
