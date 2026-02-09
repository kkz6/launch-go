package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	gitlabAPIURL  = "https://gitlab.com/api/v4"
	gitlabBaseURL = "https://gitlab.com"
)

// GitLabProvider implements the Provider interface for GitLab
type GitLabProvider struct {
	*BaseGitProvider
}

// NewGitLabProvider creates a new GitLab provider
func NewGitLabProvider(config *ProviderConfig) *GitLabProvider {
	base := NewBaseGitProvider(
		config,
		WithProviderType(GitProviderGitLab),
		WithBaseURL(gitlabBaseURL),
		WithAPIURL(gitlabAPIURL),
	)

	return &GitLabProvider{
		BaseGitProvider: base,
	}
}

// GetInstallationURL returns the URL to install the GitLab integration
func (p *GitLabProvider) GetInstallationURL() (string, error) {
	if p.Config().ClientID == "" {
		return "", ErrProviderNotConfigured
	}
	// GitLab uses OAuth for integration
	return fmt.Sprintf("%s/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=api",
		p.BaseURL(), p.Config().ClientID, ""), nil
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
	// GitLab uses X-Gitlab-Token header for validation
	// Use constant-time comparison to prevent timing attacks
	return p.VerifyTokenSignature(signature)
}

// GetCommitData extracts commit data from a webhook payload
func (p *GitLabProvider) GetCommitData(payload map[string]interface{}) *CommitData {
	return CommitDataFromGitLabPayload(payload)
}

// TestConnection tests the connection to GitLab
func (p *GitLabProvider) TestConnection(ctx context.Context) error {
	token, err := p.GetOAuthToken()
	if err != nil {
		// If no token, just test that the API is reachable
		resp, reqErr := p.DoRaw(ctx, http.MethodGet, "/user", "", nil)
		if reqErr != nil {
			return reqErr
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			_, _ = io.ReadAll(resp.Body)
			return fmt.Errorf("connection test failed: status %d", resp.StatusCode)
		}
		return nil
	}

	resp, err := p.DoRaw(ctx, http.MethodGet, "/user", token, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		return fmt.Errorf("connection test failed: status %d", resp.StatusCode)
	}

	return nil
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
	return p.GetOAuthToken()
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

// CreateDeployment creates a deployment on GitLab
func (p *GitLabProvider) CreateDeployment(ctx context.Context, info *DeploymentInfo) (*DeploymentResult, error) {
	if p.SourceControl() == nil {
		return nil, nil
	}

	token, err := p.GetInstallationToken(ctx, "")
	if err != nil {
		return nil, nil // Don't fail deployment if we can't get token
	}

	projectID := info.ProjectID
	if projectID == "" {
		return nil, nil // Need project ID for GitLab
	}

	environment := info.Environment
	if environment == "" {
		environment = "production"
	}

	// Create deployment
	path := fmt.Sprintf("/projects/%s/deployments", projectID)
	body := map[string]interface{}{
		"ref":         info.Branch,
		"environment": environment,
		"status":      DeploymentStatusInProgress.GitLabStatus(),
	}
	if info.GitHash != "" {
		body["sha"] = info.GitHash
	}

	resp, err := p.DoRaw(ctx, http.MethodPost, path, token, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, nil // Don't fail if deployment creation fails
	}

	var deploymentResp map[string]interface{}
	if err := DecodeJSON(resp, &deploymentResp); err != nil {
		return nil, err
	}

	deploymentID := ExtractFloatID(deploymentResp, "id")

	// Remove user from data
	delete(deploymentResp, "user")

	return &DeploymentResult{
		ID:   deploymentID,
		Data: deploymentResp,
	}, nil
}

// UpdateDeploymentStatus updates the status of a deployment on GitLab
func (p *GitLabProvider) UpdateDeploymentStatus(ctx context.Context, info *DeploymentInfo, vcsData map[string]interface{}, status DeploymentStatus) error {
	if p.SourceControl() == nil {
		return nil
	}

	deploymentID := ExtractFloatID(vcsData, "id")
	if deploymentID == "" {
		return nil
	}

	projectID := info.ProjectID
	if projectID == "" {
		return nil
	}

	token, err := p.GetInstallationToken(ctx, "")
	if err != nil {
		return nil
	}

	path := fmt.Sprintf("/projects/%s/deployments/%s", projectID, deploymentID)
	body := map[string]interface{}{
		"status": status.GitLabStatus(),
	}

	resp, err := p.DoRaw(ctx, http.MethodPut, path, token, body)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	return nil
}

// FetchProjects fetches all projects accessible to the authenticated user
func (p *GitLabProvider) FetchProjects(ctx context.Context) ([]map[string]interface{}, error) {
	token, err := p.GetOAuthToken()
	if err != nil {
		return nil, err
	}

	items, err := p.FetchAllPages(
		ctx,
		"/projects?membership=true&per_page=100",
		token,
		func(response map[string]interface{}) ([]map[string]interface{}, error) {
			// GitLab returns an array directly
			return nil, nil
		},
		func(resp *http.Response, body map[string]interface{}) string {
			// GitLab uses Link header for pagination
			linkHeader := resp.Header.Get("Link")
			if linkHeader == "" {
				return ""
			}

			// Parse GitLab's Link header format
			links := strings.Split(linkHeader, ",")
			for _, link := range links {
				parts := strings.Split(link, ";")
				if len(parts) != 2 {
					continue
				}
				if strings.TrimSpace(parts[1]) == `rel="next"` {
					url := strings.TrimSpace(parts[0])
					// Extract path from full URL
					url = strings.Trim(url, "<>")
					if strings.HasPrefix(url, gitlabAPIURL) {
						return strings.TrimPrefix(url, gitlabAPIURL)
					}
					return url
				}
			}
			return ""
		},
	)
	if err != nil {
		return nil, err
	}

	// Parse each project
	var projects []map[string]interface{}
	for _, item := range items {
		projects = append(projects, parseGitLabRepository(item))
	}

	return projects, nil
}
