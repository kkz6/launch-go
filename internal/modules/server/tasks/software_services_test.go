package tasks

import (
	"fmt"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// services.go
// =============================================================================

func TestServiceLifecycleTasks(t *testing.T) {
	tests := []struct {
		name     string
		task     *taskrunner.BaseTask
		wantName string
		wantCmd  string
		wantTTL  time.Duration
	}{
		{"restart", RestartService("caddy"), "Restart caddy", "sudo service caddy restart", 30 * time.Second},
		{"stop", StopService("caddy"), "Stop caddy", "sudo service caddy stop", 30 * time.Second},
		{"start", StartService("caddy"), "Start caddy", "sudo service caddy start", 30 * time.Second},
		{"reload", ReloadService("caddy"), "Reload caddy", "sudo systemctl reload caddy", 30 * time.Second},
		{"check status", CheckServiceStatus("caddy"), "Check caddy Status", "systemctl is-active caddy", 15 * time.Second},
		{"reload caddy", ReloadCaddy(), "Reload Caddy", "sudo systemctl reload caddy", 30 * time.Second},
		{"reboot", RebootServer(), "Reboot Server", "sudo reboot", 15 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.task)
			assert.Equal(t, tc.wantName, tc.task.Name())
			assert.Equal(t, tc.wantCmd, tc.task.Script())
			assert.Equal(t, tc.wantTTL, tc.task.Timeout())
		})
	}
}

// The named restart helpers exist so callers don't have to remember the
// service name each distro uses (redis-server, not redis).
func TestNamedRestartHelpers(t *testing.T) {
	tests := []struct {
		name    string
		task    *taskrunner.BaseTask
		service string
	}{
		{"mysql", RestartMySQL(), "mysql"},
		{"postgresql", RestartPostgreSQL(), "postgresql"},
		{"redis", RestartRedis(), "redis-server"},
		{"nginx", RestartNginx(), "nginx"},
		{"supervisor", RestartSupervisor(), "supervisor"},
		{"php", RestartPhp("8.3"), "php8.3-fpm"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.task)
			assert.Equal(t, "Restart "+tc.service, tc.task.Name())
			assert.Equal(t, fmt.Sprintf("sudo service %s restart", tc.service), tc.task.Script())
		})
	}
}

// =============================================================================
// ssh_keys.go
// =============================================================================

func TestGenerateRsaKeyPair(t *testing.T) {
	task := GenerateRsaKeyPair("/root/.ssh/id_rsa", 2048)
	require.NotNil(t, task)

	assert.Equal(t, "Generate RSA Key Pair", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())
	assert.Contains(t, task.Script(), "ssh-keygen -t rsa -b 2048 -f /root/.ssh/id_rsa")
}

func TestGenerateRsaKeyPairDefaultsTo4096Bits(t *testing.T) {
	task := GenerateRsaKeyPair("/root/.ssh/id_rsa", 0)
	require.NotNil(t, task)
	assert.Contains(t, task.Script(), "-b 4096")
}

func TestGenerateEd25519KeyPair(t *testing.T) {
	task := GenerateEd25519KeyPair("/root/.ssh/id_ed25519")
	require.NotNil(t, task)

	assert.Equal(t, "Generate Ed25519 Key Pair", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())
	assert.Contains(t, task.Script(), "ssh-keygen -t ed25519 -f /root/.ssh/id_ed25519")
}

func TestGeneratePublicKey(t *testing.T) {
	task := GeneratePublicKey("/root/.ssh/id_rsa")
	require.NotNil(t, task)

	assert.Equal(t, "Generate Public Key", task.Name())
	assert.Equal(t, 15*time.Second, task.Timeout())
	assert.Equal(t, "ssh-keygen -y -f /root/.ssh/id_rsa", task.Script())
}

func TestAuthorizePublicKey(t *testing.T) {
	task := AuthorizePublicKey("ssh-ed25519 AAAAC3Nz key@host", "launch")
	require.NotNil(t, task)

	assert.Equal(t, "Authorize Public Key", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "ssh-ed25519 AAAAC3Nz key@host")
	assert.Contains(t, script, "chmod 700")
	assert.Contains(t, script, "chmod 600")
	assert.Contains(t, script, "chown -R launch:launch")
	assert.Contains(t, script, ">>", "authorize appends rather than truncating")
}

func TestDeauthorizePublicKey(t *testing.T) {
	task := DeauthorizePublicKey("ssh-ed25519 AAAAC3Nz key@host", "launch")
	require.NotNil(t, task)

	assert.Equal(t, "Deauthorize Public Key", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "sed -i")
	assert.Contains(t, script, "ssh-ed25519 AAAAC3Nz key@host")
	// The sed address uses \| | delimiters so slashes inside a key don't
	// terminate the pattern.
	assert.Contains(t, script, `\|`)
}

func TestGetAuthorizedKeys(t *testing.T) {
	task := GetAuthorizedKeys("launch")
	require.NotNil(t, task)

	assert.Equal(t, "Get Authorized Keys", task.Name())
	assert.Equal(t, 15*time.Second, task.Timeout())
	assert.Contains(t, task.Script(), "cat ")
	assert.Contains(t, task.Script(), "|| echo ''", "a missing file yields empty output, not a failure")
}

func TestUpdateAuthorizedKeys(t *testing.T) {
	task := UpdateAuthorizedKeys("launch", "ssh-ed25519 AAAAC3Nz key@host")
	require.NotNil(t, task)

	assert.Equal(t, "Update Authorized Keys", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "LAUNCH_EOF")
	assert.Contains(t, script, "ssh-ed25519 AAAAC3Nz key@host")
	assert.Contains(t, script, "chown -R launch:launch")
	assert.Contains(t, script, "cat > ", "update replaces the file wholesale")
	assert.NotContains(t, script, ">> ")
}

// =============================================================================
// software.go — install
// =============================================================================

func TestInstallSoftwareTasks(t *testing.T) {
	tests := []struct {
		name     string
		task     *taskrunner.BaseTask
		wantName string
		wantTTL  time.Duration
		contains []string
	}{
		{
			name:     "mysql",
			task:     InstallMySQL80(MySQLInstallConfig{RootPassword: "s3cret", DatabaseName: "app", PublicIPv4: "203.0.113.1"}),
			wantName: "Install MySQL 8.0",
			wantTTL:  900 * time.Second,
			contains: []string{"s3cret", "app"},
		},
		{
			name:     "postgresql",
			task:     InstallPostgreSQL16(PostgreSQLInstallConfig{DatabasePassword: "pgpass", DatabaseName: "app"}),
			wantName: "Install PostgreSQL 16",
			wantTTL:  900 * time.Second,
			contains: []string{"pgpass", "app"},
		},
		{
			name:     "caddy",
			task:     InstallCaddy2(CaddyInstallConfig{Username: "launch", PublicIPv4: "203.0.113.1"}),
			wantName: "Install Caddy 2",
			wantTTL:  600 * time.Second,
			contains: []string{"caddy"},
		},
		{
			name:     "redis",
			task:     InstallRedis(),
			wantName: "Install Redis",
			wantTTL:  300 * time.Second,
			contains: []string{"redis-server"},
		},
		{
			name:     "supervisor",
			task:     InstallSupervisor(),
			wantName: "Install Supervisor",
			wantTTL:  300 * time.Second,
			contains: []string{"supervisor"},
		},
		{
			name:     "composer",
			task:     InstallComposer2(ComposerInstallConfig{Username: "launch"}),
			wantName: "Install Composer 2",
			wantTTL:  300 * time.Second,
			contains: []string{"composer"},
		},
		{
			name:     "node",
			task:     InstallNode21(),
			wantName: "Install Node.js 21",
			wantTTL:  600 * time.Second,
			contains: []string{"nodejs"},
		},
		{
			name:     "bun",
			task:     InstallBun(),
			wantName: "Install Bun",
			wantTTL:  300 * time.Second,
			contains: []string{"bun"},
		},
		{
			name:     "php",
			task:     InstallPHP(PHPInstallConfig{Version: "8.3", Username: "launch", MaxChildren: 10}),
			wantName: "Install PHP 8.3",
			wantTTL:  900 * time.Second,
			contains: []string{"php8.3", "10"},
		},
		{
			name: "launch agent",
			task: InstallLaunchAgent(LaunchAgentInstallConfig{
				AgentConfigPath: "/etc/launch-agent/config.json",
				AgentURL:        "https://example.test/agent",
				RootUsername:    "root",
			}),
			wantName: "Install Launch Agent",
			wantTTL:  300 * time.Second,
			contains: []string{"https://example.test/agent"},
		},
		{
			name:     "update launch agent",
			task:     UpdateLaunchAgent(),
			wantName: "Update Launch Agent",
			wantTTL:  300 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.task)
			assert.Equal(t, tc.wantName, tc.task.Name())
			assert.Equal(t, tc.wantTTL, tc.task.Timeout())
			assert.NotEmpty(t, tc.task.Script())

			for _, want := range tc.contains {
				assert.Contains(t, tc.task.Script(), want)
			}
		})
	}
}

func TestInstallConfigDefaults(t *testing.T) {
	// MaxConnections 0 would render an unusable my.cnf.
	mysql := InstallMySQL80(MySQLInstallConfig{RootPassword: "x", DatabaseName: "app"})
	assert.Contains(t, mysql.Script(), "100")

	// MaxChildren 0 would render a php-fpm pool that accepts no requests.
	php := InstallPHP(PHPInstallConfig{Version: "8.3", Username: "launch"})
	assert.Contains(t, php.Script(), "5")

	// An empty RootUsername would render "chown :" and fail.
	agent := InstallLaunchAgent(LaunchAgentInstallConfig{
		AgentConfigPath: "/etc/launch-agent/config.json",
		AgentURL:        "https://example.test/agent",
	})
	assert.Contains(t, agent.Script(), "root")
}

// =============================================================================
// software.go — remove
// =============================================================================

func TestRemoveSoftwareTasks(t *testing.T) {
	tests := []struct {
		name     string
		task     *taskrunner.BaseTask
		wantName string
		wantTTL  time.Duration
	}{
		{"redis", RemoveRedis(), "Remove Redis", 300 * time.Second},
		{"supervisor", RemoveSupervisor(), "Remove Supervisor", 300 * time.Second},
		{"launch agent", RemoveLaunchAgent(), "Remove Launch Agent", 120 * time.Second},
		{"mysql", RemoveMySQL(), "Remove MySQL", 300 * time.Second},
		{"postgresql", RemovePostgreSQL(), "Remove PostgreSQL", 300 * time.Second},
		{"php", RemovePHP(PHPRemoveConfig{Version: "8.3"}), "Remove PHP 8.3", 300 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.task)
			assert.Equal(t, tc.wantName, tc.task.Name())
			assert.Equal(t, tc.wantTTL, tc.task.Timeout())
			assert.NotEmpty(t, tc.task.Script())
		})
	}
}

// =============================================================================
// software.go — enum-driven install/remove
// =============================================================================

func TestInstallSoftwareByEnum(t *testing.T) {
	config := SoftwareInstallConfig{
		Username:         "launch",
		MemoryInMB:       4096,
		DatabasePassword: "s3cret",
		DatabaseName:     "app",
		PublicIPv4:       "203.0.113.1",
	}

	for _, software := range []types.Software{
		types.SoftwarePhp83,
		types.SoftwareRedis,
		types.SoftwareSupervisor,
		types.SoftwareMySQL80,
		types.SoftwarePostgreSQL16,
		types.SoftwareCaddy2,
		types.SoftwareNode21,
		types.SoftwareBun,
	} {
		t.Run(string(software), func(t *testing.T) {
			task := InstallSoftware(software, config)
			require.NotNil(t, task)

			assert.Equal(t, "Install "+software.Label(), task.Name())
			assert.Equal(t, 900*time.Second, task.Timeout())
			assert.NotEmpty(t, task.Script())
		})
	}
}

// MaxChildren and MaxConnections are sized from server memory, but only for
// the software kind they apply to — the other keeps its default.
func TestInstallSoftwareSizesFromMemory(t *testing.T) {
	php := types.SoftwarePhp83
	sized := InstallSoftware(php, SoftwareInstallConfig{Username: "launch", MemoryInMB: 8192})
	unsized := InstallSoftware(php, SoftwareInstallConfig{Username: "launch"})

	assert.NotEqual(t, sized.Script(), unsized.Script(),
		"an 8GB box should get a different pm.max_children than the default")
	assert.Contains(t, unsized.Script(), "5")

	db := types.SoftwareMySQL80
	sizedDB := InstallSoftware(db, SoftwareInstallConfig{MemoryInMB: 8192, DatabasePassword: "x"})
	unsizedDB := InstallSoftware(db, SoftwareInstallConfig{DatabasePassword: "x"})

	assert.NotEqual(t, sizedDB.Script(), unsizedDB.Script())
	assert.Contains(t, unsizedDB.Script(), "100")
}

// MySQL and PostgreSQL name their install templates with the version
// (install_mysql80.sh) but their remove templates without it
// (remove_mysql.sh), so deriving the remove name from the enum alone
// resolved to files that were never written.
func TestRemoveSoftwareByEnum(t *testing.T) {
	for _, software := range []types.Software{
		types.SoftwarePhp83,
		types.SoftwareRedis,
		types.SoftwareSupervisor,
		types.SoftwareMySQL80,
		types.SoftwarePostgreSQL16,
		types.SoftwareLaunchAgent,
	} {
		t.Run(string(software), func(t *testing.T) {
			assert.True(t, software.SupportsRemove())

			task, err := RemoveSoftware(software)
			require.NoError(t, err)
			require.NotNil(t, task)

			assert.Equal(t, "Remove "+software.Label(), task.Name())
			assert.Equal(t, 300*time.Second, task.Timeout())
			assert.NotEmpty(t, task.Script())
		})
	}
}

// No removal script ships for these. RemoveSoftware runs inside
// RemoveServiceJob, and a panic there unwinds past the job wrapper so
// Failed() never runs — the service would sit in "uninstalling" while asynq
// recovered the panic and retried until the task was archived. Failing
// cleanly lets Failed() restore the previous status.
func TestRemoveSoftwareWithoutARemovalScript(t *testing.T) {
	for _, software := range []types.Software{
		types.SoftwareCaddy2,
		types.SoftwareCaddy2LB,
		types.SoftwareComposer2,
		types.SoftwareNode21,
		types.SoftwareBun,
		types.SoftwareDocker,
		types.SoftwareTraefik,
	} {
		t.Run(string(software), func(t *testing.T) {
			assert.False(t, software.SupportsRemove())

			task, err := RemoveSoftware(software)
			require.Error(t, err)
			assert.Nil(t, task)
			assert.Contains(t, err.Error(), "cannot be uninstalled")
		})
	}
}

// Software.SupportsRemove is a static list, so it can drift from the scripts
// that actually ship. This is the guard: for every software the enum knows
// about, the answer must match whether its removal template is registered.
// Adding a remove_*.sh without updating SupportsRemove — or the reverse —
// fails here rather than in production.
func TestSupportsRemoveMatchesShippedTemplates(t *testing.T) {
	for _, software := range types.AllSoftware() {
		t.Run(string(software), func(t *testing.T) {
			registered := templates.Has("server", software.RemoveTemplateName())
			assert.Equal(t, registered, software.SupportsRemove(),
				"SupportsRemove says %v but template %q registered=%v",
				software.SupportsRemove(), software.RemoveTemplateName(), registered)
		})
	}
}

// MySQL's repo key is one long-lived key whose expiry is extended in place.
// Public keyservers serve stale copies of that self-signature — a server
// provisioned via keyserver.ubuntu.com ended up rejecting the repo with
// EXPKEYSIG against the very same key ID that MySQL publishes as valid.
func TestInstallMySQLFetchesItsKeyFromMySQL(t *testing.T) {
	script := InstallMySQL80(MySQLInstallConfig{
		RootPassword: "secret",
		DatabaseName: "launch",
		PublicIPv4:   "203.0.113.1",
	}).Script()

	assert.Contains(t, script, "https://repo.mysql.com/RPM-GPG-KEY-mysql-2025",
		"the key must come from MySQL's own published file")
	// Checks the flags, not the word — the comment in the script explains
	// why keyserver.ubuntu.com is avoided and should keep saying so.
	assert.NotContains(t, script, "--recv-keys",
		"keyservers serve lapsed self-signatures for this key")
	assert.NotContains(t, script, "--keyserver ")

	// The published key files are year-suffixed and rotate, so a single
	// hardcoded URL would break the same way the hardcoded keyserver did.
	assert.Contains(t, script, "RPM-GPG-KEY-mysql-2023", "needs a fallback key file")
	assert.Contains(t, script, "expired:", "an expired key must be rejected, not installed")
}

// apt-get update exits 0 when a repository fails signature verification — it
// warns and reuses the previous index. Trusting that exit code is how a
// broken repo previously reached the install step and failed there instead.
func TestInstallMySQLVerifiesTheRepoResolves(t *testing.T) {
	script := InstallMySQL80(MySQLInstallConfig{RootPassword: "secret", DatabaseName: "launch"}).Script()

	assert.Contains(t, script, "apt-cache policy mysql-community-server")
	assert.Contains(t, script, "repo.mysql.com")
	assert.Contains(t, script, "MYSQL_REPO_SETUP_SUCCESS=true")
	assert.Contains(t, script, "falling back to Ubuntu's mysql-server package",
		"a failed repo must still yield a working MySQL")
}
