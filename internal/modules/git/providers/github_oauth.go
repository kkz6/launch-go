package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GitHubUserToken struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int64  `json:"expires_in"`
	RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
	Error                 string `json:"error"`
}

func (p *GitHubProvider) AuthorizationURL(state, challenge string) (string, error) {
	if p.Config().ClientID == "" || p.Config().ClientSecret == "" || p.Config().CallbackURL == "" {
		return "", errors.New("GitHub user authorization is not configured")
	}
	q := url.Values{"client_id": {p.Config().ClientID}, "redirect_uri": {p.Config().CallbackURL}, "state": {state}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}, "prompt": {"select_account"}, "scope": {"offline_access"}}
	return p.BaseURL() + "/login/oauth/authorize?" + q.Encode(), nil
}

func (p *GitHubProvider) ExchangeUserCode(ctx context.Context, code, verifier string) (*GitHubUserToken, error) {
	return p.userToken(ctx, map[string]string{"client_id": p.Config().ClientID, "client_secret": p.Config().ClientSecret, "code": code, "code_verifier": verifier, "redirect_uri": p.Config().CallbackURL})
}

func (p *GitHubProvider) RefreshUserToken(ctx context.Context, refresh string) (*GitHubUserToken, error) {
	return p.userToken(ctx, map[string]string{"client_id": p.Config().ClientID, "client_secret": p.Config().ClientSecret, "grant_type": "refresh_token", "refresh_token": refresh})
}

func (p *GitHubProvider) userToken(ctx context.Context, body map[string]string) (*GitHubUserToken, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL()+"/login/oauth/access_token", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		failure := githubUserResponseError(resp)
		if resp.StatusCode == http.StatusBadRequest {
			var result GitHubUserToken
			if json.NewDecoder(resp.Body).Decode(&result) == nil && result.Error == "bad_refresh_token" {
				failure.AuthorizationInvalid = true
			}
		}
		return nil, failure
	}
	var token GitHubUserToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}
	if token.Error == "bad_refresh_token" {
		return nil, &GitHubUserAccessError{StatusCode: resp.StatusCode, AuthorizationInvalid: true}
	}
	if token.Error != "" || token.AccessToken == "" || token.ExpiresIn <= 0 || token.RefreshToken == "" || token.RefreshTokenExpiresIn <= 0 {
		return nil, errors.New("GitHub authorization requires reconnect")
	}
	return &token, nil
}

func decodeGitHubJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	return decoder.Decode(target)
}

func (p *GitHubProvider) GetAuthorizedUser(ctx context.Context, token string) (map[string]any, error) {
	return p.userObject(ctx, token, "/user")
}

func (p *GitHubProvider) userObject(ctx context.Context, token, path string) (map[string]any, error) {
	resp, err := p.DoRaw(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, githubUserResponseError(resp)
	}
	var result map[string]any
	err = decodeGitHubJSON(resp, &result)
	return result, err
}

func (p *GitHubProvider) GetUserInstallations(ctx context.Context, token string) ([]map[string]any, error) {
	return p.userPages(ctx, token, "/user/installations?per_page=100", "installations")
}

func (p *GitHubProvider) GetUserInstallationRepositories(ctx context.Context, token, installationID string) ([]map[string]any, error) {
	if !decimalID(installationID) {
		return nil, ErrInstallationNotFound
	}
	repositories, err := p.userPages(ctx, token, "/user/installations/"+installationID+"/repositories?per_page=100", "repositories")
	if err != nil {
		return nil, err
	}
	return availableUserRepositories(repositories), nil
}

func (p *GitHubProvider) UserRepositoryAdmin(ctx context.Context, token, fullName, login string) (bool, error) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false, ErrPermissionDenied
	}
	resp, err := p.DoRaw(ctx, http.MethodGet, "/repos/"+url.PathEscape(parts[0])+"/"+url.PathEscape(parts[1])+"/collaborators/"+url.PathEscape(login)+"/permission", token, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	// An installation can list repositories that this user cannot administer.
	// Deny that repository without preventing review of unrelated candidates.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnavailableForLegalReasons {
		return false, nil
	}
	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") != "0" && resp.Header.Get("Retry-After") == "" {
		var denial struct{ Message string }
		if json.NewDecoder(resp.Body).Decode(&denial) == nil {
			message := strings.ToLower(denial.Message)
			if !strings.Contains(message, "rate limit") && !strings.Contains(message, "abuse") {
				return false, nil
			}
		}
	}
	if resp.StatusCode != http.StatusOK {
		failure := githubUserResponseError(resp)
		// Ordinary denials returned above. A remaining 403 represents a
		// rate limit or an unreadable response, not evidence of revocation.
		failure.Retryable = failure.Retryable || resp.StatusCode == http.StatusForbidden
		return false, failure
	}
	var result struct{ Permission string }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	return result.Permission == "admin", nil
}

func (p *GitHubProvider) userPages(ctx context.Context, token, path, key string) ([]map[string]any, error) {
	var all []map[string]any
	seen := map[string]bool{}
	for path != "" {
		if seen[path] || !strings.HasPrefix(path, "/user/") {
			return nil, errors.New("invalid GitHub pagination")
		}
		seen[path] = true
		resp, err := p.DoRaw(ctx, http.MethodGet, path, token, nil)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			failure := githubUserResponseError(resp)
			_ = resp.Body.Close()
			return nil, failure
		}
		next := ParseLinkHeader(resp.Header.Get("Link"))
		var page map[string]any
		if err := decodeGitHubJSON(resp, &page); err != nil {
			return nil, err
		}
		items, ok := page[key].([]any)
		if !ok {
			return nil, errors.New("invalid GitHub repository response")
		}
		for _, item := range items {
			value, ok := item.(map[string]any)
			if !ok {
				return nil, errors.New("invalid GitHub repository item")
			}
			all = append(all, value)
		}
		path = next
	}
	return all, nil
}

func decimalID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
