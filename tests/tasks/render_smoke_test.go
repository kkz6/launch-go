package tasks_test

import (
	"strings"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

// The software install templates have no snapshots, so nothing else proves
// they still render or that the apt helpers they call are in scope.
func TestSoftwareInstallScriptsRenderWithAptGetDefined(t *testing.T) {
	software := []types.Software{
		types.SoftwarePhp83,
		types.SoftwareRedis,
		types.SoftwareSupervisor,
		types.SoftwareMySQL80,
		types.SoftwarePostgreSQL16,
		types.SoftwareCaddy2,
		types.SoftwareNode21,
		types.SoftwareBun,
	}

	for _, sw := range software {
		t.Run(string(sw), func(t *testing.T) {
			script := tasks.InstallSoftware(sw, tasks.SoftwareInstallConfig{
				Username:         "launch",
				MemoryInMB:       2048,
				DatabasePassword: "secret",
				DatabaseName:     "launch",
				PublicIPv4:       "203.0.113.1",
			}).Script()

			if script == "" {
				t.Fatal("rendered an empty script")
			}
			if strings.Contains(script, "aptGet ") && !strings.Contains(script, "function aptGet()") {
				t.Error("calls aptGet but the rendered script never defines it")
			}
		})
	}
}
