package status

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckDaemonStatusSupervisorExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		exitCode   int
		statusLine string
		wantErr    bool
		wantOutput string
	}{
		{
			name:       "all processes running",
			statusLine: "queue-1:queue-1_00 RUNNING pid 123, uptime 0:01:02",
			wantOutput: `"status":"RUNNING"`,
		},
		{
			name:       "unhealthy process is valid status data",
			exitCode:   3,
			statusLine: "queue-1:queue-1_00 FATAL Exited too quickly",
			wantOutput: `"status":"FATAL"`,
		},
		{
			name:       "supervisor command failure remains a task failure",
			exitCode:   1,
			statusLine: "unix:///run/supervisor.sock refused connection",
			wantErr:    true,
			wantOutput: "refused connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			binDir := t.TempDir()
			supervisorctl := filepath.Join(binDir, "supervisorctl")
			require.NoError(t, os.WriteFile(supervisorctl, []byte("#!/bin/sh\nprintf '%s\\n' \"$SUPERVISOR_OUTPUT\"\nexit \"$SUPERVISOR_EXIT\"\n"), 0o755))

			cmd := exec.Command("bash", "-c", CheckDaemonStatus().Script())
			cmd.Env = append(os.Environ(),
				"PATH="+binDir+":"+os.Getenv("PATH"),
				"SUPERVISOR_EXIT="+strconv.Itoa(tt.exitCode),
				"SUPERVISOR_OUTPUT="+tt.statusLine,
			)
			output, err := cmd.CombinedOutput()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Contains(t, string(output), "===STATUS_CHECK_COMPLETE===")
			}
			require.Contains(t, strings.TrimSpace(string(output)), tt.wantOutput)
		})
	}
}
