package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	gitlabAPIURL  = "https://gitlab.com/api/v4"
	gitlabBaseURL = "https://gitlab.com"
)

// GitLabProvider implements the Provider interface for GitLab
type GitLabProvider struct {
	config        *ProviderConfig
	sourceControl *SourceControlData
	httpClient    *http.Client
}

// NewGitLabProvider creates a new GitLab provider
func NewGitLabProvider(config *ProviderConfig) *GitLabProvider {
	return &GitLabProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetSourceControl sets the source control context
func (p *GitLabProvider) SetSourceControl(sc *SourceControlData) {
	p.sourceControl = sc
}

// GetType returns the provider type
func (p *GitLabProvider) GetType() GitProviderType {
	return GitProviderGitLab
}

// GetInstallationURL returns the URL to install the GitLab integration
func (p *GitLabProvider) GetInstallationURL() (string, error) {
	if p.config.ClientID == "" {
		return "", ErrProviderNotConfigured
	}
	// GitLab uses OAuth for integration
	return fmt.Sprintf("%s/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=api",
		gitlabBaseURL, p.config.ClientID, ""), nil
}

// GetInstallation gets an installation by ID (for GitLab, this would be a connected account)
func (p *GitLabProvider) GetInstallation(ctx context.Context, installationID string) (*AppInstallationData, error) {
	// GitLab doesn't have the same installation concept as GitHub
	// This would typically return user/group information
	return &AppInstallationData{
		ID:           installationID,
		AccountID:    installationID,
		AccountLogin: "",
		AccountType:  "user",
	}, nil
}

// GetAllInstallations gets all installations (connected accounts)
func (p *GitLabProvider) GetAllInstallations(ctx context.Context) ([]AppInstallationData, error) {
	// For GitLab, we would return connected accounts
	return []AppInstallationData{}, nil
}

// GetInstallationRepositories gets repositories for an installation
func (p *GitLabProvider) GetInstallationRepositories(ctx context.Context, installationID string) ([]map[string]interface{}, error) {
	// This would use the access token to fetch projects
	return []map[string]interface{}{}, nil
}

// GetRepository gets a specific repository
func (p *GitLabProvider) GetRepository(ctx context.Context, installationID, owner, repo string) (map[string]interface{}, error) {
	return nil, ErrProviderNotConfigured
}

// ValidateWebhook validates a webhook signature
func (p *GitLabProvider) ValidateWebhook(payload []byte, signature string) bool {
	if p.config.WebhookSecret == "" {
		return false
	}

	// GitLab uses X-Gitlab-Token header for validation
	return signature == p.config.WebhookSecret
}

// GetCommitData extracts commit data from a webhook payload
func (p *GitLabProvider) GetCommitData(payload map[string]interface{}) *CommitData {
	return CommitDataFromGitLabPayload(payload)
}

// TestConnection tests the connection to GitLab
func (p *GitLabProvider) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", gitlabAPIURL+"/user", nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("connection test failed: %s", string(body))
	}

	return nil
}

// GetSSHURL returns the SSH URL for a repository
func (p *GitLabProvider) GetSSHURL(repo string) string {
	return fmt.Sprintf("git@gitlab.com:%s.git", repo)
}

// GetHTTPSURL returns the HTTPS URL for a repository
func (p *GitLabProvider) GetHTTPSURL(repo string) string {
	return fmt.Sprintf("https://gitlab.com/%s.git", repo)
}

// DeployKey deploys an SSH key to a repository
func (p *GitLabProvider) DeployKey(ctx context.Context, sourceControlID, title, repo, key string) error {
	return ErrProviderNotConfigured
}

// GetLastCommit gets the last commit for a repository and branch
func (p *GitLabProvider) GetLastCommit(ctx context.Context, sourceControlID, repo, branch string) (*CommitData, error) {
	return nil, ErrProviderNotConfigured
}

// GetInstallationToken returns the OAuth access token for GitLab
// GitLab uses OAuth tokens stored in source control data, not app installation tokens
func (p *GitLabProvider) GetInstallationToken(ctx context.Context, installationID string) (string, error) {
	if p.sourceControl == nil || p.sourceControl.ProviderData == nil {
		return "", ErrAuthenticationFailed
	}

	token, ok := p.sourceControl.ProviderData["access_token"].(string)
	if !ok || token == "" {
		return "", ErrAuthenticationFailed
	}

	return token, nil
}

// generateSignature generates a webhook signature for testing
func (p *GitLabProvider) generateSignature(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(p.config.WebhookSecret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// makeAuthenticatedRequest makes an authenticated request to the GitLab API
func (p *GitLabProvider) makeAuthenticatedRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// Add authentication headers based on available credentials
	if p.sourceControl != nil && p.sourceControl.ProviderData != nil {
		if token, ok := p.sourceControl.ProviderData["access_token"].(string); ok {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	return p.httpClient.Do(req)
}

// parseGitLabRepository parses a GitLab project response into a standard format
func parseGitLabRepository(project map[string]interface{}) map[string]interface{} {
	pathWithNamespace, _ := project["path_with_namespace"].(string)
	name, _ := project["name"].(string)
	webURL, _ := project["web_url"].(string)
	sshURLToRepo, _ := project["ssh_url_to_repo"].(string)
	defaultBranch, _ := project["default_branch"].(string)

	visibility, _ := project["visibility"].(string)
	isPublic := visibility == "public"

	return map[string]interface{}{
		"id":             project["id"],
		"name":           name,
		"full_name":      pathWithNamespace,
		"private":        !isPublic,
		"html_url":       webURL,
		"ssh_url":        sshURLToRepo,
		"default_branch": defaultBranch,
		"description":    project["description"],
	}
}

// fetchProjects fetches projects from GitLab API with pagination
func (p *GitLabProvider) fetchProjects(ctx context.Context, accessToken string) ([]map[string]interface{}, error) {
	var allProjects []map[string]interface{}
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("%s/projects?membership=true&per_page=%d&page=%d", gitlabAPIURL, perPage, page)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to fetch projects: %s", string(body))
		}

		var projects []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if len(projects) == 0 {
			break
		}

		for _, project := range projects {
			allProjects = append(allProjects, parseGitLabRepository(project))
		}

		page++
	}

	return allProjects, nil
}
