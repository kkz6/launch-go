package tasks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunScriptUsesActionName(t *testing.T) {
	t.Parallel()

	task := RunScript(RunScriptConfig{
		Name:    "Run Database Maintenance",
		Content: "echo done",
	})

	require.Equal(t, "Run Database Maintenance", task.Name())
}

func TestRunScriptUsesDefaultName(t *testing.T) {
	t.Parallel()

	task := RunScript(RunScriptConfig{Content: "echo done"})

	require.Equal(t, "Run Script", task.Name())
}
