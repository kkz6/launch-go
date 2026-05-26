package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

// resolveAuthenticatedCloneURL turns a plain git repo URL into one the
// docker server can clone without a credential prompt. If
// source_control_id is set on the source_config, we look up the
// connected provider, decrypt its access_token from ProviderData, and
// inject it into the HTTPS URL.
//
// Returns the original URL untouched when there's no source_control_id,
// when the lookup fails, or when the URL isn't HTTPS (SSH clones need
// a separate deploy-key flow that lands in a follow-up). Errors are
// logged but never returned — the deploy job tolerates a missing
// source-control row by falling back to a public clone attempt.
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

	token := extractAccessToken(sc.ProviderData)
	if token == "" {
		d.Logger.Warn().
			Str("source_control_id", scID).
			Str("provider", string(sc.Provider)).
			Msg("source control has no access_token; falling back to unauthenticated clone")
		return originalRepo
	}

	authed, err := injectTokenIntoHTTPSURL(originalRepo, sc.Provider, token)
	if err != nil {
		d.Logger.Warn().Err(err).
			Str("provider", string(sc.Provider)).
			Msg("could not build authenticated clone URL; falling back to original")
		return originalRepo
	}
	return authed
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

// injectTokenIntoHTTPSURL prefixes the URL's host with provider-
// specific basic-auth credentials so `git clone <url>` succeeds with
// no extra config.
//
// GitHub:     https://oauth2:<token>@github.com/owner/repo.git
// GitLab:     https://oauth2:<token>@gitlab.com/owner/repo.git
// Bitbucket:  https://x-token-auth:<token>@bitbucket.org/owner/repo.git
//
// Only HTTPS URLs are rewritten — SSH URLs (`git@github.com:...`)
// need a deploy key on the server, which is a separate flow.
func injectTokenIntoHTTPSURL(repoURL string, provider gittypes.GitProviderType, token string) (string, error) {
	if !strings.HasPrefix(repoURL, "https://") {
		return "", fmt.Errorf("not an https URL (cannot inject token)")
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}

	// Provider-specific username (token-secret pattern). The colon
	// before the token is what tells git this is a basic-auth pair —
	// the token itself goes in the password slot, not the user slot,
	// otherwise GitHub's PATs don't authenticate.
	username := "oauth2"
	switch provider {
	case gittypes.GitProviderBitbucket:
		username = "x-token-auth"
	}
	u.User = url.UserPassword(username, token)
	return u.String(), nil
}
