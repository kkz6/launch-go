package dto

import (
	"regexp"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/git/gitref"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

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

	// Store full SHA - UI can truncate for display
	sha := commitID

	var name, email string
	if author, ok := headCommit["author"].(map[string]interface{}); ok {
		name, _ = author["name"].(string)
		email, _ = author["email"].(string)
	}

	message, _ := headCommit["message"].(string)
	url, _ := headCommit["url"].(string)

	ref, _ := data["ref"].(string)
	branch := gitref.ExtractBranchName(ref)

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

	// Store full SHA - UI can truncate for display
	sha := commitID

	var name, email string
	if author, ok := firstCommit["author"].(map[string]interface{}); ok {
		name, _ = author["name"].(string)
		email, _ = author["email"].(string)
	}

	title, _ := firstCommit["title"].(string)
	url, _ := firstCommit["url"].(string)

	ref, _ := data["ref"].(string)
	branch := gitref.ExtractBranchName(ref)

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

	// Store full SHA - UI can truncate for display
	sha := hash

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

// RepositoryData represents repository data from API
type RepositoryData struct {
	Name           string                 `json:"name"`
	FullName       string                 `json:"full_name"`
	IsPublic       bool                   `json:"public"`
	DefaultBranch  string                 `json:"default_branch"`
	HTMLURL        string                 `json:"html_url"`
	SSHURL         string                 `json:"ssh_url"`
	AdditionalData map[string]interface{} `json:"additional_data,omitempty"`
}

// RepositoryDataFromAPIResponse creates RepositoryData from an API response
func RepositoryDataFromAPIResponse(data map[string]interface{}) *RepositoryData {
	name, _ := data["name"].(string)
	fullName, _ := data["full_name"].(string)

	private, _ := data["private"].(bool)
	isPublic := !private

	defaultBranch, _ := data["default_branch"].(string)
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	htmlURL, _ := data["html_url"].(string)
	sshURL, _ := data["ssh_url"].(string)

	var id interface{}
	if idFloat, ok := data["id"].(float64); ok {
		id = int64(idFloat)
	} else {
		id = data["id"]
	}

	return &RepositoryData{
		Name:          name,
		FullName:      fullName,
		IsPublic:      isPublic,
		DefaultBranch: defaultBranch,
		HTMLURL:       htmlURL,
		SSHURL:        sshURL,
		AdditionalData: map[string]interface{}{
			"id":          id,
			"description": data["description"],
			"language":    data["language"],
			"updated_at":  data["updated_at"],
		},
	}
}

// ToMap converts RepositoryData to a map for storage
func (r *RepositoryData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"name":            r.Name,
		"full_name":       r.FullName,
		"public":          r.IsPublic,
		"default_branch":  r.DefaultBranch,
		"html_url":        r.HTMLURL,
		"ssh_url":         r.SSHURL,
		"additional_data": r.AdditionalData,
	}
}

// WebhookPayload represents a parsed webhook payload
type WebhookPayload struct {
	Provider  gittypes.GitProviderType `json:"provider"`
	Event     string                   `json:"event"`
	Action    string                   `json:"action,omitempty"`
	Data      map[string]interface{}   `json:"data"`
	Signature string                   `json:"-"`
}
