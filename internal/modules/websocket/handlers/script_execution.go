package handlers

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	scriptModels "github.com/kkz6/launch-go/internal/modules/script/models"
	"github.com/kkz6/launch-go/internal/modules/script/support"
	scripttypes "github.com/kkz6/launch-go/internal/modules/script/types"
	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
)

// ScriptExecutionHandler handles WebSocket connections for script execution streaming
type ScriptExecutionHandler struct {
	Base
}

const maxStoredScriptOutput = 10 << 20

// NewScriptExecutionHandler creates a new script execution handler
func NewScriptExecutionHandler(base Base) *ScriptExecutionHandler {
	return &ScriptExecutionHandler{
		Base: base.WithComponent("script-execution"),
	}
}

// NewScriptExecutionHandlerWithDeps creates a new script execution handler with individual dependencies (legacy).
// Deprecated: Use NewScriptExecutionHandler with Base instead.
func NewScriptExecutionHandlerWithDeps(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *launchcache.TeamMembershipCache) *ScriptExecutionHandler {
	return NewScriptExecutionHandler(NewBase(db, jwtSecret, logger, membershipCache))
}

// Handler returns a Fiber handler for script execution WebSocket connections
func (h *ScriptExecutionHandler) Handler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		executionIDStr := c.Query("executionId")
		if executionIDStr == "" {
			h.sendError(c, "Missing executionId parameter")
			return
		}

		executionID, err := strconv.ParseUint(executionIDStr, 10, 64)
		if err != nil {
			h.sendError(c, "Invalid executionId")
			return
		}

		// Authenticate
		claims, err := h.Authenticate(c)
		if err != nil {
			h.sendError(c, "Authentication failed")
			return
		}

		// Fetch execution record
		var execution scriptModels.ScriptExecution
		if err := h.DB.Where("id = ?", executionID).First(&execution).Error; err != nil {
			h.sendError(c, "Execution not found")
			return
		}

		// Fetch script
		var script scriptModels.Script
		if err := h.DB.Where("id = ?", execution.ScriptID).First(&script).Error; err != nil {
			h.sendError(c, "Script not found")
			return
		}

		// Verify access - script must be owned by user or shared with team
		if script.UserID != claims.UserID && (script.TeamID == nil || *script.TeamID != claims.TeamID) {
			h.sendError(c, "Access denied")
			return
		}

		// Fetch server
		var server serverModels.Server
		if err := h.DB.Where("id = ? AND team_id = ?", execution.ServerID, claims.TeamID).First(&server).Error; err != nil {
			h.sendError(c, "Server not found")
			return
		}

		// If execution is already complete, send stored output and close
		if execution.IsComplete() {
			if execution.Output != nil {
				c.WriteMessage(websocket.TextMessage, []byte(*execution.Output))
			}
			h.sendJSON(c, map[string]any{
				"type":      "completed",
				"status":    string(execution.Status),
				"exit_code": execution.ExitCode,
			})
			c.Close()
			return
		}

		// If execution is still pending, update to running and execute
		switch execution.Status {
		case scriptModels.ExecutionStatusPending:
			h.executeScript(c, &execution, &script, &server)
		case scriptModels.ExecutionStatusRunning:
			// Already running elsewhere - just notify and close
			h.sendJSON(c, map[string]any{
				"type":    "error",
				"message": h.Translate(c, "Execution is already running"),
			})
			c.Close()
		}
	})
}

func (h *ScriptExecutionHandler) executeScript(c *websocket.Conn, execution *scriptModels.ScriptExecution, script *scriptModels.Script, server *serverModels.Server) {
	// Update status to running
	now := time.Now()
	if err := h.DB.Model(execution).Updates(map[string]any{
		"status":     scriptModels.ExecutionStatusRunning,
		"started_at": now,
	}).Error; err != nil {
		h.LogError(err, "Failed to persist script execution start", "execution_id", execution.ID)
		h.sendError(c, "Failed to start execution")
		return
	}

	h.sendJSON(c, map[string]any{
		"type":   "started",
		"status": "running",
	})

	// Get SSH config
	sshConfig := server.ConnectionAsRoot()
	if sshConfig.Host == "" {
		h.finishWithError(c, execution, "Server has no public IP")
		return
	}

	if sshConfig.PrivateKey == "" {
		h.finishWithError(c, execution, "No SSH key configured")
		return
	}

	// Resolve the run-as type to actual username from server
	runAsUser := resolveRunAsUser(execution.RunAs, script.RunAs, server)

	// Use the managed taskrunner connection so script execution enforces the
	// server's pinned/TOFU host key just like other remote tasks.
	sshConfig.User = runAsUser
	conn, err := sshConfig.Dial(10 * time.Second)
	if err != nil {
		h.finishWithError(c, execution, "SSH connection failed: %s", err.Error())
		return
	}
	defer conn.Close()

	// Create SSH session
	session, err := conn.NewSession()
	if err != nil {
		h.finishWithError(c, execution, "Failed to create SSH session")
		return
	}
	defer session.Close()

	// Interpolate variables in script content
	interpolatedContent := support.InterpolateVariables(script.Content, server)

	// Build command - wrap script content in bash
	command := fmt.Sprintf("bash -c %q", interpolatedContent)

	h.LogInfo("Executing script",
		"execution_id", execution.ID,
		"server_id", server.ID,
		"user", runAsUser,
	)

	// Get stdout and stderr pipes
	stdout, err := session.StdoutPipe()
	if err != nil {
		h.finishWithError(c, execution, "Failed to get stdout pipe")
		return
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		h.finishWithError(c, execution, "Failed to get stderr pipe")
		return
	}

	// Start command
	if err := session.Start(command); err != nil {
		h.finishWithError(c, execution, "Failed to start script: %s", err.Error())
		return
	}

	// Collect output for storage. Gorilla WebSocket permits one concurrent
	// writer, so both SSH streams are serialized before they reach the socket.
	var (
		outputBuffer []byte
		outputMu     sync.Mutex
		writeMu      sync.Mutex
		readers      sync.WaitGroup
	)
	appendOutput := func(chunk []byte) {
		outputMu.Lock()
		defer outputMu.Unlock()
		remaining := maxStoredScriptOutput - len(outputBuffer)
		if remaining > 0 {
			if len(chunk) > remaining {
				chunk = chunk[:remaining]
			}
			outputBuffer = append(outputBuffer, chunk...)
		}
	}
	stream := func(reader io.Reader) {
		defer readers.Done()
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				appendOutput(chunk)
				writeMu.Lock()
				writeErr := c.WriteMessage(websocket.TextMessage, chunk)
				writeMu.Unlock()
				if writeErr != nil {
					h.LogError(writeErr, "Failed to stream script output", "execution_id", execution.ID)
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					h.LogError(err, "Failed to read script output", "execution_id", execution.ID)
				}
				return
			}
		}
	}

	readers.Add(2)
	go stream(stdout)
	go stream(stderr)

	// Wait for command to complete
	err = session.Wait()
	readers.Wait()
	outputMu.Lock()
	output := string(outputBuffer)
	outputMu.Unlock()

	// Get exit code
	exitCode := 0
	if err != nil {
		if _, ok := err.(*ssh.ExitError); !ok {
			h.LogError(err, "Script process wait failed", "execution_id", execution.ID)
		}
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			exitCode = 1
		}
	}

	// Determine final status
	finalStatus := scriptModels.ExecutionStatusFinished
	if exitCode != 0 {
		finalStatus = scriptModels.ExecutionStatusFailed
	}

	// Update execution record
	finishedAt := time.Now()
	if err := h.DB.Model(execution).Updates(map[string]any{
		"status":      finalStatus,
		"exit_code":   exitCode,
		"output":      output,
		"finished_at": finishedAt,
	}).Error; err != nil {
		h.LogError(err, "Failed to persist script execution result", "execution_id", execution.ID)
		h.sendError(c, "Failed to persist execution result")
		c.Close()
		return
	}

	// Send completion message
	h.sendJSON(c, map[string]any{
		"type":      "completed",
		"status":    string(finalStatus),
		"exit_code": exitCode,
	})

	h.LogInfo("Script execution completed",
		"execution_id", execution.ID,
		"status", string(finalStatus),
		"exit_code", exitCode,
	)

	c.Close()
}

func (h *ScriptExecutionHandler) finishWithError(c *websocket.Conn, execution *scriptModels.ScriptExecution, message string, args ...any) {
	errMsg := fmt.Sprintf(message, args...)
	h.LogError(nil, "Script execution failed",
		"execution_id", execution.ID,
		"error", errMsg,
	)

	// Update execution record
	finishedAt := time.Now()
	if err := h.DB.Model(execution).Updates(map[string]any{
		"status":      scriptModels.ExecutionStatusFailed,
		"output":      errMsg,
		"finished_at": finishedAt,
	}).Error; err != nil {
		h.LogError(err, "Failed to persist script execution failure", "execution_id", execution.ID)
	}

	h.sendJSON(c, map[string]any{
		"type":    "error",
		"message": h.Translate(c, message, args...),
		"status":  "failed",
	})
	c.Close()
}

func (h *ScriptExecutionHandler) sendError(c *websocket.Conn, msg string) {
	h.LogWarn(msg)
	_ = h.SendJSON(c, map[string]any{
		"type":    "error",
		"message": h.Translate(c, msg),
	})
	c.Close()
}

func (h *ScriptExecutionHandler) sendJSON(c *websocket.Conn, data map[string]any) {
	if err := h.SendJSON(c, data); err != nil {
		h.LogError(err, "Failed to send script execution event")
	}
}

// resolveRunAsUser resolves the run-as type to the actual username from the server
// Priority: execution.RunAs > script.RunAs > default to root
func resolveRunAsUser(executionRunAs *scripttypes.RunAsUser, scriptRunAs scripttypes.RunAsUser, server *serverModels.Server) string {
	// Use execution's run-as if specified, otherwise fall back to script's default
	runAs := scriptRunAs
	if executionRunAs != nil {
		runAs = *executionRunAs
	}

	switch runAs {
	case scripttypes.RunAsUserLocal:
		return server.GetUsername()
	case scripttypes.RunAsUserRoot:
		return server.RootUsername()
	default:
		return server.RootUsername()
	}
}
