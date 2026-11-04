package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetemplates "github.com/kkz6/launch-go/internal/modules/site/tasks/templates"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	tasktemplates "github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

const candidateRelease = "20261104120000-01h00000000000000000000000"

func TestZeroDowntimeRetentionAfterPromotion(t *testing.T) {
	registerReleaseTemplates(t)
	for _, phase := range []string{"success", "before", "after"} {
		t.Run(phase, func(t *testing.T) {
			root, site := releaseFixture(t)
			obsolete := createRelease(t, site.Path, "20261101000000")
			previous := createRelease(t, site.Path, "20261102000000")
			unused := createRelease(t, site.Path, "20261103000000")
			current := filepath.Join(site.Path, "current")
			require.NoError(t, os.Symlink(previous, current))
			before := "test -f '" + escapeShellPath(filepath.Join(obsolete, "sentinel")) + "'\n"
			after := before
			if phase == "before" {
				before += "exit 23"
			}
			if phase == "after" {
				after += "exit 24"
			}
			site.HookBeforeMakingCurrent = &before
			site.HookAfterMakingCurrent = &after
			script := buildZeroDowntimeScript(DeployOptions{Site: site, Deployment: &models.Deployment{}, ReleaseTimestamp: candidateRelease})
			err := runReleaseScript(t, root, script)
			candidate := filepath.Join(site.Path, "releases", candidateRelease)
			if phase == "success" {
				require.NoError(t, err)
				require.NoDirExists(t, obsolete)
				require.NoDirExists(t, unused)
			} else {
				require.Error(t, err)
				require.DirExists(t, obsolete)
				require.DirExists(t, unused)
			}
			require.FileExists(t, filepath.Join(previous, "sentinel"))
			require.DirExists(t, candidate)
			want := candidate
			if phase == "before" {
				want = previous
			}
			assertCurrentRelease(t, current, want)
		})
	}
}

func TestZeroDowntimeFirstRelease(t *testing.T) {
	registerReleaseTemplates(t)
	root, site := releaseFixture(t)
	site.InstalledAt = nil
	script := buildZeroDowntimeScript(DeployOptions{Site: site, Deployment: &models.Deployment{}, ReleaseTimestamp: candidateRelease})
	require.NoError(t, runReleaseScript(t, root, script))
	assertCurrentRelease(t, filepath.Join(site.Path, "current"), filepath.Join(site.Path, "releases", candidateRelease))
}

func TestZeroDowntimeRejectsExistingCandidateBeforeHooks(t *testing.T) {
	registerReleaseTemplates(t)
	for _, candidateKind := range []string{"serving", "inactive", "symlink"} {
		t.Run(candidateKind, func(t *testing.T) {
			root, site := releaseFixture(t)
			current := filepath.Join(site.Path, "current")
			candidate := filepath.Join(site.Path, "releases", candidateRelease)
			live := candidate
			if candidateKind == "serving" {
				createRelease(t, site.Path, candidateRelease)
			} else if candidateKind == "inactive" {
				live = createRelease(t, site.Path, "20260101000000")
				createRelease(t, site.Path, candidateRelease)
			} else {
				live = createRelease(t, site.Path, "20260101000000")
				recovery := createRelease(t, site.Path, "20260201000000")
				require.NoError(t, os.Symlink(recovery, candidate))
			}
			require.NoError(t, os.Symlink(live, current))
			hook := "touch mutating-hook-ran"
			site.HookBeforeUpdatingRepository = &hook
			script := buildZeroDowntimeScript(DeployOptions{Site: site, Deployment: &models.Deployment{}, ReleaseTimestamp: candidateRelease})
			require.Error(t, runReleaseScript(t, root, script))
			require.NoFileExists(t, filepath.Join(candidate, "mutating-hook-ran"))
			assertCurrentRelease(t, current, live)
		})
	}
}

func TestReleaseCleanupProtectsOlderLiveReleaseAndUnrelatedEntries(t *testing.T) {
	registerReleaseTemplates(t)
	root, site := releaseFixture(t)
	old := createRelease(t, site.Path, "20260101000000")
	live := createRelease(t, site.Path, "20260201000000")
	previous := createRelease(t, site.Path, "20260301000000")
	candidate := createRelease(t, site.Path, candidateRelease)
	latest := createRelease(t, site.Path, "20261201000000-01H00000000000000000000000")
	unrelated := createRelease(t, site.Path, "user-data")
	link := filepath.Join(site.Path, "releases", "20250101000000")
	require.NoError(t, os.Symlink(unrelated, link))
	current := filepath.Join(site.Path, "current")
	require.NoError(t, os.Symlink(live, current))
	script := tasktemplates.ShellDefaultsLenient() + "\nPREVIOUS_RELEASE='" + escapeShellPath(previous) + "'\n" +
		tasktemplates.MustRender("site", "deployment/cleanup_old_releases.sh", map[string]any{
			"ReleasesDirectory": filepath.Join(site.Path, "releases"), "CurrentDirectory": current,
			"LatestDeploymentTimestamp": candidateRelease, "RetentionCount": 1,
		})
	require.NoError(t, runReleaseScript(t, root, script))
	require.NoDirExists(t, old)
	for _, dir := range []string{live, previous, candidate, latest, unrelated} {
		require.FileExists(t, filepath.Join(dir, "sentinel"))
	}
	_, err := os.Readlink(link)
	require.NoError(t, err)
	assertCurrentRelease(t, current, live)
}

func TestRollbackPreservesCurrentOnInvalidTarget(t *testing.T) {
	registerReleaseTemplates(t)
	for _, targetKind := range []string{"missing", "symlink", "valid"} {
		t.Run(targetKind, func(t *testing.T) {
			root, site := releaseFixture(t)
			old := createRelease(t, site.Path, "20260101000000")
			current := filepath.Join(site.Path, "current")
			require.NoError(t, os.Symlink(old, current))
			target := filepath.Join(site.Path, "releases", candidateRelease)
			if targetKind == "valid" {
				createRelease(t, site.Path, candidateRelease)
			} else if targetKind == "symlink" {
				require.NoError(t, os.Symlink(old, target))
			}
			script := RollbackDeployment(RollbackDeploymentConfig{SitePath: site.Path, ReleaseDirectory: target, CurrentDirectory: current}).Script()
			err := runReleaseScript(t, root, script)
			want := old
			if targetKind == "valid" {
				require.NoError(t, err)
				want = target
			} else {
				require.Error(t, err)
			}
			assertCurrentRelease(t, current, want)
			require.FileExists(t, filepath.Join(old, "sentinel"))
			staging, err := filepath.Glob(current + ".switch.*")
			require.NoError(t, err)
			require.Empty(t, staging)
		})
	}
}

func TestAtomicPromotionRefusesExistingDirectory(t *testing.T) {
	registerReleaseTemplates(t)
	root, site := releaseFixture(t)
	target := createRelease(t, site.Path, candidateRelease)
	current := filepath.Join(site.Path, "current")
	require.NoError(t, os.MkdirAll(current, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(current, "sentinel"), []byte("serving"), 0o600))
	script := RollbackDeployment(RollbackDeploymentConfig{SitePath: site.Path, ReleaseDirectory: target, CurrentDirectory: current}).Script()
	require.Error(t, runReleaseScript(t, root, script))
	contents, err := os.ReadFile(filepath.Join(current, "sentinel"))
	require.NoError(t, err)
	require.Equal(t, "serving", string(contents))
}

func TestAtomicPromotionHasNoMissingCurrentWindow(t *testing.T) {
	registerReleaseTemplates(t)
	root, _ := releaseFixture(t)
	// Docker Desktop's shared host filesystem does not have Linux rename
	// semantics. Exercise concurrent readers on the native test filesystem.
	atomicRoot := filepath.Join("/tmp", "launch-atomic-"+filepath.Base(filepath.Dir(root)))
	first := filepath.Join(atomicRoot, "first")
	second := filepath.Join(atomicRoot, "second")
	current := filepath.Join(atomicRoot, "current")
	marker := filepath.Join(atomicRoot, "reader-started")
	failure := filepath.Join(atomicRoot, "missing-current")
	script := tasktemplates.ShellDefaultsLenient() + "\nmkdir -p '" + first + "' '" + second + "'\n" +
		"touch '" + filepath.Join(first, "sentinel") + "' '" + filepath.Join(second, "sentinel") + "'\n" +
		"ln -s '" + first + "' '" + current + "'\n(\n touch '" + marker + "'\n while true; do\n" +
		"  if [ ! -f '" + escapeShellPath(filepath.Join(current, "sentinel")) + "' ]; then touch '" + failure + "'; exit; fi\n done\n) &\n" +
		"READER_PID=$!\ntrap 'kill \"$READER_PID\" 2>/dev/null || true; wait \"$READER_PID\" 2>/dev/null || true; rm -rf -- \"" + atomicRoot + "\"' EXIT\n" +
		"while [ ! -f '" + marker + "' ]; do sleep 0.01; done\n"
	for i := 0; i < 20; i++ {
		for _, target := range []string{second, first} {
			script += tasktemplates.MustRender("site", "deployment/make_deployment_current.sh", RollbackDeploymentConfig{
				SitePath: atomicRoot, ReleaseDirectory: target, CurrentDirectory: current,
			}) + "\n"
		}
	}
	script += "test ! -f '" + failure + "'\ntest \"$(readlink '" + current + "')\" = '" + first + "'\n"
	require.NoError(t, runReleaseScript(t, root, script))
}

func TestDeploymentLockRejectsConcurrentDeployAndRollback(t *testing.T) {
	registerReleaseTemplates(t)
	for _, operation := range []string{"deploy", "rollback"} {
		t.Run(operation, func(t *testing.T) {
			root, site := releaseFixture(t)
			previous := createRelease(t, site.Path, "20260101000000")
			current := filepath.Join(site.Path, "current")
			require.NoError(t, os.Symlink(previous, current))
			target := filepath.Join(site.Path, "releases", candidateRelease)
			hook := "touch mutating-hook-ran"
			site.HookBeforeUpdatingRepository = &hook
			script := buildZeroDowntimeScript(DeployOptions{Site: site, Deployment: &models.Deployment{}, ReleaseTimestamp: candidateRelease})
			if operation == "rollback" {
				createRelease(t, site.Path, candidateRelease)
				script = RollbackDeployment(RollbackDeploymentConfig{SitePath: site.Path, ReleaseDirectory: target, CurrentDirectory: current}).Script()
			}
			scriptPath := filepath.Join(root, "operation.sh")
			require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o600))
			lockPath := escapeShellPath(filepath.Join(site.Path, ".deployment.lock"))
			harness := tasktemplates.ShellDefaultsLenient() + "\nexec 8>'" + lockPath + "'\nflock -n 8\n" +
				"if bash '" + scriptPath + "' 8>&-; then echo 'Concurrent operation unexpectedly succeeded'; exit 1; fi\n"
			require.NoError(t, runReleaseScript(t, root, harness))
			assertCurrentRelease(t, current, previous)
			require.NoFileExists(t, filepath.Join(target, "mutating-hook-ran"))
			if operation == "deploy" {
				require.NoDirExists(t, target)
			}
		})
	}
}

func TestDeploymentLockIsHeldUntilCompletionButNotByBackgroundHook(t *testing.T) {
	registerReleaseTemplates(t)
	root, site := releaseFixture(t)
	lockPath := escapeShellPath(filepath.Join(site.Path, ".deployment.lock"))
	pidPath := filepath.Join(root, "background-pid")
	finishedPath := filepath.Join(root, "hook-exit")
	hook := "exec 8>'" + lockPath + "'\nif flock -n 8; then echo 'Deployment lock released too soon'; exit 1; fi\nexec 8>&-\n" +
		"sleep 10 >/dev/null 2>&1 &\nprintf '%s' \"$!\" > '" + pidPath + "'\n" +
		"trap 'touch \"" + finishedPath + "\"' EXIT"
	site.HookAfterMakingCurrent = &hook
	script := buildZeroDowntimeScript(DeployOptions{Site: site, Deployment: &models.Deployment{}, ReleaseTimestamp: candidateRelease})
	scriptPath := filepath.Join(root, "operation.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o600))
	harness := tasktemplates.ShellDefaultsLenient() + "\n" +
		"trap 'if [ -f \"" + pidPath + "\" ]; then kill \"$(cat \"" + pidPath + "\")\" 2>/dev/null || true; fi' EXIT\n" +
		"bash '" + scriptPath + "'\nkill -0 \"$(cat '" + pidPath + "')\"\n" +
		"test -f '" + finishedPath + "'\nexec 8>'" + lockPath + "'\nflock -n 8\n"
	require.NoError(t, runReleaseScript(t, root, harness))
}

func TestZeroDowntimeCleanupPreservesDeploymentHistory(t *testing.T) {
	db := deploymentCallbackDB(t)
	for i := 0; i < 8; i++ {
		require.NoError(t, db.Exec(
			"INSERT INTO deployments (id, site_id, status, created_at) VALUES (?, ?, ?, ?)",
			fmt.Sprintf("history-%d", i), "site-1", "queued", time.Unix(int64(i), 0),
		).Error)
	}
	task := &deploySiteTask{callback: callbackData{SiteID: "site-1", ZeroDowntimeDeployment: true, DeploymentReleasesRetention: 1}}
	task.cleanupOldDeployments(context.Background(), &taskrunner.CallbackContext{DB: db})
	var count int64
	require.NoError(t, db.Model(&models.Deployment{}).Count(&count).Error)
	require.EqualValues(t, 9, count)
	task.callback.ZeroDowntimeDeployment = false
	task.cleanupOldDeployments(context.Background(), &taskrunner.CallbackContext{DB: db})
	require.NoError(t, db.Model(&models.Deployment{}).Count(&count).Error)
	require.EqualValues(t, 5, count)
}

func registerReleaseTemplates(t *testing.T) {
	t.Helper()
	tasktemplates.Reset()
	require.NoError(t, tasktemplates.Register("site", sitetemplates.FS, &tasktemplates.RegisterOptions{UseLenientShellMode: true}))
	t.Cleanup(tasktemplates.Reset)
}

func releaseFixture(t *testing.T) (string, *models.Site) {
	t.Helper()
	if runtime.GOOS != "linux" && os.Getenv("LAUNCH_TEST_DOCKER_IMAGE") == "" {
		t.Skip("requires Linux coreutils; set LAUNCH_TEST_DOCKER_IMAGE to run the scripts in a local Linux image")
	}
	root := t.TempDir()
	installed := time.Now()
	site := &models.Site{
		Path: filepath.Join(root, "site ' $HOME"), Type: sitetypes.SiteTypeGeneric,
		ZeroDowntimeDeployment: true, DeploymentReleasesRetention: 1,
	}
	site.InstalledAt = &installed
	require.NoError(t, os.MkdirAll(filepath.Join(site.Path, "releases"), 0o755))
	return root, site
}

func createRelease(t *testing.T, sitePath, name string) string {
	t.Helper()
	dir := filepath.Join(sitePath, "releases", name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sentinel"), []byte(name), 0o600))
	return dir
}

func assertCurrentRelease(t *testing.T, current, want string) {
	t.Helper()
	actual, err := filepath.EvalSymlinks(current)
	require.NoError(t, err)
	resolvedWant, err := filepath.EvalSymlinks(want)
	require.NoError(t, err)
	require.Equal(t, resolvedWant, actual)
}

func runReleaseScript(t *testing.T, root, script string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-s")
	if image := os.Getenv("LAUNCH_TEST_DOCKER_IMAGE"); image != "" {
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", "-i",
			"--mount", "type=bind,source="+root+",target="+root, image, "bash", "-s")
	}
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("script output: %s", output)
	}
	require.NoError(t, ctx.Err(), "script exceeded timeout")
	return err
}
