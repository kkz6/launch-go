package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

// resolveAuthenticatedCloneURL turns a plain git repo URL into one the
// docker server can clone without a credential prompt.
//
// Two paths depending on the source-control type:
//
//  1. GitHub App installation (the modern flow) — there's no
//     long-lived access_token; we mint a fresh ~1h installation
//     token via the JWT-signed `/app/installations/{id}/access_tokens`
//     endpoint, then build https://x-access-token:<token>@github.com/...
//
//  2. OAuth/PAT-stored access_token on ProviderData (legacy / GitLab
//     / Bitbucket flows) — use the token verbatim with the
//     provider-specific username convention.
//
// Both paths convert the stored URL from SSH form
// (`git@github.com:owner/repo.git`) to authenticated HTTPS — the
// docker host doesn't have an SSH deploy key by default and we
// don't want to make customers manage one.
//
// Returns the original URL untouched when there's no source_control_id,
// when the lookup fails, or when both token paths come up empty.
// Errors are logged but never returned — the deploy job tolerates
// a missing source-control row by falling back to the original URL
// (clone will fail with a clear "Permission denied (publickey)"
// message rather than this function swallowing it).
func (d *JobDeps) resolveAuthenticatedCloneURL(
	ctx context.Context, sourceConfig map[string]any, originalRepo string,
) string {
	if originalRepo == "" {
		return originalRepo
	}
	scIDAny, ok := sourceConfig["source_control_id"]
	if !ok {
		return originalRepo
	}
	scID, ok := scIDAny.(string)
	if !ok || scID == "" {
		return originalRepo
	}

	// The git module exposes a sql-backed FindByID; we don't need an
	// interface for this one read so call it directly.
	var sc gitmodels.SourceControl
	if err := d.DB.WithContext(ctx).Where("id = ?", scID).First(&sc).Error; err != nil {
		d.Logger.Warn().Err(err).
			Str("source_control_id", scID).
			Msg("source control not found; falling back to unauthenticated clone")
		return originalRepo
	}

	// Path 1: GitHub App installation → mint fresh install token.
	// Detected by provider=github + non-empty installation_id +
	// the git providers factory being wired (it is in normal runs;
	// the slice-C webhook test rig is the only consumer that runs
	// with GitProviders=nil and that path doesn't hit this code).
	if sc.Provider == gittypes.GitProviderGitHub &&
		sc.InstallationID != nil && *sc.InstallationID != "" &&
		d.GitProviders != nil {
		token, err := d.mintGitHubInstallationToken(ctx, *sc.InstallationID)
		if err != nil {
			d.Logger.Warn().Err(err).
				Str("source_control_id", scID).
				Str("installation_id", *sc.InstallationID).
				Msg("could not mint GitHub installation token; falling back to stored access_token")
			// Don't return yet — let path 2 try.
		} else if token != "" {
			authed, err := injectTokenIntoCloneURL(originalRepo, sc.Provider, token)
			if err != nil {
				d.Logger.Warn().Err(err).
					Str("provider", string(sc.Provider)).
					Msg("install-token URL injection failed; falling back to original")
				return originalRepo
			}
			return authed
		}
	}

	// Path 2: stored access_token (OAuth flow / non-App providers).
	token := extractAccessToken(sc.ProviderData)
	if token == "" {
		d.Logger.Warn().
			Str("source_control_id", scID).
			Str("provider", string(sc.Provider)).
			Msg("source control has neither installation_id nor access_token; falling back to unauthenticated clone")
		return originalRepo
	}

	authed, err := injectTokenIntoCloneURL(originalRepo, sc.Provider, token)
	if err != nil {
		d.Logger.Warn().Err(err).
			Str("provider", string(sc.Provider)).
			Msg("could not build authenticated clone URL; falling back to original")
		return originalRepo
	}
	return authed
}

// mintGitHubInstallationToken asks the GitHub provider for a fresh
// installation token. Same code path the GHA bootstrap job uses,
// just lifted into a small helper so the server-build clone flow
// shares the JWT-signing + caching behaviour rather than rolling
// its own.
func (d *JobDeps) mintGitHubInstallationToken(ctx context.Context, installationID string) (string, error) {
	provider, err := d.GitProviders.GetProvider(gitproviders.GitProviderType(gittypes.GitProviderGitHub))
	if err != nil {
		return "", fmt.Errorf("resolve github provider: %w", err)
	}
	gh, ok := provider.(*gitproviders.GitHubProvider)
	if !ok {
		return "", errors.New("provider for github is not a *GitHubProvider")
	}
	return gh.GetInstallationToken(ctx, installationID)
}

// extractAccessToken pulls the access_token field out of the JSON-
// encoded ProviderData blob the git module stores on source_controls.
// Returns "" on any decoding failure — caller falls back to public.
func extractAccessToken(providerData *string) string {
	if providerData == nil || *providerData == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(*providerData), &m); err != nil {
		return ""
	}
	if t, ok := m["access_token"].(string); ok {
		return t
	}
	return ""
}

// injectTokenIntoCloneURL prefixes the URL's host with provider-
// specific basic-auth credentials so `git clone <url>` succeeds with
// no extra config.
//
// GitHub:     https://x-access-token:<token>@github.com/owner/repo.git
// GitLab:     https://oauth2:<token>@gitlab.com/owner/repo.git
// Bitbucket:  https://x-token-auth:<token>@bitbucket.org/owner/repo.git
//
// SSH-form URLs (`git@github.com:owner/repo.git`) are converted to
// HTTPS first — the docker host doesn't carry a deploy key and we
// don't want customers to manage one. Plain `https://...` URLs are
// rewritten in place.
//
// Anything else (custom schemes, malformed URLs) errors out and the
// caller falls back to the original URL.
func injectTokenIntoCloneURL(repoURL string, provider gittypes.GitProviderType, token string) (string, error) {
	httpsURL, err := normalizeToHTTPS(repoURL)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(httpsURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}

	// Provider-specific username (token-secret pattern). The colon
	// before the token is what tells git this is a basic-auth pair —
	// the token itself goes in the password slot, not the user slot,
	// otherwise GitHub's install tokens / PATs don't authenticate.
	//
	// GitHub install tokens are documented to use `x-access-token` as
	// the username; `oauth2` also works for classic PATs but the App
	// flow is the path we exercise in prod, so default to the form
	// GitHub blesses.
	username := "x-access-token"
	switch provider {
	case gittypes.GitProviderGitLab:
		username = "oauth2"
	case gittypes.GitProviderBitbucket:
		username = "x-token-auth"
	}
	u.User = url.UserPassword(username, token)
	return u.String(), nil
}

// normalizeToHTTPS converts the SSH form `git@host:owner/repo.git`
// (which git treats as scp-style, not a real URL) into the matching
// `https://host/owner/repo.git`. Pass-through for URLs that already
// start with https://. Anything else is rejected — http:// in
// particular would silently downgrade credentials so we don't try
// to accommodate it.
func normalizeToHTTPS(repoURL string) (string, error) {
	if strings.HasPrefix(repoURL, "https://") {
		return repoURL, nil
	}
	// scp-style: `git@github.com:owner/repo.git` — note the colon
	// after the host, NOT a slash. url.Parse mis-handles these, so
	// we split manually.
	if strings.HasPrefix(repoURL, "git@") {
		rest := strings.TrimPrefix(repoURL, "git@")
		colon := strings.Index(rest, ":")
		if colon <= 0 || colon == len(rest)-1 {
			return "", fmt.Errorf("malformed ssh url: %q", repoURL)
		}
		host := rest[:colon]
		path := rest[colon+1:]
		return "https://" + host + "/" + path, nil
	}
	// ssh:// scheme — same conversion, just done through url.Parse.
	if strings.HasPrefix(repoURL, "ssh://") {
		u, err := url.Parse(repoURL)
		if err != nil {
			return "", fmt.Errorf("parse ssh url: %w", err)
		}
		path := strings.TrimPrefix(u.Path, "/")
		return "https://" + u.Host + "/" + path, nil
	}
	return "", fmt.Errorf("unsupported url scheme (need https:// or ssh): %q", repoURL)
}
