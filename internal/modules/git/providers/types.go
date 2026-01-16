package providers

import (
	"regexp"
	"strings"
)

// GitProviderType represents the type of git provider (for provider operations)
type GitProviderType string

const (
	GitProviderGitHub    GitProviderType = "github"
	GitProviderGitLab    GitProviderType = "gitlab"
	GitProviderBitbucket GitProviderType = "bitbucket"
)

// String returns the string representation of the provider
func (p GitProviderType) String() string {
	return string(p)
}

// Label returns a human-readable label for the provider
func (p GitProviderType) Label() string {
	switch p {
	case GitProviderGitHub:
		return "GitHub"
	case GitProviderGitLab:
		return "GitLab"
	case GitProviderBitbucket:
		return "Bitbucket"
	default:
		return strings.Title(string(p))
	}
}

// IsValid checks if the provider type is valid
func (p GitProviderType) IsValid() bool {
	switch p {
	case GitProviderGitHub, GitProviderGitLab, GitProviderBitbucket:
		return true
	default:
		return false
	}
}

// HTTPSURL returns the plain HTTPS clone URL for a repository
func (p GitProviderType) HTTPSURL(repoFullName string) string {
	switch p {
	case GitProviderGitHub:
		return "https://github.com/" + repoFullName + ".git"
	case GitProviderGitLab:
		return "https://gitlab.com/" + repoFullName + ".git"
	case GitProviderBitbucket:
		return "https://bitbucket.org/" + repoFullName + ".git"
	default:
		return ""
	}
}

// SSHURL returns the SSH clone URL for a repository
func (p GitProviderType) SSHURL(repoFullName string) string {
	switch p {
	case GitProviderGitHub:
		return "git@github.com:" + repoFullName + ".git"
	case GitProviderGitLab:
		return "git@gitlab.com:" + repoFullName + ".git"
	case GitProviderBitbucket:
		return "git@bitbucket.org:" + repoFullName + ".git"
	default:
		return ""
	}
}

// AuthURL returns the HTTPS URL with embedded token for authenticated cloning
func (p GitProviderType) AuthURL(token, repoFullName string) string {
	switch p {
	case GitProviderGitHub:
		return "https://x-access-token:" + token + "@github.com/" + repoFullName + ".git"
	case GitProviderGitLab:
		return "https://gitlab-ci-token:" + token + "@gitlab.com/" + repoFullName + ".git"
	case GitProviderBitbucket:
		return "https://x-token-auth:" + token + "@bitbucket.org/" + repoFullName + ".git"
	default:
		return ""
	}
}

// AppInstallationData represents data from a git provider app installation
type AppInstallationData struct {
	ID                      string                 `json:"id"`
	AccountID               string                 `json:"account_id"`
	AccountLogin            string                 `json:"account_login"`
	AccountType             string                 `json:"account_type"`
	AccountAvatarURL        string                 `json:"account_avatar_url"`
	Permissions             map[string]interface{} `json:"permissions"`
	Repositories            []interface{}          `json:"repositories"`
	TargetType              string                 `json:"target_type,omitempty"`
	HTMLURL                 string                 `json:"html_url,omitempty"`
	CreatedAt               string                 `json:"created_at,omitempty"`
	UpdatedAt               string                 `json:"updated_at,omitempty"`
	SuspendedAt             string                 `json:"suspended_at,omitempty"`
	Events                  []string               `json:"events,omitempty"`
	SingleFileName          string                 `json:"single_file_name,omitempty"`
	HasMultipleRepositories bool                   `json:"has_multiple_repositories"`
	RepositorySelection     string                 `json:"repository_selection,omitempty"`
	AppSlug                 string                 `json:"app_slug,omitempty"`
	AppID                   int64                  `json:"app_id,omitempty"`
}

// IsOrganization checks if the installation is for an organization
func (a *AppInstallationData) IsOrganization() bool {
	return strings.ToLower(a.AccountType) == "organization"
}

// IsUser checks if the installation is for a user
func (a *AppInstallationData) IsUser() bool {
	return strings.ToLower(a.AccountType) == "user"
}

// IsSuspended checks if the installation is suspended
func (a *AppInstallationData) IsSuspended() bool {
	return a.SuspendedAt != ""
}

// ToMap converts the installation data to a map
func (a *AppInstallationData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":                        a.ID,
		"account_id":                a.AccountID,
		"account_login":             a.AccountLogin,
		"account_type":              a.AccountType,
		"account_avatar_url":        a.AccountAvatarURL,
		"permissions":               a.Permissions,
		"repositories":              a.Repositories,
		"target_type":               a.TargetType,
		"html_url":                  a.HTMLURL,
		"created_at":                a.CreatedAt,
		"updated_at":                a.UpdatedAt,
		"suspended_at":              a.SuspendedAt,
		"events":                    a.Events,
		"single_file_name":          a.SingleFileName,
		"has_multiple_repositories": a.HasMultipleRepositories,
		"repository_selection":      a.RepositorySelection,
		"app_slug":                  a.AppSlug,
		"app_id":                    a.AppID,
	}
}

// CommitData represents commit information from a webhook payload
type CommitData struct {
	CommitID string `json:"commit_id"`
	SHA      string `json:"sha"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Message  string `json:"message,omitempty"`
	URL      string `json:"url,omitempty"`
	Branch   string `json:"branch,omitempty"`
}

// ToMap converts commit data to a flat map for storage
func (c *CommitData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"sha":     c.SHA,
		"url":     c.URL,
		"name":    c.Name,
		"email":   c.Email,
		"message": c.Message,
	}
}

// SourceControlData represents source control data for provider operations
type SourceControlData struct {
	ID                      string
	UserID                  string
	TeamID                  string
	Provider                GitProviderType
	URL                     *string
	ProviderID              *string
	ProviderData            map[string]interface{}
	ProviderAccountID       *string
	Login                   *string
	Name                    *string
	Type                    *string
	AvatarURL               *string
	HTMLURL                 *string
	InstallationID          *string
	Permissions             map[string]interface{}
	RepositorySelection     *string
	HasMultipleRepositories bool
	RepositoryCount         int
}

// GetLogin returns the login or empty string if nil
func (s *SourceControlData) GetLogin() string {
	if s.Login == nil {
		return ""
	}
	return *s.Login
}

// GetInstallationID returns the installation ID or empty string if nil
func (s *SourceControlData) GetInstallationID() string {
	if s.InstallationID == nil {
		return ""
	}
	return *s.InstallationID
}

// CommitDataFromGitHubPayload creates CommitData from a GitHub webhook payload
func CommitDataFromGitHubPayload(data map[string]interface{}) *CommitData {
	headCommit, ok := data["head_commit"].(map[string]interface{})
	if !ok {
		return nil
	}

	commitID, _ := headCommit["id"].(string)
	if commitID == "" {
		return nil
	}

	sha := commitID
	if len(sha) > 7 {
		sha = sha[:7]
	}

	var name, email string
	if author, ok := headCommit["author"].(map[string]interface{}); ok {
		name, _ = author["name"].(string)
		email, _ = author["email"].(string)
	}

	message, _ := headCommit["message"].(string)
	url, _ := headCommit["url"].(string)

	ref, _ := data["ref"].(string)
	branch := strings.TrimPrefix(ref, "refs/heads/")

	return &CommitData{
		CommitID: commitID,
		SHA:      sha,
		Name:     name,
		Email:    email,
		Message:  message,
		URL:      url,
		Branch:   branch,
	}
}

// CommitDataFromGitLabPayload creates CommitData from a GitLab webhook payload
func CommitDataFromGitLabPayload(data map[string]interface{}) *CommitData {
	commits, ok := data["commits"].([]interface{})
	if !ok || len(commits) == 0 {
		return nil
	}

	firstCommit, ok := commits[0].(map[string]interface{})
	if !ok {
		return nil
	}

	commitID, _ := firstCommit["id"].(string)
	if commitID == "" {
		return nil
	}

	sha := commitID
	if len(sha) > 7 {
		sha = sha[:7]
	}

	var name, email string
	if author, ok := firstCommit["author"].(map[string]interface{}); ok {
		name, _ = author["name"].(string)
		email, _ = author["email"].(string)
	}

	title, _ := firstCommit["title"].(string)
	url, _ := firstCommit["url"].(string)

	ref, _ := data["ref"].(string)
	branch := strings.TrimPrefix(ref, "refs/heads/")

	return &CommitData{
		CommitID: commitID,
		SHA:      sha,
		Name:     name,
		Email:    email,
		Message:  title,
		URL:      url,
		Branch:   branch,
	}
}

// CommitDataFromBitbucketPayload creates CommitData from a Bitbucket webhook payload
func CommitDataFromBitbucketPayload(data map[string]interface{}) *CommitData {
	push, ok := data["push"].(map[string]interface{})
	if !ok {
		return nil
	}

	changes, ok := push["changes"].([]interface{})
	if !ok || len(changes) == 0 {
		return nil
	}

	firstChange, ok := changes[0].(map[string]interface{})
	if !ok {
		return nil
	}

	commits, ok := firstChange["commits"].([]interface{})
	if !ok || len(commits) == 0 {
		return nil
	}

	firstCommit, ok := commits[0].(map[string]interface{})
	if !ok {
		return nil
	}

	hash, _ := firstCommit["hash"].(string)
	if hash == "" {
		return nil
	}

	sha := hash
	if len(sha) > 7 {
		sha = sha[:7]
	}

	var name, email string
	if author, ok := firstCommit["author"].(map[string]interface{}); ok {
		raw, _ := author["raw"].(string)
		name, email = parseCommitter(raw)
	}

	message, _ := firstCommit["message"].(string)
	message = strings.ReplaceAll(message, "\n", "")

	var url string
	if links, ok := firstCommit["links"].(map[string]interface{}); ok {
		if html, ok := links["html"].(map[string]interface{}); ok {
			url, _ = html["href"].(string)
		}
	}

	var branch string
	if newRef, ok := firstChange["new"].(map[string]interface{}); ok {
		branch, _ = newRef["name"].(string)
	}

	if branch == "" {
		if oldRef, ok := firstChange["old"].(map[string]interface{}); ok {
			branch, _ = oldRef["name"].(string)
		}
	}

	return &CommitData{
		CommitID: hash,
		SHA:      sha,
		Name:     name,
		Email:    email,
		Message:  message,
		URL:      url,
		Branch:   branch,
	}
}

// parseCommitter parses a raw author string like "John Doe <john@example.com>"
func parseCommitter(raw string) (name, email string) {
	re := regexp.MustCompile(`^(.*?)\s*<(.+?)>$`)
	matches := re.FindStringSubmatch(raw)

	if len(matches) == 3 {
		return strings.TrimSpace(matches[1]), matches[2]
	}

	return "", ""
}

// DeploymentStatus represents the status of a deployment on the git provider
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusInProgress DeploymentStatus = "in_progress"
	DeploymentStatusSuccess    DeploymentStatus = "success"
	DeploymentStatusFailure    DeploymentStatus = "failure"
	DeploymentStatusError      DeploymentStatus = "error"
)

// GitHubDeploymentStatus returns the GitHub-specific status string
func (s DeploymentStatus) GitHubStatus() string {
	switch s {
	case DeploymentStatusPending:
		return "pending"
	case DeploymentStatusInProgress:
		return "in_progress"
	case DeploymentStatusSuccess:
		return "success"
	case DeploymentStatusFailure:
		return "failure"
	case DeploymentStatusError:
		return "error"
	default:
		return "pending"
	}
}

// GitLabDeploymentStatus returns the GitLab-specific status string
func (s DeploymentStatus) GitLabStatus() string {
	switch s {
	case DeploymentStatusPending:
		return "created"
	case DeploymentStatusInProgress:
		return "running"
	case DeploymentStatusSuccess:
		return "success"
	case DeploymentStatusFailure:
		return "failed"
	case DeploymentStatusError:
		return "failed"
	default:
		return "created"
	}
}

// DeploymentInfo contains information for creating a deployment on the git provider
type DeploymentInfo struct {
	ServerID       string
	SiteID         string
	DeploymentID   string
	RepoFullName   string
	Branch         string
	GitHash        string
	SiteURL        string
	Environment    string
	Description    string
	ProjectID      string // For GitLab (numeric project ID from additional_data)
}

// DeploymentResult contains the result of creating a deployment on the git provider
type DeploymentResult struct {
	ID          string                 `json:"id"`
	StatusesURL string                 `json:"statuses_url,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}
