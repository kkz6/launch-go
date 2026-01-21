package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/cryptoutil"
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
	// Use constant-time comparison to prevent timing attacks
	return cryptoutil.SecureCompare(signature, p.config.WebhookSecret)
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
	if p.sourceControl == nil {
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
	url := fmt.Sprintf("%s/projects/%s/deployments", gitlabAPIURL, projectID)
	body := map[string]interface{}{
		"ref":         info.Branch,
		"environment": environment,
		"status":      DeploymentStatusInProgress.GitLabStatus(),
	}
	if info.GitHash != "" {
		body["sha"] = info.GitHash
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, jsonReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, nil // Don't fail if deployment creation fails
	}

	var deploymentResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&deploymentResp); err != nil {
		return nil, err
	}

	deploymentID := ""
	if idFloat, ok := deploymentResp["id"].(float64); ok {
		deploymentID = fmt.Sprintf("%.0f", idFloat)
	}

	// Remove user from data
	delete(deploymentResp, "user")

	return &DeploymentResult{
		ID:   deploymentID,
		Data: deploymentResp,
	}, nil
}

// UpdateDeploymentStatus updates the status of a deployment on GitLab
func (p *GitLabProvider) UpdateDeploymentStatus(ctx context.Context, info *DeploymentInfo, vcsData map[string]interface{}, status DeploymentStatus) error {
	if p.sourceControl == nil {
		return nil
	}

	deploymentID, ok := vcsData["id"].(float64)
	if !ok {
		// Try string
		idStr, strOK := vcsData["id"].(string)
		if !strOK {
			return nil
		}
		if _, err := fmt.Sscanf(idStr, "%f", &deploymentID); err != nil {
			return nil
		}
	}

	projectID := info.ProjectID
	if projectID == "" {
		return nil
	}

	token, err := p.GetInstallationToken(ctx, "")
	if err != nil {
		return nil
	}

	url := fmt.Sprintf("%s/projects/%s/deployments/%.0f", gitlabAPIURL, projectID, deploymentID)
	body := map[string]interface{}{
		"status": status.GitLabStatus(),
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, jsonReader(bodyBytes))
	if err != nil {
		return nil
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	return nil
}

// jsonReader creates an io.Reader from bytes
func jsonReader(data []byte) io.Reader {
	return &byteReader{data: data}
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
