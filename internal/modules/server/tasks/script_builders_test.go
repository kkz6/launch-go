package tasks

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// whoami.go
// =============================================================================

func TestWhoami(t *testing.T) {
	task := Whoami()
	require.NotNil(t, task)

	assert.Equal(t, "Whoami", task.Name())
	assert.Equal(t, "whoami", task.Script())
	assert.Equal(t, 15*time.Second, task.Timeout())
}

// =============================================================================
// register.go
// =============================================================================

func TestRegisterTaskCallbacks(t *testing.T) {
	RegisterTaskCallbacks()

	for _, typeName := range []string{ProvisionFreshServerTaskType, ProvisionDockerServerTaskType} {
		handler, err := taskrunner.Reconstruct(typeName, []byte(`{}`))
		require.NoError(t, err, "type %s should be registered", typeName)
		assert.NotNil(t, handler)
	}
}

// =============================================================================
// cron.go
// =============================================================================

func TestUploadCron(t *testing.T) {
	task := UploadCron(UploadCronConfig{
		Path:     "/home/launch/.launch/cron-abc",
		Contents: "* * * * * launch php artisan schedule:run",
		LogPath:  "/home/launch/.launch/cron-abc.log",
		User:     "launch",
	})
	require.NotNil(t, task)

	assert.Equal(t, "Upload Cron File", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "/home/launch/.launch/cron-abc")
	assert.Contains(t, script, "schedule:run")
	assert.Contains(t, script, "chown launch:launch")
	assert.Contains(t, script, "CRONEOF")
}

func TestDeleteCron(t *testing.T) {
	task := DeleteCron(DeleteCronConfig{Path: "/home/launch/.launch/cron-abc"})
	require.NotNil(t, task)

	assert.Equal(t, "Delete Cron File", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "rm -f \"/home/launch/.launch/cron-abc\"")
	assert.Contains(t, script, "does not exist, skipping")
}

// =============================================================================
// backup.go
// =============================================================================

func TestRunBackup(t *testing.T) {
	task := RunBackup(RunBackupConfig{BackupID: "bk-123"})
	require.NotNil(t, task)

	assert.Equal(t, "Run Backup", task.Name())
	assert.Equal(t, "launch-agent perform -m bk-123", task.Script())
	assert.Equal(t, time.Hour, task.Timeout())
}

func TestDeleteBackup(t *testing.T) {
	task := DeleteBackup(DeleteBackupConfig{BackupID: "bk-123"})
	require.NotNil(t, task)

	assert.Equal(t, "Delete Backup", task.Name())
	assert.Equal(t, "launch-agent delete-backup -m bk-123", task.Script())
	assert.Equal(t, 5*time.Minute, task.Timeout())
}

func TestSyncLaunchConfig(t *testing.T) {
	task := SyncLaunchConfig(SyncLaunchConfigConfig{
		ConfigPath: "/etc/launch-agent/config.json",
		ConfigJSON: `{"backups":[]}`,
	})
	require.NotNil(t, task)

	assert.Equal(t, "Sync Launch Config", task.Name())
	assert.Equal(t, 2*time.Minute, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "/etc/launch-agent/config.json")
	assert.Contains(t, script, `{"backups":[]}`)
	assert.Contains(t, script, "systemctl restart launch-agent")
}

// =============================================================================
// firewall.go
// =============================================================================

func TestFirewallRuleFormatting(t *testing.T) {
	tests := []struct {
		name     string
		action   FirewallAction
		port     string
		protocol string
		fromIP   string
		remove   bool
		expected string
	}{
		{
			name:     "allow tcp port",
			action:   FirewallAllow,
			port:     "8080",
			protocol: "tcp",
			expected: "sudo ufw allow proto tcp to any port 8080",
		},
		{
			name:     "allow without protocol",
			action:   FirewallAllow,
			port:     "443",
			expected: "sudo ufw allow to any port 443",
		},
		{
			// Deny and reject are inserted at position 1 so they take
			// precedence over any broader allow rule already present.
			name:     "deny inserts at position 1",
			action:   FirewallDeny,
			port:     "22",
			protocol: "tcp",
			expected: "sudo ufw insert 1 deny proto tcp to any port 22",
		},
		{
			name:     "reject inserts at position 1",
			action:   FirewallReject,
			port:     "3306",
			expected: "sudo ufw insert 1 reject to any port 3306",
		},
		{
			name:     "allow from a specific address",
			action:   FirewallAllow,
			port:     "5432",
			protocol: "tcp",
			fromIP:   "10.0.0.5",
			expected: "sudo ufw allow proto tcp from 10.0.0.5 to any port 5432",
		},
		{
			// delete wins over the insert branch: you remove a rule by
			// naming it, not by naming its position.
			name:     "delete a deny rule",
			action:   FirewallDeny,
			port:     "22",
			protocol: "tcp",
			remove:   true,
			expected: "sudo ufw delete deny proto tcp to any port 22",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var task *taskrunner.BaseTask
			if tc.remove {
				task = DeleteFirewallRule(tc.action, tc.port, tc.protocol, tc.fromIP)
				assert.Equal(t, "Delete Firewall Rule", task.Name())
			} else {
				task = AddFirewallRule(tc.action, tc.port, tc.protocol, tc.fromIP)
				assert.Equal(t, "Add Firewall Rule", task.Name())
			}

			require.NotNil(t, task)
			assert.Equal(t, tc.expected, task.Script())
			assert.Equal(t, 30*time.Second, task.Timeout())
		})
	}
}

// =============================================================================
// daemon_status.go
// =============================================================================

func TestCheckDaemonStatus(t *testing.T) {
	task := CheckDaemonStatus()
	require.NotNil(t, task)

	assert.Equal(t, "Check Daemon Status", task.Name())
	assert.Contains(t, task.Script(), "supervisorctl")
}

func TestParseDaemonStatusOutput(t *testing.T) {
	output := strings.Join([]string{
		`{"daemon_id":"d1","status":"running","pid":"1234","uptime_seconds":90}`,
		"",
		"not json, ignored",
		`{"daemon_id":"d2","status":"stopped","pid":"","uptime_seconds":0}`,
		"===STATUS_CHECK_COMPLETE===",
	}, "\n")

	statuses := ParseDaemonStatusOutput(output)
	require.Len(t, statuses, 2)

	assert.Equal(t, "d1", statuses[0].DaemonID)
	assert.Equal(t, "running", statuses[0].Status)
	assert.Equal(t, "1234", statuses[0].PID)
	assert.Equal(t, 90, statuses[0].UptimeSeconds)
	assert.Equal(t, "d2", statuses[1].DaemonID)
	assert.Equal(t, "stopped", statuses[1].Status)
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{seconds: 0, want: ""},
		{seconds: -1, want: ""},
		{seconds: 45, want: "45 seconds"},
		{seconds: 90, want: "1 minute, 30 seconds"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, FormatUptime(tc.seconds), "FormatUptime(%d)", tc.seconds)
	}

	// Longer spans vary in wording between implementations; assert the
	// components rather than an exact string.
	day := FormatUptime(90000)
	assert.Contains(t, day, "day")
}

// =============================================================================
// file_operations.go
// =============================================================================

func TestUploadFile(t *testing.T) {
	contents := "hello world\n"
	task := UploadFile(UploadFileConfig{
		Path:     "/etc/launch/app.conf",
		Contents: contents,
		Mode:     "600",
	})
	require.NotNil(t, task)

	assert.Equal(t, "Upload File", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, taskrunner.ShellQuote("/etc/launch/app.conf"))
	assert.Contains(t, script, base64.StdEncoding.EncodeToString([]byte(contents)),
		"contents are shipped base64-encoded so arbitrary bytes survive the shell")
	assert.Contains(t, script, "chmod 600")
}

func TestUploadFileFallsBackToDefaultMode(t *testing.T) {
	for _, mode := range []string{"", "64", "89a", "12345", "abc"} {
		task := UploadFile(UploadFileConfig{Path: "/tmp/f", Contents: "x", Mode: mode})
		require.NotNil(t, task)
		assert.Contains(t, task.Script(), "chmod 644", "mode %q should fall back to 644", mode)
	}
}

func TestValidFileMode(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{mode: "644", want: true},
		{mode: "0644", want: true},
		{mode: "777", want: true},
		{mode: "", want: false},
		{mode: "64", want: false},
		{mode: "12345", want: false},
		{mode: "648", want: false},
		{mode: "abc", want: false},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, validFileMode(tc.mode), "validFileMode(%q)", tc.mode)
	}
}

func TestDeleteFile(t *testing.T) {
	task := DeleteFile(DeleteFileConfig{Path: "/etc/launch/app.conf"})
	require.NotNil(t, task)

	assert.Equal(t, "Delete File", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, taskrunner.ShellQuote("/etc/launch/app.conf"))
	assert.Contains(t, script, "does not exist, skipping")
}

func TestGetFile(t *testing.T) {
	task := GetFile(GetFileConfig{Path: "/var/log/app.log", MaxBytes: 2048})
	require.NotNil(t, task)

	assert.Equal(t, "Get File", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())
	assert.Contains(t, task.Script(), "tail -c 2048")
	assert.Contains(t, task.Script(), taskrunner.ShellQuote("/var/log/app.log"))
}

func TestGetFileDefaultsToOneMegabyte(t *testing.T) {
	for _, maxBytes := range []int{0, -1} {
		task := GetFile(GetFileConfig{Path: "/var/log/app.log", MaxBytes: maxBytes})
		require.NotNil(t, task)
		assert.Contains(t, task.Script(), "tail -c 1048576", "MaxBytes %d should default to 1MB", maxBytes)
	}
}

// =============================================================================
// daemon.go
// =============================================================================

func TestUploadDaemon(t *testing.T) {
	task := UploadDaemon(UploadDaemonConfig{
		Path:         "/etc/supervisor/conf.d/daemon-1.conf",
		Contents:     "[program:daemon-1]",
		LogPath:      "/home/launch/.launch/daemon-1.log",
		ErrorLogPath: "/home/launch/.launch/daemon-1.error.log",
		User:         "launch",
	})
	require.NotNil(t, task)

	assert.Equal(t, "Upload Daemon Config", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "/etc/supervisor/conf.d/daemon-1.conf")
	assert.Contains(t, script, "[program:daemon-1]")
	assert.Contains(t, script, "Daemon config uploaded successfully")
}

func TestDeleteDaemon(t *testing.T) {
	task := DeleteDaemon(DeleteDaemonConfig{
		Path:        "/etc/supervisor/conf.d/daemon-1.conf",
		ProgramName: "daemon-1",
	})
	require.NotNil(t, task)

	assert.Equal(t, "Delete Daemon", task.Name())
	assert.Equal(t, time.Minute, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, `supervisorctl stop "daemon-1":*`)
	assert.Contains(t, script, "/etc/supervisor/conf.d/daemon-1.conf")
	assert.Contains(t, script, "supervisorctl update")
}

func TestRestartDaemon(t *testing.T) {
	task := RestartDaemon(RestartDaemonConfig{ProgramName: "daemon-1"})
	require.NotNil(t, task)

	assert.Equal(t, "Restart Daemon", task.Name())
	assert.Equal(t, time.Minute, task.Timeout())
	assert.Contains(t, task.Script(), `supervisorctl restart "daemon-1":*`)
}

func TestReloadSupervisor(t *testing.T) {
	task := ReloadSupervisor()
	require.NotNil(t, task)

	assert.Equal(t, "Reload Supervisor", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "supervisorctl reread")
	assert.Contains(t, script, "supervisorctl update")
}

// =============================================================================
// vulnerability_audit.go
// =============================================================================

func TestVulnerabilityAudit(t *testing.T) {
	task := VulnerabilityAudit()
	require.NotNil(t, task)

	assert.Equal(t, "Vulnerability Audit", task.Name())
	assert.Equal(t, 30*time.Minute, task.Timeout())

	script := task.Script()
	for _, section := range []string{
		"=== SYSTEM INFORMATION ===",
		"=== SECURITY UPDATES ===",
		"=== NETWORK SECURITY ===",
		"=== SSH SECURITY ===",
		"=== FILE PERMISSIONS ===",
		"=== USER ACCOUNTS ===",
		"=== RUNNING SERVICES ===",
		"=== LOG ANALYSIS ===",
		"=== SCHEDULED TASKS ===",
		"=== AUDIT SUMMARY ===",
	} {
		assert.Contains(t, script, section)
	}

	// The report is written to a temp file, streamed back, then removed —
	// leaving it behind would accumulate on every audit run.
	assert.Contains(t, script, `rm -f "$REPORT_FILE"`)
}
