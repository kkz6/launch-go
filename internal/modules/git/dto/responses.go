package dto

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/kkz6/launch-go/internal/modules/git/models"
)

// SourceControlResponse is the response for a source control
type SourceControlResponse struct {
	ID                      string               `json:"id"`
	Provider                string               `json:"provider"`
	ProviderLabel           string               `json:"provider_label"`
	Login                   string               `json:"login"`
	Name                    string               `json:"name,omitempty"`
	Type                    string               `json:"type,omitempty"`
	AvatarURL               string               `json:"avatar_url,omitempty"`
	HTMLURL                 string               `json:"html_url,omitempty"`
	InstallationID          string               `json:"installation_id,omitempty"`
	RepositorySelection     string               `json:"repository_selection,omitempty"`
	HasMultipleRepositories bool                 `json:"has_multiple_repositories"`
	RepositoryCount         int                  `json:"repository_count"`
	ConnectedAt             *string              `json:"connected_at,omitempty"`
	LastSyncedAt            *string              `json:"last_synced_at,omitempty"`
	CreatedAt               string               `json:"created_at"`
	Repositories            []RepositoryResponse `json:"repositories,omitempty"`
}

// ToSourceControlResponse converts a SourceControl to SourceControlResponse
func ToSourceControlResponse(sc *models.SourceControl) SourceControlResponse {
	repositoryCount := 0
	if sc.RepositoryCount != nil {
		repositoryCount = *sc.RepositoryCount
	}

	createdAt := ""
	if sc.CreatedAt != nil {
		createdAt = sc.CreatedAt.Format(time.RFC3339)
	}

	resp := SourceControlResponse{
		ID:                      sc.ID,
		Provider:                sc.Provider.String(),
		ProviderLabel:           sc.Provider.Label(),
		HasMultipleRepositories: sc.HasMultipleRepositories,
		RepositoryCount:         repositoryCount,
		CreatedAt:               createdAt,
	}

	if sc.Login != nil {
		resp.Login = *sc.Login
	}

	if sc.Name != nil {
		resp.Name = *sc.Name
	}

	if sc.Type != nil {
		resp.Type = *sc.Type
	}

	if sc.AvatarURL != nil {
		resp.AvatarURL = *sc.AvatarURL
	}

	if sc.HTMLURL != nil {
		resp.HTMLURL = *sc.HTMLURL
	}

	if sc.InstallationID != nil {
		resp.InstallationID = *sc.InstallationID
	}

	if sc.RepositorySelection != nil {
		resp.RepositorySelection = *sc.RepositorySelection
	}

	if sc.ConnectedAt != nil {
		t := sc.ConnectedAt.Format(time.RFC3339)
		resp.ConnectedAt = &t
	}

	if sc.LastSyncedAt != nil {
		t := sc.LastSyncedAt.Format(time.RFC3339)
		resp.LastSyncedAt = &t
	}

	if len(sc.Repositories) > 0 {
		resp.Repositories = make([]RepositoryResponse, len(sc.Repositories))
		for i, repo := range sc.Repositories {
			resp.Repositories[i] = ToRepositoryResponse(&repo)
		}
	}

	return resp
}

// RepositoryResponse is the response for a repository
type RepositoryResponse struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	FullName       string                 `json:"full_name"`
	Public         bool                   `json:"public"`
	DefaultBranch  string                 `json:"default_branch"`
	HTMLURL        string                 `json:"html_url,omitempty"`
	SSHURL         string                 `json:"ssh_url,omitempty"`
	AdditionalData map[string]interface{} `json:"additional_data,omitempty"`
}

// ToRepositoryResponse converts a SourceControlRepository to RepositoryResponse
func ToRepositoryResponse(repo *models.SourceControlRepository) RepositoryResponse {
	resp := RepositoryResponse{
		ID:            strconv.FormatUint(repo.ID, 10),
		Name:          repo.Name,
		FullName:      repo.FullName,
		Public:        repo.Public,
		DefaultBranch: repo.GetDefaultBranchOrMain(),
		HTMLURL:       repo.GetHTMLURL(),
		SSHURL:        repo.SSHURL,
	}

	if repo.AdditionalData != nil {
		var additionalData map[string]interface{}
		if err := json.Unmarshal([]byte(*repo.AdditionalData), &additionalData); err == nil {
			resp.AdditionalData = additionalData
		}
	}

	return resp
}

// InstallationURLResponse is the response for getting an installation URL
type InstallationURLResponse struct {
	URL      string `json:"url"`
	Provider string `json:"provider"`
}

// InstallationsResponse is the response for listing installations
type InstallationsResponse struct {
	Installations []AppInstallationData `json:"installations"`
	Provider      string                `json:"provider"`
}

// InstallationSummaryData represents a summary of an installation
type InstallationSummaryData struct {
	ID                      string `json:"id"`
	ProviderID              string `json:"provider_id"`
	Provider                string `json:"provider"`
	Login                   string `json:"login"`
	Name                    string `json:"name"`
	Type                    string `json:"type"`
	AvatarURL               string `json:"avatar_url"`
	HTMLURL                 string `json:"html_url"`
	InstallationID          string `json:"installation_id"`
	RepositorySelection     string `json:"repository_selection"`
	HasMultipleRepositories bool   `json:"has_multiple_repositories"`
	RepositoryCount         int    `json:"repository_count"`
	LastSyncedAt            string `json:"last_synced_at,omitempty"`
}

// InstallationSummaryFromSourceControl creates an InstallationSummaryData from a SourceControl
func InstallationSummaryFromSourceControl(sc *models.SourceControl) InstallationSummaryData {
	repositoryCount := 0
	if sc.RepositoryCount != nil {
		repositoryCount = *sc.RepositoryCount
	}

	summary := InstallationSummaryData{
		ID:                      sc.ID,
		Provider:                sc.Provider.String(),
		HasMultipleRepositories: sc.HasMultipleRepositories,
		RepositoryCount:         repositoryCount,
		ProviderID:              sc.ProviderID,
	}

	if sc.Login != nil {
		summary.Login = *sc.Login
	}

	if sc.Name != nil {
		summary.Name = *sc.Name
	}

	if sc.Type != nil {
		summary.Type = *sc.Type
	}

	if sc.AvatarURL != nil {
		summary.AvatarURL = *sc.AvatarURL
	}

	if sc.HTMLURL != nil {
		summary.HTMLURL = *sc.HTMLURL
	}

	if sc.InstallationID != nil {
		summary.InstallationID = *sc.InstallationID
	}

	if sc.RepositorySelection != nil {
		summary.RepositorySelection = *sc.RepositorySelection
	}

	if sc.LastSyncedAt != nil {
		summary.LastSyncedAt = sc.LastSyncedAt.Format(time.RFC3339)
	}

	return summary
}
