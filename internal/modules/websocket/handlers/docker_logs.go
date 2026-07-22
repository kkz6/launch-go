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

// Handler returns a Fiber WebSocket handler. URL parameters (mutually
// exclusive — exactly one of the *id query params must be present):
//
//	applicationId   stream from the application's container.
//	databaseId      stream from the managed-database container.
//	composeId       stream from every service in the compose stack
//	                (docker compose logs -f --project-name <name>).
//	tail            optional, default 200 — number of historical lines
//	                to backfill before live tailing starts.
func (h *DockerLogsHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		appID := c.Query("applicationId")
		dbID := c.Query("databaseId")
		composeID := c.Query("composeId")
		// `service` only applies to compose streams. When non-empty
		// the script runs `docker compose logs <service>` so the
		// stream shows only that container's stdout. Empty (default)
		// = aggregate logs from every service in the stack. Letters,
		// digits, hyphen, underscore only — anything else is rejected
		// before reaching the shell.
		service := c.Query("service")
		tail := parseTail(c.Query("tail", "200"))

		set := 0
		for _, v := range []string{appID, dbID, composeID} {
			if v != "" {
				set++
			}
		}
		if set != 1 {
			h.SendError(c, "Pass exactly one of applicationId / databaseId / composeId")
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

		switch {
		case appID != "":
			h.streamForApplication(c, claims.TeamID, appID, tail)
		case dbID != "":
			h.streamForDatabase(c, claims.TeamID, dbID, tail)
		case composeID != "":
			h.streamForCompose(c, claims.TeamID, composeID, service, tail)
		}
	})
}

// isSafeServiceName checks `service` against the docker compose
// service-name rules: lowercase letters / digits / underscore / dash.
// Defence-in-depth — the value gets shell-quoted before interpolation
// regardless, but rejecting bad input here makes the failure mode
// obvious in the WS handshake instead of a malformed docker error
// down the wire.
func isSafeServiceName(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return true
}

// streamForApplication tails a single docker container belonging to a
// docker application. Same flow as before — kept as a separate method
// now that the dispatcher handles three sources.
func (h *DockerLogsHandler) streamForApplication(
	c *websocket.Conn, teamID, applicationID string, tail int,
) {
	var app dockermodels.Application
	if err := h.DB.Where("id = ? AND team_id = ?", applicationID, teamID).First(&app).Error; err != nil {
		h.LogError(err, "Application not found", "application_id", applicationID)
		h.SendError(c, "Application not found")
		c.Close()
		return
	}
	if app.ContainerID == nil || *app.ContainerID == "" {
		// No container_id yet means the app hasn't deployed
		// successfully. Tell the user instead of trying to docker-logs
		// an empty string (which docker would treat as "all").
		_ = SendEvent(c, "no_container", map[string]any{
			"message": "This application has not been deployed yet.",
		})
		c.Close()
		return
	}

	server, err := h.findServer(app.ServerID, teamID)
	if err != nil {
		h.SendError(c, "Server not found")
		c.Close()
		return
	}

	h.LogInfo("Docker logs stream requested (application)",
		"application_id", app.ID,
		"container_id", *app.ContainerID,
		"server_id", server.ID,
		"tail", tail,
	)

	cmd := fmt.Sprintf(
		`docker logs --follow --tail %d --timestamps %s 2>&1`,
		tail, shellQuote(*app.ContainerID),
	)
	h.streamCommand(c, server, cmd, *app.ContainerID)
}

// streamForDatabase tails a managed-database container's logs. The
// database row carries container_id the same way an application does,
// so the SSH-side command is identical to the application path.
func (h *DockerLogsHandler) streamForDatabase(
	c *websocket.Conn, teamID, databaseID string, tail int,
) {
	var db dockermodels.Database
	if err := h.DB.Where("id = ? AND team_id = ?", databaseID, teamID).First(&db).Error; err != nil {
		h.LogError(err, "Database not found", "database_id", databaseID)
		h.SendError(c, "Database not found")
		c.Close()
		return
	}
	// The database row doesn't yet store its container_id directly —
	// it's derivable from project+name via DatabaseContainerName. We
	// need the project to compose the name, so look it up first.
	var project dockermodels.Project
	if err := h.DB.Where("id = ? AND team_id = ?", db.ProjectID, teamID).First(&project).Error; err != nil {
		h.LogError(err, "Project not found", "project_id", db.ProjectID)
		h.SendError(c, "Project not found")
		c.Close()
		return
	}

	server, err := h.findServer(db.ServerID, teamID)
	if err != nil {
		h.SendError(c, "Server not found")
		c.Close()
		return
	}

	containerName := databaseContainerName(project.Name, db.Name)
	h.LogInfo("Docker logs stream requested (database)",
		"database_id", db.ID,
		"container_name", containerName,
		"server_id", server.ID,
		"tail", tail,
	)

	cmd := fmt.Sprintf(
		`docker logs --follow --tail %d --timestamps %s 2>&1`,
		tail, shellQuote(containerName),
	)
	h.streamCommand(c, server, cmd, containerName)
}

// streamForCompose tails the compose stack's logs via
// `docker compose logs --follow`. When `service` is empty the
// command aggregates every container; when set, only that service's
// logs stream. Each line is prefixed with the service name by docker
// compose so an "all services" stream is still distinguishable.
func (h *DockerLogsHandler) streamForCompose(
	c *websocket.Conn, teamID, composeID, service string, tail int,
) {
	if !isSafeServiceName(service) {
		h.SendError(c, "Invalid service name")
		c.Close()
		return
	}
	var stack dockermodels.Compose
	if err := h.DB.Where("id = ? AND team_id = ?", composeID, teamID).First(&stack).Error; err != nil {
		h.LogError(err, "Compose stack not found", "compose_id", composeID)
		h.SendError(c, "Compose stack not found")
		c.Close()
		return
	}
	var project dockermodels.Project
	if err := h.DB.Where("id = ? AND team_id = ?", stack.ProjectID, teamID).First(&project).Error; err != nil {
		h.SendError(c, "Project not found")
		c.Close()
		return
	}

	server, err := h.findServer(stack.ServerID, teamID)
	if err != nil {
		h.SendError(c, "Server not found")
		c.Close()
		return
	}

	projectName := composeProjectName(project.Name, stack.Name)
	h.LogInfo("Docker logs stream requested (compose)",
		"compose_id", stack.ID,
		"compose_project", projectName,
		"service", service,
		"server_id", server.ID,
		"tail", tail,
	)

	// Append service name when set — `docker compose logs <svc>`
	// scopes the stream to that one container. `--follow` keeps
	// streaming new lines until the WS closes.
	cmd := fmt.Sprintf(
		`docker compose --project-name %s logs --follow --tail %d --timestamps`,
		shellQuote(projectName), tail,
	)
	if service != "" {
		cmd += " " + shellQuote(service)
	}
	cmd += " 2>&1"
	subject := projectName
	if service != "" {
		subject = projectName + ":" + service
	}
	h.streamCommand(c, server, cmd, subject)
}

// findServer is a tiny DB lookup helper used by every streamForX.
func (h *DockerLogsHandler) findServer(serverID, teamID string) (*serverModels.Server, error) {
	var s serverModels.Server
	if err := h.DB.Where("id = ? AND team_id = ?", serverID, teamID).First(&s).Error; err != nil {
		h.LogError(err, "Server not found", "server_id", serverID)
		return nil, err
	}
	return &s, nil
}

// streamCommand SSHs to the docker host and pipes the given command's
// stdout into the WebSocket. Closes cleanly when the client disconnects.
//
// The `subject` parameter is purely for logging context — typically a
// container name or compose project name. It doesn't affect the SSH
// command itself.
func (h *DockerLogsHandler) streamCommand(
	c *websocket.Conn, server *serverModels.Server, command, subject string,
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

	conn, err := sshConfig.Dial(10 * time.Second)
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

	// `command` is the already-rendered shell command — `docker logs -f`
	// for application/database streams, `docker compose logs -f` for
	// compose. We pull both stdout and stderr to be safe for older
	// docker versions that route stderr separately.
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

	_ = SendEvent(c, "connected", map[string]any{"subject": subject})

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
	h.LogInfo("Docker logs stream ended", "subject", subject)
}

// databaseContainerName mirrors tasks.DatabaseContainerName. Duplicated
// here intentionally — importing the docker tasks package from the
// websocket handler would invert the dependency direction (the docker
// module already imports websocket types via the broadcast hub).
// Slugify behaviour matches tasks.SlugFromName: lowercase a-z0-9 plus
// dashes, anything else collapses to a single dash.
func databaseContainerName(projectName, dbName string) string {
	return fmt.Sprintf("launch-db-%s-%s", slugifyForContainer(projectName), slugifyForContainer(dbName))
}

// composeProjectName mirrors tasks.SlugFromName-based naming so it
// matches what jobs.DeployComposeJob actually used at `docker compose
// --project-name`.
func composeProjectName(projectName, composeName string) string {
	return fmt.Sprintf("%s-%s", slugifyForContainer(projectName), slugifyForContainer(composeName))
}

// slugifyForContainer keeps this handler self-contained. Mirror of
// tasks.SlugFromName — see the docker tasks package for the canonical
// implementation + tests.
func slugifyForContainer(name string) string {
	out := []byte{}
	prevDash := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32)
			prevDash = false
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			out = append(out, c)
			prevDash = false
		default:
			if !prevDash && len(out) > 0 {
				out = append(out, '-')
				prevDash = true
			}
		}
	}
	// Trim trailing dashes.
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return "app"
	}
	return string(out)
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
