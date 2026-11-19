package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
)

func TestRuntimeActivationUsesSelectedPHPAndSkipsStaticSites(t *testing.T) {
	registerReleaseTemplates(t)
	version := sitetypes.PhpVersion83
	for _, siteType := range []sitetypes.SiteType{sitetypes.SiteTypeLaravel, sitetypes.SiteTypeGeneric, sitetypes.SiteTypeWordpress, sitetypes.SiteTypePhpMyAdmin} {
		t.Run(string(siteType), func(t *testing.T) {
			task, err := RefreshRuntime(RuntimeActivationConfig{SitePath: "/home/launcher/app", SiteType: siteType, PhpVersion: &version})
			require.NoError(t, err)
			require.Contains(t, task.Script(), "sudo -n /usr/sbin/service php8.3-fpm reload")
			require.Contains(t, task.Script(), "Refreshing PHP runtime (php8.3-fpm)...")
			require.Contains(t, task.Script(), "PHP runtime refreshed (php8.3-fpm).")
			require.NotContains(t, task.Script(), "LAUNCHCTL_RUNTIME_REFRESH_")
			require.Contains(t, task.Script(), "exec 8<'/usr/sbin/php-fpm8.3'")
			require.Contains(t, task.Script(), "flock -x -w 30 8")
			require.NotContains(t, task.Script(), " restart")
			require.NotContains(t, task.Script(), "artisan")
			require.NotContains(t, task.Script(), "mv -Tf")
		})
	}
	task, err := RefreshRuntime(RuntimeActivationConfig{SitePath: "/home/launcher/static", SiteType: sitetypes.SiteTypeStatic})
	require.NoError(t, err)
	require.NotContains(t, task.Script(), "sudo")
	require.NotContains(t, task.Script(), "php-fpm")
}

func TestDeploymentRuntimeLogsShowReadableProgress(t *testing.T) {
	registerReleaseTemplates(t)
	for _, mode := range []string{"standard", "zero_downtime"} {
		t.Run(mode, func(t *testing.T) {
			root, site := releaseFixture(t)
			// The standard deployment builder requires an unquoted directory name.
			site.Path = filepath.Join(root, "site")
			version := sitetypes.PhpVersion83
			site.Type, site.PhpVersion = sitetypes.SiteTypeGeneric, &version
			fixture := fakeRuntime(t, root, "")
			opts := DeployOptions{Site: site, Deployment: &models.Deployment{}, ReleaseTimestamp: candidateRelease}
			script := buildStandardScript(opts)
			if mode == "zero_downtime" {
				script = buildZeroDowntimeScript(opts)
			}
			output := filepath.Join(root, "deployment.log")
			require.NoError(t, runReleaseScript(t, root, "(\n"+fixture.wrap(script)+"\n) > '"+escapeShellPath(output)+"' 2>&1"))
			log, err := os.ReadFile(output)
			require.NoError(t, err)
			require.Contains(t, string(log), "Refreshing PHP runtime (php8.3-fpm)...")
			require.Contains(t, string(log), "PHP runtime refreshed (php8.3-fpm).")
			require.Contains(t, string(log), "Done!")
			require.NotContains(t, string(log), "LAUNCHCTL_")
		})
	}
}

func TestRuntimeActivationRejectsInvalidConfigurationBeforeCodeChanges(t *testing.T) {
	registerReleaseTemplates(t)
	invalid := sitetypes.PhpVersion("83; touch /tmp/injected")
	for _, config := range []RuntimeActivationConfig{
		{SitePath: "/app", SiteType: sitetypes.SiteTypeLaravel},
		{SitePath: "/app", SiteType: sitetypes.SiteTypeLaravel, PhpVersion: &invalid},
		{SitePath: "/app", SiteType: sitetypes.SiteType("unknown")},
	} {
		_, err := RefreshRuntime(config)
		require.Error(t, err)
		rollback := RollbackDeployment(RollbackDeploymentConfig{SitePath: "/app", Runtime: config}).Script()
		require.NotContains(t, rollback, "mv -Tf")
		require.Contains(t, rollback, "exit 1")
		if config.SiteType.IsValid() && config.PhpVersion == nil {
			continue // Legacy forward deployments keep their existing CLI behavior.
		}
		for _, build := range []func(DeployOptions) string{buildStandardScript, buildZeroDowntimeScript} {
			script := build(DeployOptions{Site: &models.Site{Path: config.SitePath, Type: config.SiteType, PhpVersion: config.PhpVersion}})
			require.NotContains(t, script, "mkdir")
			require.NotContains(t, script, "mv -Tf")
			require.NotContains(t, script, "/tmp/injected")
		}
	}
	_, err := RefreshRuntime(RuntimeActivationConfig{SitePath: "relative", SiteType: sitetypes.SiteTypeStatic})
	require.Error(t, err)
}

func TestLegacyDeploymentWithoutPHPVersionDoesNotGuessHTTPRuntime(t *testing.T) {
	registerReleaseTemplates(t)
	root, site := releaseFixture(t)
	site.Type = sitetypes.SiteTypeGeneric
	script := buildZeroDowntimeScript(DeployOptions{Site: site, Deployment: &models.Deployment{}, ReleaseTimestamp: candidateRelease})
	require.Contains(t, script, "PHP runtime refresh skipped: no PHP version is selected.")
	require.NotContains(t, script, "LAUNCHCTL_RUNTIME_REFRESH_")
	require.NotContains(t, script, "/usr/sbin/service")
	require.NoError(t, runReleaseScript(t, root, script))
	assertCurrentRelease(t, filepath.Join(site.Path, "current"), filepath.Join(site.Path, "releases", candidateRelease))
}

func TestRollbackRuntimeFailurePreservesPromotedCodeAndCanRetry(t *testing.T) {
	registerReleaseTemplates(t)
	root, site := releaseFixture(t)
	version := sitetypes.PhpVersion83
	config := RuntimeActivationConfig{SitePath: site.Path, SiteType: sitetypes.SiteTypeLaravel, PhpVersion: &version}
	previous := createRelease(t, site.Path, "20261101000000")
	target := createRelease(t, site.Path, "20261102000000")
	current := filepath.Join(site.Path, "current")
	require.NoError(t, os.Symlink(previous, current))
	fixture := fakeRuntime(t, root, current)
	rollback := RollbackDeployment(RollbackDeploymentConfig{SitePath: site.Path, ReleaseDirectory: target, CurrentDirectory: current, Runtime: config})
	output := filepath.Join(root, "output")
	err := runReleaseScript(t, root, "(\nexport RUNTIME_FAIL=1\n"+fixture.wrap(rollback.Script())+"\n) > '"+escapeShellPath(output)+"' 2>&1")
	require.Error(t, err)
	failureLog, readErr := os.ReadFile(output)
	require.NoError(t, readErr)
	require.Contains(t, string(failureLog), "PHP runtime refresh failed: PHP-FPM reload failed.")
	require.Contains(t, string(failureLog), "Retry runtime refresh; the current code has not been reverted.")
	require.NotContains(t, string(failureLog), "PHP runtime refreshed")
	require.NotContains(t, string(failureLog), "LAUNCHCTL_RUNTIME_REFRESH_")
	assertCurrentRelease(t, current, target)
	require.FileExists(t, filepath.Join(previous, "sentinel"))
	trace, err := os.ReadFile(fixture.trace)
	require.NoError(t, err)
	require.Contains(t, string(trace), "-n /usr/sbin/service php8.3-fpm reload")
	require.Contains(t, string(trace), target)
	before, err := os.Lstat(current)
	require.NoError(t, err)
	refresh, err := RefreshRuntime(config)
	require.NoError(t, err)
	// Capture progress while the retry uses the already-serving symlink.
	err = runReleaseScript(t, root, "(\n"+fixture.wrap(refresh.Script())+"\n) > '"+escapeShellPath(output)+"' 2>&1")
	log, readErr := os.ReadFile(output)
	require.NoError(t, readErr)
	require.NoError(t, err, string(log))
	after, err := os.Lstat(current)
	require.NoError(t, err)
	require.Equal(t, before.ModTime(), after.ModTime())
	assertCurrentRelease(t, current, target)
	require.Contains(t, string(log), "PHP runtime refreshed (php8.3-fpm).")
	require.NotContains(t, string(log), "LAUNCHCTL_")
	require.NotContains(t, string(log), "Current release updated.")
}

func TestDeploymentRuntimeFailureDoesNotPruneRetainedReleases(t *testing.T) {
	registerReleaseTemplates(t)
	root, site := releaseFixture(t)
	version := sitetypes.PhpVersion83
	site.Type, site.PhpVersion = sitetypes.SiteTypeLaravel, &version
	previous := createRelease(t, site.Path, "20261101000000")
	retained := createRelease(t, site.Path, "20261102000000")
	current := filepath.Join(site.Path, "current")
	require.NoError(t, os.Symlink(previous, current))
	fixture := fakeRuntime(t, root, current)
	script := buildZeroDowntimeScript(DeployOptions{Site: site, Deployment: &models.Deployment{}, ReleaseTimestamp: candidateRelease})
	require.Error(t, runReleaseScript(t, root, "export RUNTIME_FAIL=1\n"+fixture.wrap(script)))
	assertCurrentRelease(t, current, filepath.Join(site.Path, "releases", candidateRelease))
	require.FileExists(t, filepath.Join(previous, "sentinel"))
	require.FileExists(t, filepath.Join(retained, "sentinel"))
}

func TestRuntimeRefreshSerializesDifferentSitesUsingSamePHPVersion(t *testing.T) {
	registerReleaseTemplates(t)
	root, site := releaseFixture(t)
	fixture := fakeRuntime(t, root, "")
	version := sitetypes.PhpVersion83
	first, err := RefreshRuntime(RuntimeActivationConfig{SitePath: site.Path, SiteType: sitetypes.SiteTypeGeneric, PhpVersion: &version})
	require.NoError(t, err)
	second, err := RefreshRuntime(RuntimeActivationConfig{SitePath: filepath.Join(root, "other-site"), SiteType: sitetypes.SiteTypeGeneric, PhpVersion: &version})
	require.NoError(t, err)
	script := "set -e\n(\n" + fixture.wrap(first.Script()) + "\n) &\nfirst=$!\n(\n" + fixture.wrap(second.Script()) + "\n) &\nsecond=$!\nwait \"$first\"\nwait \"$second\"\n"
	require.NoError(t, runReleaseScript(t, root, script))
	trace, err := os.ReadFile(fixture.trace)
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(trace), "-n /usr/sbin/service php8.3-fpm reload"))
}

type runtimeFixture struct{ root, binary, trace string }

func fakeRuntime(t *testing.T, root, current string) runtimeFixture {
	t.Helper()
	f := runtimeFixture{root: root, binary: filepath.Join(root, "php-fpm8.3"), trace: filepath.Join(root, "runtime-trace")}
	require.NoError(t, os.WriteFile(f.binary, nil, 0o444))
	bin := filepath.Join(root, "bin")
	require.NoError(t, os.Mkdir(bin, 0o755))
	// A separate guard detects overlapping reloads, independent of flock.
	fakeSudo := fmt.Sprintf(`#!/bin/bash
set -e
mkdir -- '%s/reloading'
trap 'rmdir -- "$RUNTIME_ROOT/reloading"' EXIT
printf '%%s\n' "$*" >> '%s'
if [ -L '%s' ]; then readlink -f -- '%s' >> '%s'; fi
sleep 0.2
if [ "${RUNTIME_FAIL:-0}" = 1 ]; then exit 23; fi
`, escapeShellPath(root), escapeShellPath(f.trace), escapeShellPath(current), escapeShellPath(current), escapeShellPath(f.trace))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "sudo"), []byte(fakeSudo), 0o755))
	return f
}

func (f runtimeFixture) wrap(script string) string {
	return fmt.Sprintf("export PATH='%s/bin':\"$PATH\"\nexport RUNTIME_ROOT='%s'\n", escapeShellPath(f.root), escapeShellPath(f.root)) +
		strings.ReplaceAll(script, "/usr/sbin/php-fpm8.3", escapeShellPath(f.binary))
}
