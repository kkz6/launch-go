package handlers

import (
	"net/url"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	scriptmodels "github.com/kkz6/launch-go/internal/modules/script/models"
)

func TestScriptExecutionDeniesWritesButAllowsStoredOutput(t *testing.T) {
	for _, actor := range []struct {
		name   string
		role   string
		claims jwt.MapClaims
	}{
		{name: "member", role: "member"},
		{name: "read-only owner", role: "owner", claims: jwt.MapClaims{
			"read_only": true, "impersonation_sid": "spectate-session", "impersonator_id": "staff-user",
		}},
	} {
		for _, status := range []scriptmodels.ExecutionStatus{scriptmodels.ExecutionStatusPending, scriptmodels.ExecutionStatusFinished} {
			t.Run(actor.name+"/"+string(status), func(t *testing.T) {
				endpoint, db := socketAuthFixture(t, actor.role, true)
				require.NoError(t, db.Exec("CREATE TABLE scripts (id TEXT PRIMARY KEY, user_id TEXT, team_id TEXT, name TEXT, user TEXT, content TEXT)").Error)
				require.NoError(t, db.Exec("CREATE TABLE script_executions (id INTEGER PRIMARY KEY, script_id TEXT, server_id TEXT, status TEXT, exit_code INTEGER, output TEXT, started_at DATETIME, finished_at DATETIME, updated_at DATETIME)").Error)
				require.NoError(t, db.Exec("INSERT INTO scripts VALUES ('script-1', 'author-user', 'team-1', 'Shared script', 'root', 'echo hello')").Error)
				require.NoError(t, db.Exec("INSERT INTO script_executions (id, script_id, server_id, status) VALUES (1, 'script-1', 'server-1', ?)", status).Error)
				const output = "Previously stored output\n"
				if status == scriptmodels.ExecutionStatusFinished {
					require.NoError(t, db.Exec("UPDATE script_executions SET output = ?, exit_code = 0 WHERE id = 1", output).Error)
				}
				connection := dialAuthenticatedSocket(t, endpoint+"/scripts/execute", url.Values{"executionId": {"1"}}, actor.claims)
				_, message, err := connection.ReadMessage()
				require.NoError(t, err)
				if status == scriptmodels.ExecutionStatusPending {
					require.JSONEq(t, `{"type":"error","message":"Permission denied"}`, string(message))
				} else {
					require.Equal(t, output, string(message))
					_, completed, err := connection.ReadMessage()
					require.NoError(t, err)
					require.JSONEq(t, `{"type":"completed","status":"finished","exit_code":0}`, string(completed))
				}
				var stored scriptmodels.ScriptExecution
				require.NoError(t, db.First(&stored, 1).Error)
				require.Equal(t, status, stored.Status)
				require.Nil(t, stored.StartedAt, "reading a socket must not start an unauthorized execution")
				require.Nil(t, stored.FinishedAt)
				if status == scriptmodels.ExecutionStatusPending {
					require.Nil(t, stored.Output)
					require.Nil(t, stored.ExitCode)
				} else {
					require.NotNil(t, stored.Output)
					require.Equal(t, output, *stored.Output)
				}
			})
		}
	}
}
