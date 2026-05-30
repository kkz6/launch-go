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

// --- parseGHASourceConfig + parseRepoIdentifier -------------------
//
// Regression: the docker application/compose services persist the git
// source as `source_config.repo = "<full clone URL>"` (e.g.
// `git@github.com:owner/name.git`). The bootstrap job's parser
// previously looked for separate `owner` + `repo` keys or a combined
// `repository` field, both of which are absent — so every GHA-backed
// application failed with "owner/repo missing from source_config".
// These cases lock the URL-aware parsing in.

func TestParseRepoIdentifier_Shapes(t *testing.T) {
	cases := []struct {
		in       string
		owner    string
		repo     string
		ok       bool
		scenario string
	}{
		{"", "", "", false, "empty"},
		{"   ", "", "", false, "whitespace"},
		{"single", "", "", false, "no slash"},
		{"owner/repo", "owner", "repo", true, "bare owner/repo"},
		{"owner/repo.git", "owner", "repo", true, "trailing .git"},
		{"git@github.com:owner/repo.git", "owner", "repo", true, "scp-like ssh"},
		{"git@github.com:owner/repo", "owner", "repo", true, "scp-like ssh no suffix"},
		{"https://github.com/owner/repo.git", "owner", "repo", true, "https"},
		{"https://github.com/owner/repo", "owner", "repo", true, "https no suffix"},
		{"ssh://git@github.com/owner/repo.git", "owner", "repo", true, "ssh:// scheme"},
		{"git@github.com:", "", "", false, "scp prefix but no path"},
	}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			owner, repo, ok := parseRepoIdentifier(tc.in)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.owner, owner)
			assert.Equal(t, tc.repo, repo)
		})
	}
}

func TestParseGHASourceConfig_AcceptsCloneURLInRepoField(t *testing.T) {
	// This is the exact shape application_service.go writes today.
	raw := map[string]any{
		"repo":              "git@github.com:kkz6/launch-gha-test.git",
		"branch":            "main",
		"source_control_id": "01k0pzy8ynwnz1j4ytd7eyncdp",
	}
	cfg, err := parseGHASourceConfig(raw)
	assert.NoError(t, err)
	assert.Equal(t, "kkz6", cfg.Owner)
	assert.Equal(t, "launch-gha-test", cfg.Repo)
	assert.Equal(t, "main", cfg.Branch)
	assert.Equal(t, "01k0pzy8ynwnz1j4ytd7eyncdp", cfg.SourceControlID)
	// Defaults filled in.
	assert.Equal(t, "Dockerfile", cfg.DockerfilePath)
	assert.Equal(t, "docker-compose.yml", cfg.ComposeFilePath)
	assert.Equal(t, ".github/workflows/launch-deploy.yml", cfg.WorkflowPath)
}

func TestParseGHASourceConfig_AcceptsHTTPSCloneURL(t *testing.T) {
	cfg, err := parseGHASourceConfig(map[string]any{
		"repo":              "https://github.com/kkz6/launch-gha-test",
		"source_control_id": "01k0pzy8ynwnz1j4ytd7eyncdp",
	})
	assert.NoError(t, err)
	assert.Equal(t, "kkz6", cfg.Owner)
	assert.Equal(t, "launch-gha-test", cfg.Repo)
}

func TestParseGHASourceConfig_StillAcceptsExplicitOwnerRepo(t *testing.T) {
	// Older write-paths or admin-tooling might still set owner+repo as
	// separate keys; the parser must not regress on that contract.
	cfg, err := parseGHASourceConfig(map[string]any{
		"owner":             "kkz6",
		"repo":              "launch-gha-test",
		"source_control_id": "01k0pzy8ynwnz1j4ytd7eyncdp",
	})
	assert.NoError(t, err)
	assert.Equal(t, "kkz6", cfg.Owner)
	assert.Equal(t, "launch-gha-test", cfg.Repo)
}

func TestParseGHASourceConfig_LegacyRepositoryKey(t *testing.T) {
	cfg, err := parseGHASourceConfig(map[string]any{
		"repository":        "kkz6/launch-gha-test",
		"source_control_id": "01k0pzy8ynwnz1j4ytd7eyncdp",
	})
	assert.NoError(t, err)
	assert.Equal(t, "kkz6", cfg.Owner)
	assert.Equal(t, "launch-gha-test", cfg.Repo)
}

func TestParseGHASourceConfig_MissingSourceControlID(t *testing.T) {
	_, err := parseGHASourceConfig(map[string]any{
		"repo": "git@github.com:kkz6/launch-gha-test.git",
	})
	assert.ErrorContains(t, err, "source_control_id missing")
}

// --- ghcrImageRepository -------------------------------------------
//
// The webhook handler validates incoming `image_tag` against
// source_config.gha_image_repository (which the bootstrap writes).
// GHCR rejects mixed-case refs so the value must be lowercased.

func TestGHCRImageRepository_Lowercases(t *testing.T) {
	cases := []struct {
		owner, repo string
		want        string
	}{
		{"kkz6", "launch-gha-test", "ghcr.io/kkz6/launch-gha-test"},
		{"KKZ6", "Launch-GHA-Test", "ghcr.io/kkz6/launch-gha-test"},
		{"Acme", "MyRepo", "ghcr.io/acme/myrepo"},
		{"a", "b", "ghcr.io/a/b"},
	}
	for _, tc := range cases {
		t.Run(tc.owner+"/"+tc.repo, func(t *testing.T) {
			assert.Equal(t, tc.want, ghcrImageRepository(tc.owner, tc.repo))
		})
	}
}

func TestParseGHASourceConfig_UnparseableRepo(t *testing.T) {
	_, err := parseGHASourceConfig(map[string]any{
		"repo":              "not-a-url",
		"source_control_id": "01k0pzy8ynwnz1j4ytd7eyncdp",
	})
	assert.ErrorContains(t, err, "owner/repo missing")
}
