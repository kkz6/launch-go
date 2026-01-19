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
	scriptModels "github.com/kkz6/launch-go/internal/modules/script/models"
	"github.com/kkz6/launch-go/internal/modules/script/support"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	ws "github.com/kkz6/launch-go/internal/websocket"
)

// ScriptExecutionHandler handles WebSocket connections for script execution streaming
type ScriptExecutionHandler struct {
	db              *gorm.DB
	jwtSecret       string
	logger          zerolog.Logger
	membershipCache *cache.TeamMembershipCache
}

// NewScriptExecutionHandler creates a new script execution handler
func NewScriptExecutionHandler(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *cache.TeamMembershipCache) *ScriptExecutionHandler {
	return &ScriptExecutionHandler{
		db:              db,
		jwtSecret:       jwtSecret,
		logger:          logger.With().Str("component", "script-execution").Logger(),
		membershipCache: membershipCache,
	}
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
		claims, err := ws.AuthenticateWebSocket(c, h.jwtSecret, h.membershipCache)
		if err != nil {
			h.sendError(c, "Authentication failed")
			return
		}

		// Fetch execution record
		var execution scriptModels.ScriptExecution
		if err := h.db.Where("id = ?", executionID).First(&execution).Error; err != nil {
			h.sendError(c, "Execution not found")
			return
		}

		// Fetch script
		var script scriptModels.Script
		if err := h.db.Where("id = ?", execution.ScriptID).First(&script).Error; err != nil {
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
		if err := h.db.Where("id = ? AND team_id = ?", execution.ServerID, claims.TeamID).First(&server).Error; err != nil {
			h.sendError(c, "Server not found")
			return
		}

		// If execution is already complete, send stored output and close
		if execution.IsComplete() {
			if execution.Output != nil {
				c.WriteMessage(websocket.TextMessage, []byte(*execution.Output))
			}
			h.sendJSON(c, map[string]interface{}{
				"type":      "completed",
				"status":    string(execution.Status),
				"exit_code": execution.ExitCode,
			})
			c.Close()
			return
		}

		// If execution is still pending, update to running and execute
		if execution.Status == scriptModels.ExecutionStatusPending {
			h.executeScript(c, &execution, &script, &server)
		} else if execution.Status == scriptModels.ExecutionStatusRunning {
			// Already running elsewhere - just notify and close
			h.sendJSON(c, map[string]interface{}{
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
	h.db.Model(execution).Updates(map[string]interface{}{
		"status":     scriptModels.ExecutionStatusRunning,
		"started_at": now,
	})

	h.sendJSON(c, map[string]interface{}{
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

	// Determine user to run as
	runAsUser := script.User
	if execution.User != nil && *execution.User != "" {
		runAsUser = *execution.User
	}

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

	h.logger.Info().
		Uint64("execution_id", execution.ID).
		Str("server_id", server.ID).
		Str("user", runAsUser).
		Msg("Executing script")

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
	h.db.Model(execution).Updates(map[string]interface{}{
		"status":      finalStatus,
		"exit_code":   exitCode,
		"output":      output,
		"finished_at": finishedAt,
	})

	// Send completion message
	h.sendJSON(c, map[string]interface{}{
		"type":      "completed",
		"status":    string(finalStatus),
		"exit_code": exitCode,
	})

	h.logger.Info().
		Uint64("execution_id", execution.ID).
		Str("status", string(finalStatus)).
		Int("exit_code", exitCode).
		Msg("Script execution completed")

	c.Close()
}

func (h *ScriptExecutionHandler) finishWithError(c *websocket.Conn, execution *scriptModels.ScriptExecution, errMsg string) {
	h.logger.Error().
		Uint64("execution_id", execution.ID).
		Str("error", errMsg).
		Msg("Script execution failed")

	// Update execution record
	finishedAt := time.Now()
	h.db.Model(execution).Updates(map[string]interface{}{
		"status":      scriptModels.ExecutionStatusFailed,
		"output":      errMsg,
		"finished_at": finishedAt,
	})

	h.sendJSON(c, map[string]interface{}{
		"type":    "error",
		"message": errMsg,
		"status":  "failed",
	})
	c.Close()
}

func (h *ScriptExecutionHandler) sendError(c *websocket.Conn, msg string) {
	h.logger.Warn().Msg(msg)
	h.sendJSON(c, map[string]interface{}{
		"type":    "error",
		"message": msg,
	})
	c.Close()
}

func (h *ScriptExecutionHandler) sendJSON(c *websocket.Conn, data map[string]interface{}) {
	// Simple JSON encoding
	msg := "{"
	first := true
	for k, v := range data {
		if !first {
			msg += ","
		}
		first = false
		msg += fmt.Sprintf(`"%s":`, k)
		switch val := v.(type) {
		case string:
			msg += fmt.Sprintf(`"%s"`, val)
		case int:
			msg += fmt.Sprintf("%d", val)
		case *int:
			if val != nil {
				msg += fmt.Sprintf("%d", *val)
			} else {
				msg += "null"
			}
		default:
			msg += fmt.Sprintf(`"%v"`, val)
		}
	}
	msg += "}"
	c.WriteMessage(websocket.TextMessage, []byte(msg))
}
