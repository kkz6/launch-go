package dto

import (
	"encoding/json"
	"strconv"

	"github.com/kkz6/launch-go/internal/modules/git/models"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// SourceControlResponse is the response for a source control
type SourceControlResponse struct {
	ID                      string               `json:"id"`
	Status                  string               `json:"status"`
	SelectionVersion        uint64               `json:"selection_version"`
	Generation              uint64               `json:"generation"`
	CleanupPending          bool                 `json:"cleanup_pending"`
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
	if sc.Status == "active" {
		for _, r := range sc.Repositories {
			if r.Selected && r.Available {
				repositoryCount++
			}
		}
	}

	resp := SourceControlResponse{
		ID:     sc.ID,
		Status: sc.Status, Generation: sc.Generation, SelectionVersion: sc.SelectionVersion, CleanupPending: sc.CleanupPending,
		Provider:                sc.Provider.String(),
		ProviderLabel:           sc.Provider.Label(),
		HasMultipleRepositories: sc.HasMultipleRepositories,
		RepositoryCount:         repositoryCount,
		ConnectedAt:             pkgdto.FormatTime(sc.ConnectedAt),
		LastSyncedAt:            pkgdto.FormatTime(sc.LastSyncedAt),
		CreatedAt:               pkgdto.FormatTimeOrEmpty(sc.CreatedAt),
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

	if len(sc.Repositories) > 0 {
		for i := range sc.Repositories {
			r := &sc.Repositories[i]
			if sc.Status == "active" && r.Selected && r.Available {
				resp.Repositories = append(resp.Repositories, ToRepositoryResponse(r))
			}
		}
	}

	return resp
}

// RepositoryResponse is the response for a repository
type RepositoryResponse struct {
	ID                 string                 `json:"id"`
	GitHubRepositoryID string                 `json:"github_repository_id,omitempty"`
	Name               string                 `json:"name"`
	FullName           string                 `json:"full_name"`
	Public             bool                   `json:"public"`
	DefaultBranch      string                 `json:"default_branch"`
	HTMLURL            string                 `json:"html_url,omitempty"`
	SSHURL             string                 `json:"ssh_url,omitempty"`
	AdditionalData     map[string]interface{} `json:"additional_data,omitempty"`
}

// ToRepositoryResponse converts a SourceControlRepository to RepositoryResponse
func ToRepositoryResponse(repo *models.SourceControlRepository) RepositoryResponse {
	resp := RepositoryResponse{
		ID:                 strconv.FormatUint(repo.ID, 10),
		GitHubRepositoryID: repo.GitHubRepositoryID,
		Name:               repo.Name,
		FullName:           repo.FullName,
		Public:             repo.Public,
		DefaultBranch:      repo.GetDefaultBranchOrMain(),
		HTMLURL:            repo.GetHTMLURL(),
		SSHURL:             repo.SSHURL,
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

// InstallationSummaryData represents a summary of an installation (matches Laravel's InstallationSummaryData)
type InstallationSummaryData struct {
	ID                      string  `json:"id"`
	SourceControlID         string  `json:"sourceControlId"`
	Status                  string  `json:"status"`
	SelectionVersion        uint64  `json:"selection_version"`
	Generation              uint64  `json:"generation"`
	CleanupPending          bool    `json:"cleanup_pending"`
	AccountLogin            string  `json:"accountLogin"`
	AccountType             string  `json:"accountType"`
	AccountAvatarURL        *string `json:"accountAvatarUrl"`
	HTMLURL                 *string `json:"htmlUrl"`
	CreatedAt               *string `json:"createdAt"`
	RepositorySelection     *string `json:"repositorySelection"`
	HasMultipleRepositories bool    `json:"hasMultipleRepositories"`
	RepositoryCount         int     `json:"repositoryCount"`
}

// InstallationSummaryFromSourceControl creates an InstallationSummaryData from a SourceControl
func InstallationSummaryFromSourceControl(sc *models.SourceControl) InstallationSummaryData {
	repositoryCount := 0
	if sc.Status == "active" {
		for _, r := range sc.Repositories {
			if r.Selected && r.Available {
				repositoryCount++
			}
		}
	}

	summary := InstallationSummaryData{
		SourceControlID: sc.ID, Status: sc.Status, Generation: sc.Generation, SelectionVersion: sc.SelectionVersion, CleanupPending: sc.CleanupPending,
		HasMultipleRepositories: sc.HasMultipleRepositories,
		RepositoryCount:         repositoryCount,
	}

	// Use installation_id as the id (matches Laravel's behavior)
	if sc.InstallationID != nil && *sc.InstallationID != "" {
		summary.ID = *sc.InstallationID
	} else {
		summary.ID = sc.ProviderID
	}

	if sc.Login != nil {
		summary.AccountLogin = *sc.Login
	} else {
		summary.AccountLogin = "Unknown"
	}

	if sc.Type != nil {
		summary.AccountType = *sc.Type
	} else {
		summary.AccountType = "User"
	}

	if sc.AvatarURL != nil {
		summary.AccountAvatarURL = sc.AvatarURL
	}

	if sc.HTMLURL != nil {
		summary.HTMLURL = sc.HTMLURL
	}

	summary.CreatedAt = pkgdto.FormatTime(sc.ConnectedAt)

	if sc.RepositorySelection != nil {
		summary.RepositorySelection = sc.RepositorySelection
	}

	return summary
}
