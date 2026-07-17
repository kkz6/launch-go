package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/httpclient"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

const (
	githubAPIURL  = "https://api.github.com"
	githubBaseURL = "https://github.com"
)

// GitHubProvider implements the Provider interface for GitHub
type GitHubProvider struct {
	*BaseGitProvider
}

// NewGitHubProvider creates a new GitHub provider
func NewGitHubProvider(config *ProviderConfig) *GitHubProvider {
	base := NewBaseGitProvider(
		config,
		WithProviderType(GitProviderGitHub),
		WithBaseURL(githubBaseURL),
		WithAPIURL(githubAPIURL),
	)

	// Set GitHub-specific Accept header
	base.APIClient().SetHeader("Accept", "application/vnd.github.v3+json")

	return &GitHubProvider{
		BaseGitProvider: base,
	}
}

// GetInstallationURL returns the URL to install the GitHub App
func (p *GitHubProvider) GetInstallationURL() (string, error) {
	if p.Config().AppSlug == "" {
		return "", errors.New("GitHub App slug is not configured")
	}
	return util.New(p.BaseURL()).Path("apps", p.Config().AppSlug, "installations", "new").String(), nil
}

// generateJWT generates a JWT for GitHub App authentication
func (p *GitHubProvider) generateJWT() (string, error) {
	config := p.Config()
	if config.AppID == "" || config.PrivateKey == "" {
		return "", errors.New("GitHub App configuration is missing")
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": config.AppID,
	}

	privateKey := strings.ReplaceAll(config.PrivateKey, "\\n", "\n")
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKey))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(key)
}

// GetInstallationToken gets an access token for an installation
func (p *GitHubProvider) GetInstallationToken(ctx context.Context, installationID string) (string, error) {
	jwtToken, err := p.generateJWT()
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("/app/installations/%s/access_tokens", installationID)
	resp, err := p.DoRaw(ctx, http.MethodPost, path, jwtToken, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		_, _ = io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get installation token: status %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := DecodeJSON(resp, &result); err != nil {
		return "", err
	}

	return result.Token, nil
}

// GetInstallation gets an installation by ID
func (p *GitHubProvider) GetInstallation(ctx context.Context, installationID string) (*AppInstallationData, error) {
	jwtToken, err := p.generateJWT()
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/app/installations/%s", installationID)
	resp, err := p.DoRaw(ctx, http.MethodGet, path, jwtToken, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrInstallationNotFound
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get installation: status %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := DecodeJSON(resp, &data); err != nil {
		return nil, err
	}

	return p.mapInstallationData(data), nil
}

// GetAllInstallations gets all installations for the app
func (p *GitHubProvider) GetAllInstallations(ctx context.Context) ([]AppInstallationData, error) {
	jwtToken, err := p.generateJWT()
	if err != nil {
		return nil, err
	}

	items, err := p.FetchAllPages(
		ctx,
		"/app/installations?per_page=100",
		jwtToken,
		func(response map[string]interface{}) ([]map[string]interface{}, error) {
			// For this endpoint, the response is an array directly, so this won't be called
			return nil, nil
		},
		func(resp *http.Response, body map[string]interface{}) string {
			if HasNextPage(resp.Header.Get("Link")) {
				return ParseLinkHeader(resp.Header.Get("Link"))
			}
			return ""
		},
	)
	if err != nil {
		return nil, err
	}

	var installations []AppInstallationData
	for _, item := range items {
		installations = append(installations, *p.mapInstallationData(item))
	}

	return installations, nil
}

// GetInstallationRepositories gets repositories for an installation
func (p *GitHubProvider) GetInstallationRepositories(ctx context.Context, installationID string) ([]map[string]interface{}, error) {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	var allRepos []map[string]interface{}
	path := "/installation/repositories?per_page=100"

	for path != "" {
		resp, err := p.DoRaw(ctx, http.MethodGet, path, token, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to get repositories: status %d", resp.StatusCode)
		}

		var result struct {
			Repositories []map[string]interface{} `json:"repositories"`
			TotalCount   int                      `json:"total_count"`
		}

		if err := DecodeJSON(resp, &result); err != nil {
			return nil, err
		}

		allRepos = append(allRepos, result.Repositories...)

		if len(allRepos) >= result.TotalCount {
			break
		}

		if HasNextPage(resp.Header.Get("Link")) {
			path = ParseLinkHeader(resp.Header.Get("Link"))
		} else {
			path = ""
		}
	}

	return allRepos, nil
}

// GetRepository gets a specific repository
func (p *GitHubProvider) GetRepository(ctx context.Context, installationID, owner, repo string) (map[string]interface{}, error) {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	resp, err := p.DoRaw(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fiberutil.NotFound()
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrPermissionDenied
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get repository: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := DecodeJSON(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ValidateWebhook validates a webhook signature
func (p *GitHubProvider) ValidateWebhook(payload []byte, signature string) bool {
	return p.VerifyHMACSHA256Signature(payload, signature, "sha256=")
}

// GetCommitData extracts commit data from a webhook payload
func (p *GitHubProvider) GetCommitData(payload map[string]interface{}) *CommitData {
	return CommitDataFromGitHubPayload(payload)
}

// TestConnection tests the connection to GitHub
func (p *GitHubProvider) TestConnection(ctx context.Context) error {
	_, err := p.GetAllInstallations(ctx)
	return err
}

// DeployKey deploys an SSH key to a repository
func (p *GitHubProvider) DeployKey(ctx context.Context, sourceControlID, title, repo, key string) error {
	sc := p.SourceControl()
	if sc == nil || sc.InstallationID == nil {
		return errors.New("no source control configured")
	}

	token, err := p.GetInstallationToken(ctx, *sc.InstallationID)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/repos/%s/keys", repo)
	body := map[string]interface{}{
		"title":     title,
		"key":       key,
		"read_only": true,
	}

	resp, err := p.DoRaw(ctx, http.MethodPost, path, token, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		_, _ = io.ReadAll(resp.Body)
		return fmt.Errorf("failed to deploy key: status %d", resp.StatusCode)
	}

	return nil
}

// GetLastCommit gets the last commit for a repository and branch
func (p *GitHubProvider) GetLastCommit(ctx context.Context, sourceControlID, repo, branch string) (*CommitData, error) {
	sc := p.SourceControl()
	if sc == nil || sc.InstallationID == nil {
		return nil, errors.New("no source control configured")
	}

	token, err := p.GetInstallationToken(ctx, *sc.InstallationID)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/repos/%s/commits?sha=%s&per_page=1", repo, branch)
	resp, err := p.DoRaw(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get commits: status %d", resp.StatusCode)
	}

	var commits []map[string]interface{}
	if err := DecodeJSON(resp, &commits); err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return nil, nil
	}

	commit := commits[0]
	sha, _ := commit["sha"].(string)
	shortSHA := sha
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}

	var name, email, message, htmlURL string
	if commitData, ok := commit["commit"].(map[string]interface{}); ok {
		if author, ok := commitData["author"].(map[string]interface{}); ok {
			name, _ = author["name"].(string)
			email, _ = author["email"].(string)
		}
		message, _ = commitData["message"].(string)
	}
	htmlURL, _ = commit["html_url"].(string)

	return &CommitData{
		CommitID: sha,
		SHA:      shortSHA,
		Name:     name,
		Email:    email,
		Message:  message,
		URL:      htmlURL,
		Branch:   branch,
	}, nil
}

// CreateDeployment creates a deployment on GitHub
func (p *GitHubProvider) CreateDeployment(ctx context.Context, info *DeploymentInfo) (*DeploymentResult, error) {
	sc := p.SourceControl()
	if sc == nil || sc.InstallationID == nil {
		return nil, errors.New("no source control configured")
	}

	token, err := p.GetInstallationToken(ctx, *sc.InstallationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get installation token: %w", err)
	}

	// Create deployment
	path := fmt.Sprintf("/repos/%s/deployments", info.RepoFullName)
	ref := info.Branch
	if info.GitHash != "" {
		ref = info.GitHash
	}
	body := map[string]interface{}{
		"ref":               ref,
		"description":       info.Description,
		"auto_merge":        false,
		"required_contexts": []string{},
	}

	resp, err := p.DoRaw(ctx, http.MethodPost, path, token, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create deployment: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var deploymentResp map[string]interface{}
	if err := DecodeJSON(resp, &deploymentResp); err != nil {
		return nil, err
	}

	deploymentID := ExtractFloatID(deploymentResp, "id")
	statusesURL, _ := deploymentResp["statuses_url"].(string)

	// Remove creator from data to avoid storing sensitive info
	delete(deploymentResp, "creator")

	result := &DeploymentResult{
		ID:          deploymentID,
		StatusesURL: statusesURL,
		Data:        deploymentResp,
	}

	if statusesURL == "" {
		return result, nil
	}

	if err := p.createDeploymentStatus(ctx, statusesURL, token, info.SiteURL, DeploymentStatusInProgress); err != nil {
		return result, err
	}

	return result, nil
}

// createDeploymentStatus creates a deployment status
func (p *GitHubProvider) createDeploymentStatus(ctx context.Context, statusesURL, token, siteURL string, status DeploymentStatus) error {
	statusBody := map[string]interface{}{
		"state":           status.GitHubStatus(),
		"description":     "Deployment " + string(status),
		"environment_url": siteURL,
	}

	// Use a relative path if possible, or direct URL
	// Since statuses_url is a full URL, we need to make a direct request
	req, err := httpclient.NewRequest(ctx, http.MethodPost, statusesURL).
		BearerAuth(token).
		WithHeader("Accept", "application/vnd.github.v3+json").
		JSONBody(statusBody).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build deployment status request: %w", err)
	}

	resp, err := httpclient.Default().Do(req)
	if err != nil {
		return fmt.Errorf("failed to create deployment status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create deployment status: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

// UpdateDeploymentStatus updates the status of a deployment on GitHub
func (p *GitHubProvider) UpdateDeploymentStatus(ctx context.Context, info *DeploymentInfo, vcsData map[string]interface{}, status DeploymentStatus) error {
	sc := p.SourceControl()
	if sc == nil || sc.InstallationID == nil {
		return nil
	}

	statusesURL, ok := vcsData["statuses_url"].(string)
	if !ok || statusesURL == "" {
		return nil
	}

	token, err := p.GetInstallationToken(ctx, *sc.InstallationID)
	if err != nil {
		return fmt.Errorf("failed to get installation token: %w", err)
	}

	return p.createDeploymentStatus(ctx, statusesURL, token, info.SiteURL, status)
}

// mapInstallationData maps raw installation data to AppInstallationData
func (p *GitHubProvider) mapInstallationData(data map[string]interface{}) *AppInstallationData {
	id := ExtractFloatID(data, "id")

	account, _ := data["account"].(map[string]interface{})
	accountID := ExtractFloatID(account, "id")

	accountLogin, _ := account["login"].(string)
	accountType, _ := account["type"].(string)
	accountAvatarURL, _ := account["avatar_url"].(string)

	permissions, _ := data["permissions"].(map[string]interface{})
	repositories, _ := data["repositories"].([]interface{})
	targetType, _ := data["target_type"].(string)
	htmlURL, _ := data["html_url"].(string)
	createdAt, _ := data["created_at"].(string)
	updatedAt, _ := data["updated_at"].(string)
	suspendedAt, _ := data["suspended_at"].(string)
	singleFileName, _ := data["single_file_name"].(string)
	repositorySelection, _ := data["repository_selection"].(string)
	appSlug, _ := data["app_slug"].(string)

	var appID int64
	if appIDFloat, ok := data["app_id"].(float64); ok {
		appID = int64(appIDFloat)
	}

	var events []string
	if eventsList, ok := data["events"].([]interface{}); ok {
		for _, e := range eventsList {
			if s, ok := e.(string); ok {
				events = append(events, s)
			}
		}
	}

	return &AppInstallationData{
		ID:                      id,
		AccountID:               accountID,
		AccountLogin:            accountLogin,
		AccountType:             accountType,
		AccountAvatarURL:        accountAvatarURL,
		Permissions:             permissions,
		Repositories:            repositories,
		TargetType:              targetType,
		HTMLURL:                 htmlURL,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
		SuspendedAt:             suspendedAt,
		Events:                  events,
		SingleFileName:          singleFileName,
		HasMultipleRepositories: repositorySelection == "selected",
		RepositorySelection:     repositorySelection,
		AppSlug:                 appSlug,
		AppID:                   appID,
	}
}
