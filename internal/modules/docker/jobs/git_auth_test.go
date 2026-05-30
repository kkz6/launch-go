package jobs

import (
	"strings"
	"testing"

	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

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

func TestExtractAccessToken_Present(t *testing.T) {
	jsonStr := `{"access_token":"ghp_secret","other":"value"}`
	got := extractAccessToken(&jsonStr)
	if got != "ghp_secret" {
		t.Errorf("got %q, want %q", got, "ghp_secret")
	}
}

func TestExtractAccessToken_AbsentOrInvalid(t *testing.T) {
	cases := []struct {
		name string
		in   *string
	}{
		{"nil", nil},
		{"empty", strPtrLocal("")},
		{"no field", strPtrLocal(`{"other":"x"}`)},
		{"malformed", strPtrLocal(`{"access_token`)},
		// Non-string access_token shouldn't panic — should return empty.
		{"wrong type", strPtrLocal(`{"access_token":123}`)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractAccessToken(c.in); got != "" {
				t.Errorf("got %q, want empty", got)
			}
		})
	}
}

func strPtrLocal(s string) *string { return &s }
