package tasks

import (
	"os"
	"path/filepath"
	"testing"

	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/stretchr/testify/require"
)

func TestRollbackRevalidatesTargetAfterSelection(t *testing.T) {
	registerReleaseTemplates(t)
	for _, changed := range []string{"became live", "removed", "parent symlink", "site symlink"} {
		t.Run(changed, func(t *testing.T) {
			root, site := releaseFixture(t)
			current := filepath.Join(site.Path, "current")
			live := createRelease(t, site.Path, "20261101000000")
			target := createRelease(t, site.Path, "20261102000000")
			script := RollbackDeployment(RollbackDeploymentConfig{
				SitePath: site.Path, ReleaseDirectory: target, CurrentDirectory: current,
				Runtime: RuntimeActivationConfig{SiteType: sitetypes.SiteTypeStatic},
			}).Script()
			switch changed {
			case "became live":
				live = target
			case "removed":
				require.NoError(t, os.RemoveAll(target))
			case "parent symlink":
				releases := filepath.Join(site.Path, "releases")
				moved := filepath.Join(root, "outside")
				require.NoError(t, os.Rename(releases, moved))
				require.NoError(t, os.Symlink(moved, releases))
			case "site symlink":
				moved := filepath.Join(root, "outside-site")
				require.NoError(t, os.Rename(site.Path, moved))
				require.NoError(t, os.Symlink(moved, site.Path))
			}
			require.NoError(t, os.Symlink(live, current))
			require.Error(t, runReleaseScript(t, root, script))
			assertCurrentRelease(t, current, live)
		})
	}
}
