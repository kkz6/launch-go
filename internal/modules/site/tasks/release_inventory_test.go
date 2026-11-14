package tasks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/stretchr/testify/require"
)

func TestReleaseInventoryShellReadsOnlyDirectRetainedDirectories(t *testing.T) {
	root, site := releaseFixture(t)
	current := createRelease(t, site.Path, "20261109120000-current")
	createRelease(t, site.Path, "20261108120000-old")
	createRelease(t, site.Path, "invalid name")
	createRelease(t, site.Path, ".hidden")
	require.NoError(t, os.Symlink(current, filepath.Join(site.Path, "releases", "20261107120000-linked")))
	require.NoError(t, os.Symlink(current, filepath.Join(site.Path, "current")))
	lock := filepath.Join(site.Path, ".deployment.lock")
	require.NoError(t, os.WriteFile(lock, []byte("do not truncate"), 0600))
	output := filepath.Join(root, "inventory-output")
	script := "timeout 10s bash -c " + taskrunner.ShellQuote(NewReleaseInventoryTask(site.Path).Script()) + " > '" + escapeShellPath(output) + "'"
	require.NoError(t, runReleaseScript(t, root, script))
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	inventory, err := ParseReleaseInventory(string(data))
	require.NoError(t, err)
	require.Equal(t, "20261109120000-current", inventory.ActiveRelease)
	require.ElementsMatch(t, []string{"20261109120000-current", "20261108120000-old"}, inventory.Releases)
	data, err = os.ReadFile(lock)
	require.NoError(t, err)
	require.Equal(t, "do not truncate", string(data))
	assertCurrentRelease(t, filepath.Join(site.Path, "current"), current)
	require.FileExists(t, filepath.Join(current, "sentinel"))
}

func TestReleaseInventoryShellRejectsUnsafeOrUnavailableCurrent(t *testing.T) {
	for _, scenario := range []string{"external", "symlink release", "missing target", "directory current", "symlink releases", "missing lock", "symlink lock"} {
		t.Run(scenario, func(t *testing.T) {
			root, site := releaseFixture(t)
			current := createRelease(t, site.Path, "20261109120000")
			lock := filepath.Join(site.Path, ".deployment.lock")
			require.NoError(t, os.WriteFile(lock, nil, 0600))
			switch scenario {
			case "external":
				current = root
			case "symlink release":
				link := filepath.Join(site.Path, "releases", "20261108120000")
				require.NoError(t, os.Symlink(root, link))
				current = link
			case "missing target":
				current = filepath.Join(site.Path, "releases", "20261107120000")
			case "directory current":
				require.NoError(t, os.Mkdir(filepath.Join(site.Path, "current"), 0700))
			case "symlink releases":
				require.NoError(t, os.Rename(filepath.Join(site.Path, "releases"), filepath.Join(site.Path, "actual")))
				require.NoError(t, os.Symlink(filepath.Join(site.Path, "actual"), filepath.Join(site.Path, "releases")))
			case "missing lock":
				require.NoError(t, os.Remove(lock))
			case "symlink lock":
				require.NoError(t, os.Remove(lock))
				require.NoError(t, os.Symlink(filepath.Join(current, "sentinel"), lock))
			}
			if scenario != "directory current" {
				require.NoError(t, os.Symlink(current, filepath.Join(site.Path, "current")))
			}
			require.Error(t, runReleaseScript(t, root, NewReleaseInventoryTask(site.Path).Script()))
			if scenario == "missing lock" {
				require.NoFileExists(t, lock)
			}
		})
	}
}

func TestReleaseInventoryShellReportsBusyWithoutWaiting(t *testing.T) {
	root, site := releaseFixture(t)
	lock := filepath.Join(site.Path, ".deployment.lock")
	require.NoError(t, os.WriteFile(lock, nil, 0600))
	// Hold the deploy lock in the parent shell while the observation uses a
	// separate descriptor. It must exit with busy before printing live state.
	script := "exec 8>'" + escapeShellPath(lock) + "'\nflock -n 8\n(\n" + NewReleaseInventoryTask(site.Path).Script() + "\n)\ncode=$?\n[ \"$code\" -eq 75 ]"
	require.NoError(t, runReleaseScript(t, root, script))
}

func TestReleaseInventoryShellAllowsMissingCurrentWithoutGuessing(t *testing.T) {
	root, site := releaseFixture(t)
	createRelease(t, site.Path, "20261109120000")
	require.NoError(t, os.WriteFile(filepath.Join(site.Path, ".deployment.lock"), nil, 0600))
	output := filepath.Join(root, "inventory-output")
	require.NoError(t, runReleaseScript(t, root, "(\n"+NewReleaseInventoryTask(site.Path).Script()+"\n) > '"+escapeShellPath(output)+"'"))
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	inventory, err := ParseReleaseInventory(string(data))
	require.NoError(t, err)
	require.Empty(t, inventory.ActiveRelease)
	require.Equal(t, []string{"20261109120000"}, inventory.Releases)
}

func TestParseReleaseInventoryRejectsPartialOrInvalidObservations(t *testing.T) {
	for _, output := range []string{
		"LAUNCH_RELEASES_V1\nactive\tlive\nrelease\tlive\n",
		"LAUNCH_RELEASES_V1\nactive\tlive\nEND_LAUNCH_RELEASES_V1\n",
		"LAUNCH_RELEASES_V1\nactive\t../live\nEND_LAUNCH_RELEASES_V1\n",
		"LAUNCH_RELEASES_V1\nactive\t\nrelease\t../release\nEND_LAUNCH_RELEASES_V1\n",
		"LAUNCH_RELEASES_V1\nactive\t\nrelease\told\nrelease\told\nEND_LAUNCH_RELEASES_V1\n",
		"LAUNCH_RELEASES_V1\nactive\t\nactive\tlive\nEND_LAUNCH_RELEASES_V1\n",
	} {
		_, err := ParseReleaseInventory(output)
		require.Error(t, err)
	}
}
