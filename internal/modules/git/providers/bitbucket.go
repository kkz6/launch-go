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
	bitbucketAPIURL  = "https://api.bitbucket.org/2.0"
	bitbucketBaseURL = "https://bitbucket.org"
)

// BitbucketProvider implements the Provider interface for Bitbucket
type BitbucketProvider struct {
	config        *ProviderConfig
	sourceControl *SourceControlData
	httpClient    *http.Client
}

// NewBitbucketProvider creates a new Bitbucket provider
func NewBitbucketProvider(config *ProviderConfig) *BitbucketProvider {
	return &BitbucketProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetSourceControl sets the source control context
func (p *BitbucketProvider) SetSourceControl(sc *SourceControlData) {
	p.sourceControl = sc
}

// GetType returns the provider type
func (p *BitbucketProvider) GetType() GitProviderType {
	return GitProviderBitbucket
}

// GetInstallationURL returns the URL to install the Bitbucket integration
func (p *BitbucketProvider) GetInstallationURL() (string, error) {
	if p.config.ClientID == "" {
		return "", ErrProviderNotConfigured
	}
	// Bitbucket uses OAuth for integration
	return fmt.Sprintf("%s/site/oauth2/authorize?client_id=%s&response_type=code",
		bitbucketBaseURL, p.config.ClientID), nil
}

// GetInstallation gets an installation by ID
func (p *BitbucketProvider) GetInstallation(ctx context.Context, installationID string) (*AppInstallationData, error) {
	// Bitbucket doesn't have the same installation concept as GitHub
	return &AppInstallationData{
		ID:           installationID,
		AccountID:    installationID,
		AccountLogin: "",
		AccountType:  "user",
	}, nil
}

// GetAllInstallations gets all installations (connected accounts)
func (p *BitbucketProvider) GetAllInstallations(ctx context.Context) ([]AppInstallationData, error) {
	return []AppInstallationData{}, nil
}

// GetInstallationRepositories gets repositories for an installation
func (p *BitbucketProvider) GetInstallationRepositories(ctx context.Context, installationID string) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

// GetRepository gets a specific repository
func (p *BitbucketProvider) GetRepository(ctx context.Context, installationID, owner, repo string) (map[string]interface{}, error) {
	return nil, ErrProviderNotConfigured
}

// ValidateWebhook validates a webhook signature
func (p *BitbucketProvider) ValidateWebhook(payload []byte, signature string) bool {
	if p.config.WebhookSecret == "" {
		return false
	}

	// Bitbucket uses X-Hook-UUID for identification, but we can use HMAC for validation
	mac := hmac.New(sha256.New, []byte(p.config.WebhookSecret))
	mac.Write(payload)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// GetCommitData extracts commit data from a webhook payload
func (p *BitbucketProvider) GetCommitData(payload map[string]interface{}) *CommitData {
	return CommitDataFromBitbucketPayload(payload)
}

// TestConnection tests the connection to Bitbucket
func (p *BitbucketProvider) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", bitbucketAPIURL+"/user", nil)
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
func (p *BitbucketProvider) GetSSHURL(repo string) string {
	return fmt.Sprintf("git@bitbucket.org:%s.git", repo)
}

// GetHTTPSURL returns the HTTPS URL for a repository
func (p *BitbucketProvider) GetHTTPSURL(repo string) string {
	return fmt.Sprintf("https://bitbucket.org/%s.git", repo)
}

// DeployKey deploys an SSH key to a repository
func (p *BitbucketProvider) DeployKey(ctx context.Context, sourceControlID, title, repo, key string) error {
	return ErrProviderNotConfigured
}

// GetLastCommit gets the last commit for a repository and branch
func (p *BitbucketProvider) GetLastCommit(ctx context.Context, sourceControlID, repo, branch string) (*CommitData, error) {
	return nil, ErrProviderNotConfigured
}

// GetInstallationToken returns the OAuth access token for Bitbucket
// Bitbucket uses OAuth tokens stored in source control data, not app installation tokens
func (p *BitbucketProvider) GetInstallationToken(ctx context.Context, installationID string) (string, error) {
	if p.sourceControl == nil || p.sourceControl.ProviderData == nil {
		return "", ErrAuthenticationFailed
	}

	token, ok := p.sourceControl.ProviderData["access_token"].(string)
	if !ok || token == "" {
		return "", ErrAuthenticationFailed
	}

	return token, nil
}

// makeAuthenticatedRequest makes an authenticated request to the Bitbucket API
func (p *BitbucketProvider) makeAuthenticatedRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
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

// CreateDeployment creates a deployment on Bitbucket
// Bitbucket doesn't have the same deployment API as GitHub/GitLab, so this is a no-op
func (p *BitbucketProvider) CreateDeployment(ctx context.Context, info *DeploymentInfo) (*DeploymentResult, error) {
	// Bitbucket doesn't support deployment status in the same way
	return nil, nil
}

// UpdateDeploymentStatus updates the status of a deployment on Bitbucket
// Bitbucket doesn't have the same deployment API as GitHub/GitLab, so this is a no-op
func (p *BitbucketProvider) UpdateDeploymentStatus(ctx context.Context, info *DeploymentInfo, vcsData map[string]interface{}, status DeploymentStatus) error {
	// Bitbucket doesn't support deployment status in the same way
	return nil
}

// parseBitbucketRepository parses a Bitbucket repository response into a standard format
func parseBitbucketRepository(repo map[string]interface{}) map[string]interface{} {
	fullName, _ := repo["full_name"].(string)
	name, _ := repo["name"].(string)

	var htmlURL, sshURL string
	if links, ok := repo["links"].(map[string]interface{}); ok {
		if html, ok := links["html"].(map[string]interface{}); ok {
			htmlURL, _ = html["href"].(string)
		}
		if cloneLinks, ok := links["clone"].([]interface{}); ok {
			for _, link := range cloneLinks {
				if linkMap, ok := link.(map[string]interface{}); ok {
					if name, ok := linkMap["name"].(string); ok && name == "ssh" {
						sshURL, _ = linkMap["href"].(string)
					}
				}
			}
		}
	}

	isPrivate, _ := repo["is_private"].(bool)

	mainbranch, _ := repo["mainbranch"].(map[string]interface{})
	defaultBranch, _ := mainbranch["name"].(string)
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	return map[string]interface{}{
		"id":             repo["uuid"],
		"name":           name,
		"full_name":      fullName,
		"private":        isPrivate,
		"html_url":       htmlURL,
		"ssh_url":        sshURL,
		"default_branch": defaultBranch,
		"description":    repo["description"],
	}
}

// fetchRepositories fetches repositories from Bitbucket API with pagination
func (p *BitbucketProvider) fetchRepositories(ctx context.Context, accessToken string, workspace string) ([]map[string]interface{}, error) {
	var allRepos []map[string]interface{}
	url := fmt.Sprintf("%s/repositories/%s", bitbucketAPIURL, workspace)

	for url != "" {
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
			return nil, fmt.Errorf("failed to fetch repositories: %s", string(body))
		}

		var result struct {
			Values []map[string]interface{} `json:"values"`
			Next   string                   `json:"next"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, repo := range result.Values {
			allRepos = append(allRepos, parseBitbucketRepository(repo))
		}

		url = result.Next
	}

	return allRepos, nil
}
