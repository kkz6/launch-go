package tasks

import (
	"encoding/base64"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdatePHPVersionBuildsValidatedRollbackTask(t *testing.T) {
	caddyContents := "example.test {\n\tphp_fastcgi unix//run/php/php8.4-fpm.sock\n}\n"
	task := UpdatePHPVersion(UpdatePHPVersionConfig{
		SiteAddress:   "example.test",
		Version:       "8.4",
		PHPBinary:     "php8.4",
		FPMService:    "php8.4-fpm",
		FPMSocket:     "/run/php/php8.4-fpm.sock",
		CaddyfilePath: "/home/launch/example.test/Caddyfile",
		Files: []PHPVersionConfigFile{
			{
				Path:     "/home/launch/example.test/Caddyfile",
				Contents: caddyContents,
				Owner:    "launch:launch",
			},
			{
				Path:     "/etc/supervisor/conf.d/daemon-queue-1.conf",
				Contents: "[program:queue-1]\ncommand=php8.4 artisan queue:work\n",
				Owner:    "root:root",
			},
		},
		SupervisorPrograms: []string{"queue-1"},
	})

	script := task.Script()
	require.Equal(t, "Switch example.test to PHP 8.4", task.Name())
	require.Contains(t, script, `command -v "php8.4"`)
	require.Contains(t, script, `systemctl is-active --quiet "php8.4-fpm"`)
	require.Contains(t, script, `test -S "/run/php/php8.4-fpm.sock"`)
	require.Contains(t, script, "caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile")
	require.Contains(t, script, `supervisorctl restart "queue-1:*"`)
	require.Contains(t, script, "restoring previous configuration")
	require.Contains(t, script, `trap 'rollback $?' ERR`)
	require.Contains(t, script, `trap 'rollback 143' TERM`)
	require.Contains(t, script, "trap - ERR HUP INT TERM")
	require.Contains(t, script, base64.StdEncoding.EncodeToString([]byte(caddyContents)))
	require.NotContains(t, script, caddyContents)

	syntaxCheck := exec.Command("bash", "-n")
	syntaxCheck.Stdin = strings.NewReader(script)
	output, err := syntaxCheck.CombinedOutput()
	require.NoError(t, err, string(output))
}
