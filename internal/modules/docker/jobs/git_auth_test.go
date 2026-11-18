package jobs

import (
	"context"
	"strings"
	"testing"

	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

func TestQueuedPublicCloneRejectsAmbientSSHAndLocalRepositories(t *testing.T) {
	deps := &JobDeps{Deps: &pkgjobs.Deps{}}
	for _, repository := range []string{"ssh://github.com/org/private.git", "file:///srv/private", "/srv/private", "ext::helper", "https://token@github.com/org/private.git"} {
		_, _, err := deps.resolveCloneCredentials(context.Background(), "account-a", map[string]any{}, repository, nil, nil)
		if err == nil {
			t.Errorf("accepted private credential fallback %q", repository)
		}
	}
}

func TestInjectTokenIntoCloneURL_GitHubHTTPS(t *testing.T) {
	got, err := injectTokenIntoCloneURL(
		"https://github.com/acme/api.git",
		gittypes.GitProviderGitHub,
		"ghs_install_token",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// GitHub install tokens (the App flow) use `x-access-token` as the
	// username, per the GitHub docs. This is the form `docker login`,
	// `git clone`, and `gh auth` all bless.
	want := "https://x-access-token:ghs_install_token@github.com/acme/api.git"
	if got != want {
		t.Errorf("github URL = %q, want %q", got, want)
	}
}

func TestInjectTokenIntoCloneURL_GitHubSSHConverted(t *testing.T) {
	// The big behavioural change: SSH URLs no longer error — we convert
	// them to HTTPS first, then inject. The docker host doesn't carry
	// a deploy key and we don't want customers to manage one, so the
	// only sane fallback is HTTPS-with-token.
	got, err := injectTokenIntoCloneURL(
		"git@github.com:acme/api.git",
		gittypes.GitProviderGitHub,
		"ghs_install_token",
	)
	if err != nil {
		t.Fatalf("unexpected error converting ssh url: %v", err)
	}
	want := "https://x-access-token:ghs_install_token@github.com/acme/api.git"
	if got != want {
		t.Errorf("ssh→https URL = %q, want %q", got, want)
	}
}

func TestInjectTokenIntoCloneURL_GitLab(t *testing.T) {
	got, err := injectTokenIntoCloneURL(
		"https://gitlab.com/acme/api.git",
		gittypes.GitProviderGitLab,
		"glpat_secret",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// GitLab uses `oauth2:<token>` — distinct from GitHub's
	// `x-access-token`.
	if !strings.Contains(got, "oauth2:glpat_secret@gitlab.com") {
		t.Errorf("gitlab URL %q missing oauth2:<token> auth", got)
	}
}

func TestInjectTokenIntoCloneURL_Bitbucket(t *testing.T) {
	got, err := injectTokenIntoCloneURL(
		"https://bitbucket.org/acme/api.git",
		gittypes.GitProviderBitbucket,
		"bbpw_secret",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Bitbucket needs x-token-auth as the username, not oauth2 — the
	// service rejects oauth2 PATs since 2022.
	if !strings.Contains(got, "x-token-auth:bbpw_secret@bitbucket.org") {
		t.Errorf("bitbucket URL %q missing x-token-auth user", got)
	}
}

func TestInjectTokenIntoCloneURL_SSHSchemeConverted(t *testing.T) {
	// `ssh://` scheme form (rare but valid) — should also normalize.
	got, err := injectTokenIntoCloneURL(
		"ssh://git@github.com/acme/api.git",
		gittypes.GitProviderGitHub,
		"ghs_token",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "https://x-access-token:ghs_token@github.com/") {
		t.Errorf("ssh:// URL did not normalize to https with creds: %q", got)
	}
}

func TestInjectTokenIntoCloneURL_RejectsMalformed(t *testing.T) {
	// http:// would silently downgrade — refuse it so the caller falls
	// back to the original URL (and the customer sees a clear failure
	// rather than credentials over plaintext).
	_, err := injectTokenIntoCloneURL(
		"http://github.com/acme/api.git",
		gittypes.GitProviderGitHub,
		"ghs_token",
	)
	if err == nil {
		t.Fatal("expected error for plain http URL, got nil")
	}

	// scp-style with no path part — should error rather than produce
	// a half-formed URL.
	_, err = injectTokenIntoCloneURL(
		"git@github.com:",
		gittypes.GitProviderGitHub,
		"ghs_token",
	)
	if err == nil {
		t.Fatal("expected error for malformed scp URL, got nil")
	}
}

func TestGitHubCredentialRejectsForeignHostsAndEmbeddedCredentials(t *testing.T) {
	for _, repo := range []string{"https://evil.example/acme/api.git", "https://github.com.evil.example/acme/api.git", "https://github.com:443/acme/api.git", "https://username:password@github.com/acme/api.git", "https://github.com/acme/api.git?redirect=evil", "https://github.com/acme/api.git#fragment"} {
		if _, err := injectTokenIntoCloneURL(repo, gittypes.GitProviderGitHub, "private-token"); err == nil {
			t.Errorf("credential accepted unsafe repository %q", repo)
		}
	}
}
