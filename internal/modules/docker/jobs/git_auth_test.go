package jobs

import (
	"strings"
	"testing"

	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

func TestInjectTokenIntoHTTPSURL_GitHub(t *testing.T) {
	got, err := injectTokenIntoHTTPSURL(
		"https://github.com/acme/api.git",
		gittypes.GitProviderGitHub,
		"ghp_secret",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://oauth2:ghp_secret@github.com/acme/api.git"
	if got != want {
		t.Errorf("github URL = %q, want %q", got, want)
	}
}

func TestInjectTokenIntoHTTPSURL_GitLab(t *testing.T) {
	got, err := injectTokenIntoHTTPSURL(
		"https://gitlab.com/acme/api.git",
		gittypes.GitProviderGitLab,
		"glpat_secret",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// GitLab uses the same `oauth2:<token>` username as GitHub.
	if !strings.Contains(got, "oauth2:glpat_secret@gitlab.com") {
		t.Errorf("gitlab URL %q missing oauth2:<token> auth", got)
	}
}

func TestInjectTokenIntoHTTPSURL_Bitbucket(t *testing.T) {
	got, err := injectTokenIntoHTTPSURL(
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

func TestInjectTokenIntoHTTPSURL_SSHRejected(t *testing.T) {
	// SSH URLs need a separate deploy-key flow. We must not silently
	// pretend a token-prefixed SSH URL is meaningful — git would
	// reject it and the user gets a confusing failure.
	_, err := injectTokenIntoHTTPSURL(
		"git@github.com:acme/api.git",
		gittypes.GitProviderGitHub,
		"ghp_secret",
	)
	if err == nil {
		t.Fatal("expected error for SSH URL, got nil")
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
