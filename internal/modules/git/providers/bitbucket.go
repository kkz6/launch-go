package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const (
	bitbucketAPIURL  = "https://api.bitbucket.org/2.0"
	bitbucketBaseURL = "https://bitbucket.org"
)

// BitbucketProvider implements the Provider interface for Bitbucket
type BitbucketProvider struct {
	*BaseGitProvider
}

// NewBitbucketProvider creates a new Bitbucket provider
func NewBitbucketProvider(config *ProviderConfig) *BitbucketProvider {
	base := NewBaseGitProvider(
		config,
		WithProviderType(GitProviderBitbucket),
		WithBaseURL(bitbucketBaseURL),
		WithAPIURL(bitbucketAPIURL),
	)

	return &BitbucketProvider{
		BaseGitProvider: base,
	}
}

// GetInstallationURL returns the URL to install the Bitbucket integration
func (p *BitbucketProvider) GetInstallationURL() (string, error) {
	if p.Config().ClientID == "" {
		return "", ErrProviderNotConfigured
	}
	// Bitbucket uses OAuth for integration
	return fmt.Sprintf("%s/site/oauth2/authorize?client_id=%s&response_type=code",
		p.BaseURL(), p.Config().ClientID), nil
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
	return p.VerifyHMACSHA256Signature(payload, signature, "sha256=")
}

// GetCommitData extracts commit data from a webhook payload
func (p *BitbucketProvider) GetCommitData(payload map[string]interface{}) *CommitData {
	return CommitDataFromBitbucketPayload(payload)
}

// TestConnection tests the connection to Bitbucket
func (p *BitbucketProvider) TestConnection(ctx context.Context) error {
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
	return p.GetOAuthToken()
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
					if linkName, ok := linkMap["name"].(string); ok && linkName == "ssh" {
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

// FetchRepositories fetches repositories from Bitbucket API with pagination
func (p *BitbucketProvider) FetchRepositories(ctx context.Context, workspace string) ([]map[string]interface{}, error) {
	token, err := p.GetOAuthToken()
	if err != nil {
		return nil, err
	}

	items, err := p.FetchAllPages(
		ctx,
		fmt.Sprintf("/repositories/%s", workspace),
		token,
		func(response map[string]interface{}) ([]map[string]interface{}, error) {
			// Bitbucket returns items in "values" array
			values, ok := response["values"].([]interface{})
			if !ok {
				return nil, nil
			}
			var items []map[string]interface{}
			for _, v := range values {
				if item, ok := v.(map[string]interface{}); ok {
					items = append(items, item)
				}
			}
			return items, nil
		},
		func(resp *http.Response, body map[string]interface{}) string {
			// Bitbucket uses "next" field for pagination
			if next, ok := body["next"].(string); ok {
				// Extract path from full URL
				if len(next) > len(bitbucketAPIURL) {
					return next[len(bitbucketAPIURL):]
				}
				return next
			}
			return ""
		},
	)
	if err != nil {
		return nil, err
	}

	// Parse each repository
	var repos []map[string]interface{}
	for _, item := range items {
		repos = append(repos, parseBitbucketRepository(item))
	}

	return repos, nil
}
