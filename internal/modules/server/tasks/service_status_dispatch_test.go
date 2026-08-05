package tasks

import (
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every status probe prints the same ===SECTION=== markers so the parser
// downstream can split one blob of output into structured fields.
func TestServiceStatusProbes(t *testing.T) {
	tests := []struct {
		name     string
		task     *taskrunner.BaseTask
		wantName string
		wantTTL  time.Duration
		contains []string
	}{
		{
			name:     "mysql",
			task:     CheckMySQLStatus(),
			wantName: "Check MySQL Status",
			wantTTL:  30 * time.Second,
			contains: []string{"systemctl status mysql", ":3306", "===PROCESSES===", "===MEMORY==="},
		},
		{
			name:     "postgresql",
			task:     CheckPostgreSQLStatus(),
			wantName: "Check PostgreSQL Status",
			wantTTL:  30 * time.Second,
			contains: []string{"postgresql", "===PROCESSES==="},
		},
		{
			name:     "redis",
			task:     CheckRedisStatus(),
			wantName: "Check Redis Status",
			wantTTL:  30 * time.Second,
			contains: []string{"redis", "===PROCESSES==="},
		},
		{
			name:     "caddy",
			task:     CheckCaddyStatus(),
			wantName: "Check Caddy Status",
			wantTTL:  30 * time.Second,
			contains: []string{"caddy", "===PROCESSES==="},
		},
		{
			name:     "supervisor",
			task:     CheckSupervisorStatus(),
			wantName: "Check Supervisor Status",
			wantTTL:  30 * time.Second,
			contains: []string{"supervisor"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.task)

			assert.Equal(t, tc.wantName, tc.task.Name())
			assert.Equal(t, tc.wantTTL, tc.task.Timeout())

			for _, want := range tc.contains {
				assert.Contains(t, tc.task.Script(), want)
			}
		})
	}
}

// Bun and Node aren't services, so their "status" is a version probe rather
// than a systemctl query.
func TestRuntimeVersionProbes(t *testing.T) {
	bun := CheckBunStatus()
	require.NotNil(t, bun)
	assert.Equal(t, "Check Bun Status", bun.Name())
	assert.Contains(t, bun.Script(), "bun")

	node := CheckNodeStatus()
	require.NotNil(t, node)
	assert.Equal(t, "Check Node Status", node.Name())
	assert.Contains(t, node.Script(), "node")
}

func TestCheckPhpStatus(t *testing.T) {
	task := CheckPhpStatus("8.3")
	require.NotNil(t, task)

	assert.Equal(t, "Check PHP 8.3 Status", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "systemctl status php8.3-fpm")
	assert.Contains(t, script, "/etc/php/8.3/fpm/pool.d/")
	assert.Contains(t, script, "===FPM_STATUS===")
	// The ps pattern brackets its first character so the grep doesn't match
	// itself in the process list.
	assert.Contains(t, script, "[p]hp8.3-fpm")
}

func TestGetServiceStatusTaskDispatch(t *testing.T) {
	tests := []struct {
		software string
		version  string
		wantName string
	}{
		{software: "mysql80", wantName: "Check MySQL Status"},
		{software: "mysql", wantName: "Check MySQL Status"},
		{software: "postgresql16", wantName: "Check PostgreSQL Status"},
		{software: "postgresql", wantName: "Check PostgreSQL Status"},
		{software: "redis", wantName: "Check Redis Status"},
		{software: "caddy2", wantName: "Check Caddy Status"},
		{software: "caddy", wantName: "Check Caddy Status"},
		{software: "supervisor", wantName: "Check Supervisor Status"},
		{software: "bun", wantName: "Check Bun Status"},
		{software: "node", wantName: "Check Node Status"},
		{software: "node18", wantName: "Check Node Status"},
		{software: "node20", wantName: "Check Node Status"},
		{software: "node22", wantName: "Check Node Status"},
		{software: "php83", wantName: "Check PHP 8.3 Status"},
		// Anything unrecognised falls back to a plain systemctl is-active
		// rather than failing, so a newly added service still reports.
		{software: "something-else", wantName: "Check something-else Status"},
	}

	for _, tc := range tests {
		t.Run(tc.software, func(t *testing.T) {
			task := GetServiceStatusTask(tc.software, tc.version)
			require.NotNil(t, task)
			assert.Equal(t, tc.wantName, task.Name())
		})
	}
}
