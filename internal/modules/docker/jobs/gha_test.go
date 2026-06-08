package jobs

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
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

// --- applicationImageSlug -------------------------------------------
//
// Two apps built from one repo must not collide on a single
// `launch-<sha>` image tag (#94). The slug keys the tag per app:
// readable name + a short app-id fragment for collision-proof
// uniqueness, lowercased and tag-charset-safe.

func TestApplicationImageSlug(t *testing.T) {
	cases := []struct {
		name, appID, want string
	}{
		// Readable name + last-8 of the (lowercased) ULID.
		{"web", "01HJXVHGRGTQRX4P0G3Y8R6CK7", "web-3y8r6ck7"},
		{"api", "01HJXVHGRGTQRX4P0G3Y8R6CK7", "api-3y8r6ck7"},
		// Same name, different app → distinct slug (the collision case).
		{"web", "01HJXVHGRGTQRX4P0G3Y8R6AAAA", "web-y8r6aaaa"},
		// Spaces/case/punctuation collapse to a clean slug.
		{"My Cool App!!", "01HJX", "my-cool-app-01hjx"},
		{"  spaced  out  ", "ABCDEFGH", "spaced-out-abcdefgh"},
		// Name slugifies to empty → SlugFromName's "app" fallback + id
		// fragment, so the slug is never empty.
		{"!!!", "01HJXVHGRGTQRX4P0G3Y8R6CK7", "app-3y8r6ck7"},
		{"", "0123456789", "app-23456789"},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.appID, func(t *testing.T) {
			got := applicationImageSlug(tc.name, tc.appID)
			assert.Equal(t, tc.want, got)
			assert.NotEmpty(t, got, "slug must never be empty")
		})
	}
}

// TestApplicationImageSlug_DistinctPerApp is the core regression: two
// apps from the same repo at the same commit must produce different
// image tags so one doesn't overwrite the other's image.
func TestApplicationImageSlug_DistinctPerApp(t *testing.T) {
	web := applicationImageSlug("web", "01HJXVHGRGTQRX4P0G3Y8R6CK7")
	api := applicationImageSlug("api", "01HJXVHGRGTQRX4P0G3Y8R6CK8")
	assert.NotEqual(t, web, api)
}

func TestParseGHASourceConfig_UnparseableRepo(t *testing.T) {
	_, err := parseGHASourceConfig(map[string]any{
		"repo":              "not-a-url",
		"source_control_id": "01k0pzy8ynwnz1j4ytd7eyncdp",
	})
	assert.ErrorContains(t, err, "owner/repo missing")
}

// --- DeployApplicationPayload.String redaction ---
//
// The asynq payload carries OverrideRegistryPassword which on the GHA
// path holds either a GHCR pull bearer (the new "token-exchange relay"
// flow) or a GitHub App installation token (fallback). Both are
// secrets. The custom Stringer redacts them so:
//
//   logger.Info().Stringer("payload", payload)   →  doesn't leak
//   fmt.Sprintf("%v",  payload)                  →  doesn't leak
//   fmt.Sprintf("%+v", payload)                  →  doesn't leak
//   fmt.Sprintf("%s",  payload)                  →  doesn't leak
//
// (Note: `Sprintf("%#v", payload)` still goes through reflection and
//  bypasses Stringer — but `%#v` is a debug-shape and shouldn't be
//  used in any code path that ships logs to a customer log sink. The
//  rest of the format verbs all hit String().)

func TestDeployApplicationPayload_StringRedaction(t *testing.T) {
	secret := "ghs_ThisIsThePullBearerWeDoNotWantToLeak_0123456789"
	p := DeployApplicationPayload{
		ApplicationID:                        "01ks_app",
		DeploymentID:                         "01ks_deploy",
		ServerID:                             "01ks_server",
		TeamID:                               "01ks_team",
		OverrideImage:                        "ghcr.io/example/repo:tag",
		OverrideRegistryURL:                  "ghcr.io",
		OverrideRegistryUsername:             "oauth2",
		OverrideRegistryPassword:             secret,
		OverrideRegistryPasswordMintedAtUnix: "1717000000",
	}

	// Direct call.
	s := p.String()
	assert.NotContains(t, s, secret, "String() must not contain the bearer")
	assert.Contains(t, s, "[REDACTED]")
	// Non-secret fields must still survive — the point is REDACTION,
	// not stripping. An operator looking at the log needs the
	// application + deployment IDs to find the right row.
	assert.Contains(t, s, "01ks_app")
	assert.Contains(t, s, "01ks_deploy")
	assert.Contains(t, s, "ghcr.io")

	// Format verbs that go through Stringer.
	for _, verb := range []string{"%v", "%s", "%+v"} {
		got := fmt.Sprintf(verb, p)
		assert.NotContainsf(t, got, secret, "Sprintf(%q) must not leak", verb)
	}
}

func TestDeployApplicationPayload_StringRedaction_NoPassword_NoChange(t *testing.T) {
	// When there's no bearer (e.g. non-GHA deploys) the field is
	// empty — redaction should leave the empty string as the empty
	// string, NOT print "[REDACTED]" for nothing.
	p := DeployApplicationPayload{
		ApplicationID: "01ks_app",
		DeploymentID:  "01ks_deploy",
	}
	assert.NotContains(t, p.String(), "[REDACTED]")
}

// --- ghcrBearerExpiredSoon ---
//
// Pre-flight age check on the workflow-stamped bearer. Fails fast
// with a clear "queue was stalled" message instead of letting
// docker pull report an opaque 401 mid-deploy. Pure function so
// the table below covers every interesting branch without standing
// up a deploy job.

func TestGHCRBearerExpiredSoon_Cases(t *testing.T) {
	now := time.Date(2026, 5, 28, 22, 30, 0, 0, time.UTC)
	cases := []struct {
		name            string
		mintedAt        string
		expectedExpired bool
		expectedSubstr  string
	}{
		{
			name:            "empty stamp (install-token fallback path)",
			mintedAt:        "",
			expectedExpired: false,
		},
		{
			name:            "zero stamp ignored same as empty",
			mintedAt:        "0",
			expectedExpired: false,
		},
		{
			name:            "garbage stamp ignored (defensive)",
			mintedAt:        "not-a-unix-timestamp",
			expectedExpired: false,
		},
		{
			name: "just-minted bearer is fresh",
			// 30 seconds ago — well within window.
			mintedAt:        fmt.Sprintf("%d", now.Add(-30*time.Second).Unix()),
			expectedExpired: false,
		},
		{
			name:            "right at 49 min — still fresh",
			mintedAt:        fmt.Sprintf("%d", now.Add(-49*time.Minute).Unix()),
			expectedExpired: false,
		},
		{
			name:            "51 min — expired",
			mintedAt:        fmt.Sprintf("%d", now.Add(-51*time.Minute).Unix()),
			expectedExpired: true,
			expectedSubstr:  "token age",
		},
		{
			name:            "2 hours old — expired",
			mintedAt:        fmt.Sprintf("%d", now.Add(-2*time.Hour).Unix()),
			expectedExpired: true,
			expectedSubstr:  "token age",
		},
		{
			name:            "30s in the future — tolerated (clock skew)",
			mintedAt:        fmt.Sprintf("%d", now.Add(30*time.Second).Unix()),
			expectedExpired: false,
		},
		{
			name:            "10 min in the future — rejected as clock pathology",
			mintedAt:        fmt.Sprintf("%d", now.Add(10*time.Minute).Unix()),
			expectedExpired: true,
			expectedSubstr:  "minted in the future",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, expired := ghcrBearerExpiredSoon(tc.mintedAt, now)
			assert.Equal(t, tc.expectedExpired, expired)
			if tc.expectedExpired {
				require.NotEmpty(t, reason)
				if tc.expectedSubstr != "" {
					assert.Contains(t, reason, tc.expectedSubstr)
				}
			} else {
				assert.Equal(t, "", reason)
			}
		})
	}
}

// --- DeployComposePayload.String redaction (mirror) ---

func TestDeployComposePayload_StringRedaction(t *testing.T) {
	secret := "ghs_ComposeBearerSecret_xxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	p := DeployComposePayload{
		ComposeID:                "01ks_compose",
		DeploymentID:             "01ks_deploy_compose",
		ServiceImages:            map[string]string{"web": "ghcr.io/x/y/web:t"},
		OverrideRegistryPassword: secret,
	}
	s := p.String()
	assert.NotContains(t, s, secret)
	assert.Contains(t, s, "[REDACTED]")
	assert.Contains(t, s, "01ks_compose")
	for _, verb := range []string{"%v", "%s", "%+v"} {
		assert.NotContainsf(t, fmt.Sprintf(verb, p), secret, "Sprintf(%q) must not leak", verb)
	}
}

// Belt-and-braces: scan the rendered Stringer output for anything that
// looks like a GitHub token prefix. If a future struct field gets
// added that should be redacted but isn't, this catches it.
func TestDeployApplicationPayload_StringerDoesNotLeakTokenPrefixes(t *testing.T) {
	p := DeployApplicationPayload{
		OverrideRegistryPassword:             "ghs_super_secret_pat_classic_format",
		OverrideRegistryPasswordMintedAtUnix: "1717000000",
	}
	s := p.String()
	for _, prefix := range []string{"ghs_", "ghp_", "github_pat_", "ghu_", "gho_"} {
		if strings.Contains(s, prefix) {
			t.Fatalf("payload Stringer leaked a token-looking prefix %q: %s", prefix, s)
		}
	}
}

// TestResolveAppDockerfilePath verifies the GHA bootstrap reads the custom
// Dockerfile path from the application's build_config (where the create/update
// flow stores it), falling back to the source_config-derived default. Guards
// the regression where a custom location like "docker/Dockerfile" was ignored
// and the rendered workflow always built the repo-root Dockerfile.
func TestResolveAppDockerfilePath(t *testing.T) {
	t.Run("build_config override wins", func(t *testing.T) {
		got := resolveAppDockerfilePath(dbtype.JSONMap{"dockerfile_path": "docker/Dockerfile"}, "Dockerfile")
		assert.Equal(t, "docker/Dockerfile", got)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		got := resolveAppDockerfilePath(dbtype.JSONMap{"dockerfile_path": "  build/Dockerfile  "}, "Dockerfile")
		assert.Equal(t, "build/Dockerfile", got)
	})

	t.Run("empty override keeps fallback", func(t *testing.T) {
		got := resolveAppDockerfilePath(dbtype.JSONMap{"dockerfile_path": "   "}, "Dockerfile")
		assert.Equal(t, "Dockerfile", got)
	})

	t.Run("missing key keeps fallback", func(t *testing.T) {
		got := resolveAppDockerfilePath(dbtype.JSONMap{"other": "x"}, "Dockerfile")
		assert.Equal(t, "Dockerfile", got)
	})

	t.Run("nil build_config keeps fallback", func(t *testing.T) {
		got := resolveAppDockerfilePath(nil, "Dockerfile")
		assert.Equal(t, "Dockerfile", got)
	})

	t.Run("non-string value keeps fallback", func(t *testing.T) {
		got := resolveAppDockerfilePath(dbtype.JSONMap{"dockerfile_path": 123}, "Dockerfile")
		assert.Equal(t, "Dockerfile", got)
	})
}

// TestPerAppGHANaming locks in the per-app namespacing that lets multiple
// applications share one repo without colliding on the deploy-token secret or
// workflow file (the cross-app deploy 401, #83).
func TestPerAppGHANaming(t *testing.T) {
	id := "01ktkvme8k7asd426djt0xg8r7"

	secret := appDeployTokenSecretName(id)
	assert.Equal(t, "LAUNCH_DEPLOY_TOKEN_01KTKVME8K7ASD426DJT0XG8R7", secret)
	// GitHub secret names: [A-Z0-9_], must not start with a digit.
	assert.Regexp(t, `^[A-Z][A-Z0-9_]*$`, secret)

	path := appWorkflowPath(id)
	assert.Equal(t, ".github/workflows/launch-deploy-01ktkvme8k7asd426djt0xg8r7.yml", path)

	// Two different apps get distinct secret + workflow names.
	other := "01zzzzzzzzzzzzzzzzzzzzzzzz"
	assert.NotEqual(t, secret, appDeployTokenSecretName(other))
	assert.NotEqual(t, path, appWorkflowPath(other))
}

// TestMintTokenIfNeeded_ForcePushesNewSecret guards the per-app secret
// self-heal: when the deploy-token secret name changes (migration to per-app
// namespacing), a token must be minted even though a hash already exists, so
// the new secret gets populated (otherwise the per-app workflow → empty secret
// → 401).
func TestMintTokenIfNeeded_ForcePushesNewSecret(t *testing.T) {
	j := &GHABootstrapWorkflowJob{Payload: GHABootstrapWorkflowPayload{}}
	existing := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	// force=false with an existing hash and no rotate → no mint.
	raw, hash, err := j.mintTokenIfNeeded(&existing, false)
	require.NoError(t, err)
	assert.Empty(t, raw)
	assert.Empty(t, hash)

	// force=true → mint a fresh token even though a hash exists.
	raw, hash, err = j.mintTokenIfNeeded(&existing, true)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, existing, hash)

	// No existing hash → mint regardless of force.
	raw, hash, err = j.mintTokenIfNeeded(nil, false)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)
	assert.NotEmpty(t, hash)
}
