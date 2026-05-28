package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

// applyDeployApplicationOverrides is the pure helper extracted from
// DeployApplicationJob.Handle so the GHA override branch can be
// tested without standing up the full job harness. The five cases
// below are the contract this function must hold.

func TestApplyDeployApplicationOverrides_NoOverride_LeavesCfgAlone(t *testing.T) {
	cfg := tasks.DeployConfig{
		SourceType:       dockertypes.SourceTypeGit,
		GitRepo:          "https://github.com/x/y",
		RegistryUsername: "preexisting-user",
		RegistryPassword: "preexisting-pass",
	}
	applyDeployApplicationOverrides(&cfg, DeployApplicationPayload{})

	// Empty OverrideImage = no mutation. SourceType stays git, creds
	// stay whatever the saved-credential lookup already populated.
	assert.Equal(t, dockertypes.SourceTypeGit, cfg.SourceType)
	assert.Equal(t, "https://github.com/x/y", cfg.GitRepo)
	assert.Equal(t, "preexisting-user", cfg.RegistryUsername)
	assert.Equal(t, "preexisting-pass", cfg.RegistryPassword)
}

func TestApplyDeployApplicationOverrides_FlipSourceTypeToImage(t *testing.T) {
	// Starting from a git-source workload (build_location=github_actions
	// stores source_type=git on the row, but the webhook tells us to
	// run an image-pull instead).
	cfg := tasks.DeployConfig{SourceType: dockertypes.SourceTypeGit}
	applyDeployApplicationOverrides(&cfg, DeployApplicationPayload{
		OverrideImage: "ghcr.io/kkz6/test:launch-abc1234",
	})

	assert.Equal(t, dockertypes.SourceTypeImage, cfg.SourceType,
		"override must coerce the source type to image so the deploy script runs the docker login + pull stanza")
	assert.Equal(t, "ghcr.io/kkz6/test:launch-abc1234", cfg.Image)
}

func TestApplyDeployApplicationOverrides_FullCredsSet(t *testing.T) {
	cfg := tasks.DeployConfig{SourceType: dockertypes.SourceTypeGit}
	applyDeployApplicationOverrides(&cfg, DeployApplicationPayload{
		OverrideImage:            "ghcr.io/kkz6/test:t1",
		OverrideRegistryURL:      "ghcr.io",
		OverrideRegistryUsername: "x-access-token",
		OverrideRegistryPassword: "ghs_super_secret_token",
	})

	assert.Equal(t, dockertypes.SourceTypeImage, cfg.SourceType)
	assert.Equal(t, "ghcr.io", cfg.RegistryURL)
	assert.Equal(t, "x-access-token", cfg.RegistryUsername)
	assert.Equal(t, "ghs_super_secret_token", cfg.RegistryPassword)
}

func TestApplyDeployApplicationOverrides_EmptyRegistryFieldsDontWipeExisting(t *testing.T) {
	// If the payload sets OverrideImage but leaves OverrideRegistry*
	// empty (e.g. a public image scenario), we mustn't wipe whatever
	// the saved-credential lookup populated earlier — that's a
	// subtle footgun if a future call site mixes a saved-cred app
	// with an override-image deploy.
	cfg := tasks.DeployConfig{
		SourceType:       dockertypes.SourceTypeGit,
		RegistryUsername: "from-saved-cred",
		RegistryPassword: "from-saved-cred-pw",
	}
	applyDeployApplicationOverrides(&cfg, DeployApplicationPayload{
		OverrideImage: "ghcr.io/x/y:t",
	})

	assert.Equal(t, "from-saved-cred", cfg.RegistryUsername,
		"empty override username must not wipe an existing populated value")
	assert.Equal(t, "from-saved-cred-pw", cfg.RegistryPassword)
}

func TestApplyDeployApplicationOverrides_DoesntTouchOtherFields(t *testing.T) {
	// Volumes, EnvVars, ExtraPorts, BuildConfig hydration etc. happen
	// AROUND this helper. The override path mustn't drop them.
	cfg := tasks.DeployConfig{
		SourceType:    dockertypes.SourceTypeGit,
		EnvVars:       []tasks.EnvVar{{Key: "FOO", Value: "bar"}},
		Volumes:       []tasks.Volume{{Name: "data", MountPath: "/data"}},
		ExtraPorts:    []string{"3000:3000"},
		CPULimit:      "1.5",
		MemoryLimit:   "512m",
		RestartPolicy: "unless-stopped",
	}
	applyDeployApplicationOverrides(&cfg, DeployApplicationPayload{
		OverrideImage: "ghcr.io/x/y:t",
	})

	assert.Equal(t, dockertypes.SourceTypeImage, cfg.SourceType)
	assert.Len(t, cfg.EnvVars, 1)
	assert.Equal(t, "FOO", cfg.EnvVars[0].Key)
	assert.Len(t, cfg.Volumes, 1)
	assert.Equal(t, "3000:3000", cfg.ExtraPorts[0])
	assert.Equal(t, "1.5", cfg.CPULimit)
	assert.Equal(t, "unless-stopped", cfg.RestartPolicy)
}

// --- isInstallationGone -------------------------------------------

func TestIsInstallationGone_Cases(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", assertErr("network unreachable"), false},
		{"404 wrapper", assertErr("PutContents kkz6/test/.github: status 404 body Installation not found"), true},
		{"installation_not_found shape", assertErr("github: installation_not_found"), true},
		{"installation not found", assertErr("installation not found"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isInstallationGone(tc.err))
		})
	}
}

func assertErr(s string) error { return errString(s) }

type errString string

func (e errString) Error() string { return string(e) }
