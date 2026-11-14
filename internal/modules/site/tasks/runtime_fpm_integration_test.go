package tasks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
)

// Build the disposable fixture with:
// docker build -t launchctl-runtime-test tests/fixtures/php-fpm
// LAUNCH_TEST_PHP_FPM_IMAGE=launchctl-runtime-test go test ./internal/modules/site/tasks -run TestRealPHPFPMRuntimeActivation -v
func TestRealPHPFPMRuntimeActivation(t *testing.T) {
	image := os.Getenv("LAUNCH_TEST_PHP_FPM_IMAGE")
	if image == "" {
		t.Skip("set LAUNCH_TEST_PHP_FPM_IMAGE to run against an isolated PHP-FPM container")
	}
	registerReleaseTemplates(t)
	root := t.TempDir()
	version, installed := sitetypes.PhpVersion83, time.Now()
	site := &models.Site{Path: "/tmp/runtime-test/site", Type: sitetypes.SiteTypeGeneric, PhpVersion: &version, User: "launcher", ZeroDowntimeDeployment: true, DeploymentReleasesRetention: 2}
	site.InstalledAt = &installed
	hook := `printf '%s' '<?php echo "RELEASE_B";' > index.php`
	site.HookBeforeMakingCurrent = &hook
	runtime := RuntimeActivationConfig{SitePath: site.Path, SiteType: site.Type, PhpVersion: site.PhpVersion}
	refresh, err := RefreshRuntime(runtime)
	require.NoError(t, err)
	for name, script := range map[string]string{
		"deploy":   buildZeroDowntimeScript(DeployOptions{Site: site, Deployment: &models.Deployment{}, ReleaseTimestamp: candidateRelease}),
		"rollback": RollbackDeployment(RollbackDeploymentConfig{SitePath: site.Path, ReleaseDirectory: site.Path + "/releases/20261101000000", CurrentDirectory: site.Path + "/current", Runtime: runtime}).Script(),
		"promote":  RollbackDeployment(RollbackDeploymentConfig{SitePath: site.Path, ReleaseDirectory: site.Path + "/releases/" + candidateRelease, CurrentDirectory: site.Path + "/current", Runtime: runtime}).Script(),
		"refresh":  refresh.Script(),
	} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name+".sh"), []byte(script), 0o644))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", "-i", "--mount", "type=bind,source="+root+",target=/fixture,readonly", image, "bash", "-s")
	cmd.Stdin = strings.NewReader(realFPMFixture)
	output, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "PHP-FPM fixture exceeded timeout: %s", output)
	require.NoError(t, err, "PHP-FPM fixture failed: %s", output)
	require.Contains(t, string(output), "REAL_PHP_FPM_ACTIVATION_PASSED")
	t.Log(string(output))
}

const realFPMFixture = `set -euo pipefail
mkdir -p /tmp/runtime-test/site/releases/20261101000000 /run/php
chmod 755 /tmp/runtime-test
printf '%s' '<?php echo "RELEASE_A";' > /tmp/runtime-test/site/releases/20261101000000/index.php
ln -s /tmp/runtime-test/site/releases/20261101000000 /tmp/runtime-test/site/current
chown -R launcher:launcher /tmp/runtime-test/site
cat > /etc/php/8.3/fpm/conf.d/99-runtime-test.ini <<'INI'
opcache.enable=1
opcache.validate_timestamps=0
opcache.file_update_protection=0
realpath_cache_ttl=3600
INI
/usr/sbin/service php8.3-fpm start
request() {
    SCRIPT_FILENAME=/tmp/runtime-test/site/current/index.php REQUEST_METHOD=GET SCRIPT_NAME=/index.php /usr/bin/cgi-fcgi -bind -connect /run/php/php8.3-fpm.sock
}
expect_response() {
    for attempt in $(seq 1 100); do
        response=$(request 2>/dev/null || true)
        if [[ "$response" == *"$1"* ]]; then return 0; fi
        sleep 0.1
    done
    echo "Expected $1, got: $response" >&2
    exit 1
}
expect_response RELEASE_A
# Prove the fixture has a warm persistent OPcache before switching releases.
printf '%s' '<?php echo "UNRELOADED_EDIT";' > /tmp/runtime-test/site/releases/20261101000000/index.php
expect_response RELEASE_A
printf '%s' '<?php echo "RELEASE_A";' > /tmp/runtime-test/site/releases/20261101000000/index.php
runuser -u launcher -- bash /fixture/deploy.sh
expect_response RELEASE_B
runuser -u launcher -- bash /fixture/rollback.sh
expect_response RELEASE_A
# Simulate the exact missing-grant failure seen on multi-version servers.
mv /etc/sudoers.d/launchctl-fixture /tmp/runtime-test/permission
if runuser -u launcher -- bash /fixture/promote.sh > /tmp/runtime-test/failure.log 2>&1; then
    echo 'Expected the denied PHP-FPM reload to fail' >&2
    exit 1
fi
grep -q LAUNCHCTL_RELEASE_SWITCH_COMPLETED /tmp/runtime-test/failure.log
grep -q LAUNCHCTL_RUNTIME_REFRESH_FAILED /tmp/runtime-test/failure.log
if grep -q 'Rollback completed successfully' /tmp/runtime-test/failure.log; then exit 1; fi
serving=$(readlink -f /tmp/runtime-test/site/current)
test "$serving" != /tmp/runtime-test/site/releases/20261101000000
symlink_state=$(stat -c '%i:%y' /tmp/runtime-test/site/current)
mv /tmp/runtime-test/permission /etc/sudoers.d/launchctl-fixture
runuser -u launcher -- bash /fixture/refresh.sh
test "$symlink_state" = "$(stat -c '%i:%y' /tmp/runtime-test/site/current)"
test "$serving" = "$(readlink -f /tmp/runtime-test/site/current)"
expect_response RELEASE_B
echo REAL_PHP_FPM_ACTIVATION_PASSED
`
