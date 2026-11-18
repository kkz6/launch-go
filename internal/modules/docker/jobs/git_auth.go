package jobs

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/gitaccess"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

// resolveCloneCredentials returns a canonical URL and a credential file. The
// credential never enters task scripts, clone arguments or queue payloads.
func (d *JobDeps) resolveCloneCredentials(ctx context.Context, teamID string, sourceConfig map[string]any, originalRepo string, expected *access.Snapshot, expectedDependencies []access.Snapshot) (string, string, error) {
	resolved, err := gitaccess.Resolve(ctx, d.DB, d.GitProviders, teamID, sourceConfig, access.Clone, expected, true)
	if err != nil {
		return "", "", err
	}
	if resolved == nil {
		if originalRepo == "" {
			return "", "", nil
		}
		if err := gitaccess.ValidatePublicRepositoryURL(originalRepo); err != nil {
			return "", "", err
		}
		return originalRepo, "", nil
	}
	provider, err := d.GitProviders.GetProvider(gitproviders.GitProviderType(gittypes.GitProviderGitHub))
	if err != nil {
		return "", "", err
	}
	gh, ok := provider.(*gitproviders.GitHubProvider)
	if !ok {
		return "", "", errors.New("GitHub provider unavailable")
	}
	token, err := gh.GetInstallationToken(resolved.Context(ctx), resolved.InstallationID)
	if err != nil {
		return "", "", err
	}
	credential, err := injectTokenIntoCloneURL(resolved.CloneURL, gittypes.GitProviderGitHub, token)
	if err != nil {
		return "", "", err
	}
	credentials := credential + "\n" + strings.TrimSuffix(credential, ".git") + "\n"
	dependencies, err := gitaccess.ResolveDependencies(ctx, d.DB, d.GitProviders, teamID, sourceConfig, expectedDependencies, true)
	if err != nil {
		return "", "", err
	}
	for _, dependency := range dependencies {
		token, err := gh.GetInstallationToken(dependency.Context(ctx), dependency.InstallationID)
		if err != nil {
			return "", "", err
		}
		credential, err := injectTokenIntoCloneURL(dependency.CloneURL, gittypes.GitProviderGitHub, token)
		if err != nil {
			return "", "", err
		}
		credentials += credential + "\n" + strings.TrimSuffix(credential, ".git") + "\n"
	}
	return resolved.CloneURL, credentials, nil
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

	expectedHost := map[gittypes.GitProviderType]string{gittypes.GitProviderGitHub: "github.com", gittypes.GitProviderGitLab: "gitlab.com", gittypes.GitProviderBitbucket: "bitbucket.org"}[provider]
	if expectedHost == "" || u.Scheme != "https" || u.Host != expectedHost || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("repository host does not match credential provider")
	}
	if len(strings.Split(strings.Trim(u.Path, "/"), "/")) < 2 {
		return "", errors.New("invalid repository path")
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
