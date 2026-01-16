package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	githubAPIURL  = "https://api.github.com"
	githubBaseURL = "https://github.com"
)

// GitHubProvider implements the Provider interface for GitHub
type GitHubProvider struct {
	config        *ProviderConfig
	sourceControl *SourceControlData
	httpClient    *http.Client
}

// NewGitHubProvider creates a new GitHub provider
func NewGitHubProvider(config *ProviderConfig) *GitHubProvider {
	return &GitHubProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetSourceControl sets the source control context
func (p *GitHubProvider) SetSourceControl(sc *SourceControlData) {
	p.sourceControl = sc
}

// GetType returns the provider type
func (p *GitHubProvider) GetType() GitProviderType {
	return GitProviderGitHub
}

// GetInstallationURL returns the URL to install the GitHub App
func (p *GitHubProvider) GetInstallationURL() (string, error) {
	if p.config.AppSlug == "" {
		return "", errors.New("GitHub App slug is not configured")
	}
	return fmt.Sprintf("%s/apps/%s/installations/new", githubBaseURL, p.config.AppSlug), nil
}

// generateJWT generates a JWT for GitHub App authentication
func (p *GitHubProvider) generateJWT() (string, error) {
	if p.config.AppID == "" || p.config.PrivateKey == "" {
		return "", errors.New("GitHub App configuration is missing")
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": p.config.AppID,
	}

	privateKey := strings.ReplaceAll(p.config.PrivateKey, "\\n", "\n")
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKey))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(key)
}

// GetInstallationToken gets an access token for an installation
// Implements Provider interface
func (p *GitHubProvider) GetInstallationToken(ctx context.Context, installationID string) (string, error) {
	jwtToken, err := p.generateJWT()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/app/installations/%s/access_tokens", githubAPIURL, installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get installation token: %s", string(body))
	}

	var result struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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

	url := fmt.Sprintf("%s/app/installations/%s", githubAPIURL, installationID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrInstallationNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get installation: %s", string(body))
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
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

	var allInstallations []AppInstallationData
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("%s/app/installations?per_page=%d&page=%d", githubAPIURL, perPage, page)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+jwtToken)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get installations: %s", string(body))
		}

		var installations []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&installations); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, inst := range installations {
			allInstallations = append(allInstallations, *p.mapInstallationData(inst))
		}

		// Check for more pages
		linkHeader := resp.Header.Get("Link")
		if !strings.Contains(linkHeader, `rel="next"`) {
			break
		}

		page++
	}

	return allInstallations, nil
}

// GetInstallationRepositories gets repositories for an installation
func (p *GitHubProvider) GetInstallationRepositories(ctx context.Context, installationID string) ([]map[string]interface{}, error) {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	var allRepos []map[string]interface{}
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("%s/installation/repositories?per_page=%d&page=%d", githubAPIURL, perPage, page)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get repositories: %s", string(body))
		}

		var result struct {
			Repositories []map[string]interface{} `json:"repositories"`
			TotalCount   int                      `json:"total_count"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		allRepos = append(allRepos, result.Repositories...)

		if len(allRepos) >= result.TotalCount {
			break
		}

		page++
	}

	return allRepos, nil
}

// GetRepository gets a specific repository
func (p *GitHubProvider) GetRepository(ctx context.Context, installationID, owner, repo string) (map[string]interface{}, error) {
	token, err := p.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/%s", githubAPIURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrRepositoryNotFound
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrPermissionDenied
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get repository: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// ValidateWebhook validates a webhook signature
func (p *GitHubProvider) ValidateWebhook(payload []byte, signature string) bool {
	if p.config.WebhookSecret == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(p.config.WebhookSecret))
	mac.Write(payload)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
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

// GetSSHURL returns the SSH URL for a repository
func (p *GitHubProvider) GetSSHURL(repo string) string {
	return fmt.Sprintf("git@github.com:%s.git", repo)
}

// GetHTTPSURL returns the HTTPS URL for a repository
func (p *GitHubProvider) GetHTTPSURL(repo string) string {
	return fmt.Sprintf("https://github.com/%s.git", repo)
}

// DeployKey deploys an SSH key to a repository
func (p *GitHubProvider) DeployKey(ctx context.Context, sourceControlID, title, repo, key string) error {
	if p.sourceControl == nil || p.sourceControl.InstallationID == nil {
		return errors.New("no source control configured")
	}

	token, err := p.GetInstallationToken(ctx, *p.sourceControl.InstallationID)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/repos/%s/keys", githubAPIURL, repo)
	body := map[string]interface{}{
		"title":     title,
		"key":       key,
		"read_only": true,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to deploy key: %s", string(respBody))
	}

	return nil
}

// GetLastCommit gets the last commit for a repository and branch
func (p *GitHubProvider) GetLastCommit(ctx context.Context, sourceControlID, repo, branch string) (*CommitData, error) {
	if p.sourceControl == nil || p.sourceControl.InstallationID == nil {
		return nil, errors.New("no source control configured")
	}

	token, err := p.GetInstallationToken(ctx, *p.sourceControl.InstallationID)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/commits?sha=%s&per_page=1", githubAPIURL, repo, branch)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get commits: status %d", resp.StatusCode)
	}

	var commits []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
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
	if p.sourceControl == nil || p.sourceControl.InstallationID == nil {
		return nil, errors.New("no source control configured")
	}

	token, err := p.GetInstallationToken(ctx, *p.sourceControl.InstallationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get installation token: %w", err)
	}

	// Create deployment
	url := fmt.Sprintf("%s/repos/%s/deployments", githubAPIURL, info.RepoFullName)
	body := map[string]interface{}{
		"ref":         info.Branch,
		"description": info.Description,
		"environment": info.Environment,
		"auto_merge":  false,
	}
	if info.GitHash != "" {
		body["sha"] = info.GitHash
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create deployment: %s", string(respBody))
	}

	var deploymentResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&deploymentResp); err != nil {
		return nil, err
	}

	// Extract deployment ID
	deploymentID := ""
	if idFloat, ok := deploymentResp["id"].(float64); ok {
		deploymentID = fmt.Sprintf("%.0f", idFloat)
	}

	statusesURL, _ := deploymentResp["statuses_url"].(string)

	// Create initial status (in_progress)
	if statusesURL != "" {
		statusBody := map[string]interface{}{
			"state":           DeploymentStatusInProgress.GitHubStatus(),
			"description":     "Deployment in progress",
			"environment_url": info.SiteURL,
		}
		statusBodyBytes, _ := json.Marshal(statusBody)

		statusReq, err := http.NewRequestWithContext(ctx, "POST", statusesURL, strings.NewReader(string(statusBodyBytes)))
		if err == nil {
			statusReq.Header.Set("Authorization", "Bearer "+token)
			statusReq.Header.Set("Accept", "application/vnd.github.v3+json")
			statusReq.Header.Set("Content-Type", "application/json")
			statusResp, _ := p.httpClient.Do(statusReq)
			if statusResp != nil {
				statusResp.Body.Close()
			}
		}
	}

	// Remove creator from data to avoid storing sensitive info
	delete(deploymentResp, "creator")

	return &DeploymentResult{
		ID:          deploymentID,
		StatusesURL: statusesURL,
		Data:        deploymentResp,
	}, nil
}

// UpdateDeploymentStatus updates the status of a deployment on GitHub
func (p *GitHubProvider) UpdateDeploymentStatus(ctx context.Context, info *DeploymentInfo, vcsData map[string]interface{}, status DeploymentStatus) error {
	if p.sourceControl == nil || p.sourceControl.InstallationID == nil {
		return nil
	}

	statusesURL, ok := vcsData["statuses_url"].(string)
	if !ok || statusesURL == "" {
		return nil
	}

	token, err := p.GetInstallationToken(ctx, *p.sourceControl.InstallationID)
	if err != nil {
		return fmt.Errorf("failed to get installation token: %w", err)
	}

	description := "Deployment " + string(status)
	body := map[string]interface{}{
		"state":           status.GitHubStatus(),
		"description":     description,
		"environment_url": info.SiteURL,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", statusesURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update deployment status: %s", string(respBody))
	}

	return nil
}

// mapInstallationData maps raw installation data to AppInstallationData
func (p *GitHubProvider) mapInstallationData(data map[string]interface{}) *AppInstallationData {
	id := ""
	if idFloat, ok := data["id"].(float64); ok {
		id = fmt.Sprintf("%.0f", idFloat)
	}

	account, _ := data["account"].(map[string]interface{})
	accountID := ""
	if accountIDFloat, ok := account["id"].(float64); ok {
		accountID = fmt.Sprintf("%.0f", accountIDFloat)
	}

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
