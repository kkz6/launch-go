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
				"message": "Execution is already running",
			})
			c.Close()
		}
	})
}

func (h *ScriptExecutionHandler) executeScript(c *websocket.Conn, execution *scriptModels.ScriptExecution, script *scriptModels.Script, server *serverModels.Server) {
	// Update status to running
	now := time.Now()
	h.DB.Model(execution).Updates(map[string]any{
		"status":     scriptModels.ExecutionStatusRunning,
		"started_at": now,
	})

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

	// Parse private key
	signer, err := ssh.ParsePrivateKey([]byte(sshConfig.PrivateKey))
	if err != nil {
		h.finishWithError(c, execution, "Invalid SSH key")
		return
	}

	// Resolve the run-as type to actual username from server
	runAsUser := resolveRunAsUser(execution.RunAs, script.RunAs, server)

	// SSH client config
	config := &ssh.ClientConfig{
		User: runAsUser,
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
		h.finishWithError(c, execution, fmt.Sprintf("SSH connection failed: %s", err.Error()))
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
		h.finishWithError(c, execution, fmt.Sprintf("Failed to start script: %s", err.Error()))
		return
	}

	// Collect output for storage
	var outputBuffer []byte
	done := make(chan struct{})

	// Stream stdout
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
					outputBuffer = append(outputBuffer, buf[:n]...)
					c.WriteMessage(websocket.TextMessage, buf[:n])
				}
			}
		}
	}()

	// Stream stderr
	go func() {
		buf := make([]byte, 4096)
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
					outputBuffer = append(outputBuffer, buf[:n]...)
					c.WriteMessage(websocket.TextMessage, buf[:n])
				}
			}
		}
	}()

	// Wait for command to complete
	err = session.Wait()
	close(done)

	// Get exit code
	exitCode := 0
	if err != nil {
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
	output := string(outputBuffer)
	h.DB.Model(execution).Updates(map[string]any{
		"status":      finalStatus,
		"exit_code":   exitCode,
		"output":      output,
		"finished_at": finishedAt,
	})

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

func (h *ScriptExecutionHandler) finishWithError(c *websocket.Conn, execution *scriptModels.ScriptExecution, errMsg string) {
	h.LogError(nil, "Script execution failed",
		"execution_id", execution.ID,
		"error", errMsg,
	)

	// Update execution record
	finishedAt := time.Now()
	h.DB.Model(execution).Updates(map[string]any{
		"status":      scriptModels.ExecutionStatusFailed,
		"output":      errMsg,
		"finished_at": finishedAt,
	})

	h.sendJSON(c, map[string]any{
		"type":    "error",
		"message": errMsg,
		"status":  "failed",
	})
	c.Close()
}

func (h *ScriptExecutionHandler) sendError(c *websocket.Conn, msg string) {
	h.LogWarn(msg)
	_ = h.SendJSON(c, map[string]any{
		"type":    "error",
		"message": msg,
	})
	c.Close()
}

func (h *ScriptExecutionHandler) sendJSON(c *websocket.Conn, data map[string]any) {
	_ = h.SendJSON(c, data)
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
