package jobs

import (
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypeExecuteScript is the task type for executing a script
const TypeExecuteScript = "script:execute"

// ExecuteScriptPayload represents the payload for executing a script
type ExecuteScriptPayload struct {
	ExecutionID uint64 `json:"execution_id"`
	ScriptID    string `json:"script_id"`
	ServerID    string `json:"server_id"`
	TeamID      string `json:"team_id"`
}

// NewExecuteScriptTask creates an asynq task for executing a script
func NewExecuteScriptTask(executionID uint64, scriptID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeExecuteScript, ExecuteScriptPayload{
		ExecutionID: executionID,
		ScriptID:    scriptID,
		ServerID:    serverID,
		TeamID:      teamID,
	}, asynq.TaskID(fmt.Sprintf("script-exec:%d", executionID)))
}
