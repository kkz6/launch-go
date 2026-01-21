package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/git/contracts"
	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
)

// SourceControlService handles git-related business logic
type SourceControlService struct {
	*BaseService
}

// NewSourceControlService creates a new source control service
func NewSourceControlService(deps *ServiceDeps) *SourceControlService {
	return &SourceControlService{
		BaseService: NewBaseService(deps),
	}
}

// ListSourceControls lists all source controls for a team
func (s *SourceControlService) ListSourceControls(ctx context.Context, teamID string) ([]models.SourceControl, error) {
	return s.Repos().SourceControl().FindAllByTeam(ctx, teamID)
}

// GetSourceControl gets a source control by ID
func (s *SourceControlService) GetSourceControl(ctx context.Context, id, teamID string) (*models.SourceControl, error) {
	return s.Repos().SourceControl().FindByIDAndTeam(ctx, id, teamID)
}

// Connect connects a git provider using an installation ID
func (s *SourceControlService) Connect(ctx context.Context, userID, teamID string, providerType enums.GitProviderType, installationID string) (*models.SourceControl, error) {
	provider, err := s.ProviderFactory().GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return nil, err
	}

	// Get installation details from provider
	providerInstallation, err := provider.GetInstallation(ctx, installationID)
	if err != nil {
		return nil, err
	}

	// Convert to local type
	installation := fromProviderAppInstallationData(providerInstallation)

	now := time.Now()
	repoSelection := installation.RepositorySelection

	// Serialize ProviderData and Permissions to JSON strings
	providerDataJSON, _ := json.Marshal(installation.ToMap())
	providerDataStr := string(providerDataJSON)
	permissionsJSON, _ := json.Marshal(installation.Permissions)
	permissionsStr := string(permissionsJSON)
	initialRepoCount := 0

	sc := &models.SourceControl{
		Provider:                providerType,
		ProviderID:              installationID,
		ProviderData:            &providerDataStr,
		ProviderAccountID:       &installation.AccountID,
		Login:                   &installation.AccountLogin,
		Name:                    &installation.AccountLogin,
		Type:                    &installation.AccountType,
		AvatarURL:               &installation.AccountAvatarURL,
		HTMLURL:                 &installation.HTMLURL,
		InstallationID:          &installationID,
		Permissions:             &permissionsStr,
		RepositorySelection:     &repoSelection,
		HasMultipleRepositories: installation.HasMultipleRepositories,
		RepositoryCount:         &initialRepoCount,
		ConnectedAt:             &now,
		LastSyncedAt:            &now,
	}
	sc.UserID = userID
	sc.TeamID = teamID

	// Check if already exists and update, or create new
	existing, err := s.Repos().SourceControl().FindByProviderAndInstallationAndTeam(ctx, providerType, installationID, teamID)
	if err == nil {
		// Update existing
		existing.UserID = userID
		existing.ProviderData = sc.ProviderData
		existing.ProviderAccountID = sc.ProviderAccountID
		existing.Login = sc.Login
		existing.Name = sc.Name
		existing.Type = sc.Type
		existing.AvatarURL = sc.AvatarURL
		existing.HTMLURL = sc.HTMLURL
		existing.Permissions = sc.Permissions
		existing.RepositorySelection = sc.RepositorySelection
		existing.HasMultipleRepositories = sc.HasMultipleRepositories
		nowTime := time.Now()
		existing.LastSyncedAt = &nowTime

		if err := s.Repos().SourceControl().Update(ctx, existing); err != nil {
			return nil, err
		}

		sc = existing
	} else if err == repositories.ErrSourceControlNotFound {
		// Create new
		if err := s.Repos().SourceControl().Create(ctx, sc); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	// Sync repositories in the background
	go func() {
		bgCtx := context.Background()
		if err := s.SyncRepositories(bgCtx, sc); err != nil {
			s.Logger.Warn().Err(err).Str("source_control_id", sc.ID).Msg("Failed to sync repositories during connection")
		}
	}()

	return sc, nil
}

// Disconnect disconnects a source control
func (s *SourceControlService) Disconnect(ctx context.Context, id, teamID string) error {
	sc, err := s.Repos().SourceControl().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	// TODO: Check if there are associated sites
	// if hasSites {
	//     return ErrHasSites
	// }

	// Delete repositories first
	if err := s.Repos().SourceControlRepo().DeleteRepositoriesBySourceControlID(ctx, sc.ID); err != nil {
		return err
	}

	return s.Repos().SourceControl().Delete(ctx, sc.ID)
}

// GetInstallationURL gets the installation URL for a provider
func (s *SourceControlService) GetInstallationURL(providerType enums.GitProviderType) (string, error) {
	provider, err := s.ProviderFactory().GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return "", err
	}

	return provider.GetInstallationURL()
}

// GetInstallations gets all installations for a provider
func (s *SourceControlService) GetInstallations(ctx context.Context, providerType enums.GitProviderType) ([]dto.AppInstallationData, error) {
	provider, err := s.ProviderFactory().GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return nil, err
	}

	providerInstallations, err := provider.GetAllInstallations(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to local types
	installations := make([]dto.AppInstallationData, len(providerInstallations))
	for i, inst := range providerInstallations {
		installations[i] = *fromProviderAppInstallationData(&inst)
	}

	return installations, nil
}

// GetInstallationRepositoriesFromAPI gets repositories from the provider API
func (s *SourceControlService) GetInstallationRepositoriesFromAPI(ctx context.Context, providerType enums.GitProviderType, installationID string) ([]map[string]interface{}, error) {
	provider, err := s.ProviderFactory().GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return nil, err
	}

	return provider.GetInstallationRepositories(ctx, installationID)
}

// GetCachedRepositories gets repositories from the database cache
func (s *SourceControlService) GetCachedRepositories(ctx context.Context, providerType enums.GitProviderType, installationID, teamID string) ([]models.SourceControlRepository, error) {
	return s.Repos().SourceControlRepo().GetInstallationRepositories(ctx, providerType, installationID, teamID)
}

// SyncRepositories syncs repositories for a source control
func (s *SourceControlService) SyncRepositories(ctx context.Context, sc *models.SourceControl) error {
	if sc.InstallationID == nil || *sc.InstallationID == "" {
		return ErrNoInstallationID
	}

	provider, err := s.ProviderFactory().GetProvider(providers.GitProviderType(sc.Provider))
	if err != nil {
		return err
	}

	repos, err := provider.GetInstallationRepositories(ctx, *sc.InstallationID)
	if err != nil {
		return err
	}

	return s.SyncInstallationRepositories(ctx, sc, repos)
}

// SyncInstallationRepositories syncs repositories for an installation
func (s *SourceControlService) SyncInstallationRepositories(ctx context.Context, sc *models.SourceControl, repos []map[string]interface{}) error {
	// Get existing repositories
	existingRepos, err := s.Repos().SourceControlRepo().FindRepositoriesBySourceControlID(ctx, sc.ID)
	if err != nil {
		return err
	}

	// Build map of API repository IDs
	apiRepoIDs := make(map[string]bool)
	for _, repo := range repos {
		var id string
		if idFloat, ok := repo["id"].(float64); ok {
			id = fmt.Sprintf("%.0f", idFloat)
		} else if idStr, ok := repo["id"].(string); ok {
			id = idStr
		}
		if id != "" {
			apiRepoIDs[id] = true
		}
	}

	// Find repositories to delete (not in API response)
	var reposToDelete []string
	existingRepoIDs := make(map[string]bool)
	for _, repo := range existingRepos {
		if repo.AdditionalData != nil && *repo.AdditionalData != "" {
			var additionalData map[string]interface{}
			if err := json.Unmarshal([]byte(*repo.AdditionalData), &additionalData); err == nil {
				if id, ok := additionalData["id"]; ok {
					var idStr string
					if idFloat, ok := id.(float64); ok {
						idStr = fmt.Sprintf("%.0f", idFloat)
					} else if str, ok := id.(string); ok {
						idStr = str
					}
					if idStr != "" {
						existingRepoIDs[idStr] = true
						if !apiRepoIDs[idStr] {
							// TODO: Check if repo has associated sites before deleting
							reposToDelete = append(reposToDelete, fmt.Sprintf("%d", repo.ID))
						}
					}
				}
			}
		}
	}

	// Delete repositories not in API response
	if len(reposToDelete) > 0 {
		if err := s.Repos().SourceControlRepo().DeleteRepositoriesByIDs(ctx, reposToDelete); err != nil {
			return err
		}
	}

	// Create or update repositories from API response
	for _, repoData := range repos {
		var id string
		if idFloat, ok := repoData["id"].(float64); ok {
			id = fmt.Sprintf("%.0f", idFloat)
		} else if idStr, ok := repoData["id"].(string); ok {
			id = idStr
		}

		// Skip if already exists
		if existingRepoIDs[id] {
			continue
		}

		data := dto.RepositoryDataFromAPIResponse(repoData)
		if _, err := s.Repos().SourceControlRepo().UpsertRepository(ctx, sc.ID, data); err != nil {
			s.Logger.Warn().Err(err).Str("repo", data.FullName).Msg("Failed to upsert repository")
			continue
		}
	}

	// Update repository count
	now := time.Now()
	return s.Repos().SourceControl().UpdateFields(ctx, sc.ID, map[string]interface{}{
		"repository_count": len(repos),
		"last_synced_at":   now,
	})
}

// RefreshInstallationRepositories refreshes repositories for an installation
func (s *SourceControlService) RefreshInstallationRepositories(ctx context.Context, sc *models.SourceControl) error {
	return s.SyncRepositories(ctx, sc)
}

// SyncUserInstallation syncs a user's installation
func (s *SourceControlService) SyncUserInstallation(ctx context.Context, providerType enums.GitProviderType, installationID, teamID, userID string) error {
	provider, err := s.ProviderFactory().GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return err
	}

	providerInstallation, err := provider.GetInstallation(ctx, installationID)
	if err != nil {
		return err
	}

	// Convert to local type
	installation := fromProviderAppInstallationData(providerInstallation)

	now := time.Now()
	repoSelection := installation.RepositorySelection

	providerData := map[string]interface{}{
		"installation_id":      installation.ID,
		"installed_by_user_id": userID,
		"installed_at":         now.Format(time.RFC3339),
		"app_id":               installation.AppID,
		"app_slug":             installation.AppSlug,
		"target_type":          installation.TargetType,
		"events":               installation.Events,
		"created_at":           installation.CreatedAt,
		"updated_at":           installation.UpdatedAt,
		"suspended_at":         installation.SuspendedAt,
		"single_file_name":     installation.SingleFileName,
		"account": map[string]interface{}{
			"id":         installation.AccountID,
			"login":      installation.AccountLogin,
			"type":       installation.AccountType,
			"avatar_url": installation.AccountAvatarURL,
		},
	}

	// Serialize to JSON strings
	providerDataJSON, _ := json.Marshal(providerData)
	providerDataStr := string(providerDataJSON)
	permissionsJSON, _ := json.Marshal(installation.Permissions)
	permissionsStr := string(permissionsJSON)

	sc := &models.SourceControl{
		Provider:                providerType,
		ProviderID:              installation.ID,
		ProviderAccountID:       &installation.AccountID,
		Login:                   &installation.AccountLogin,
		Name:                    &installation.AccountLogin,
		Type:                    &installation.AccountType,
		AvatarURL:               &installation.AccountAvatarURL,
		HTMLURL:                 &installation.HTMLURL,
		InstallationID:          &installation.ID,
		Permissions:             &permissionsStr,
		RepositorySelection:     &repoSelection,
		HasMultipleRepositories: installation.HasMultipleRepositories,
		ProviderData:            &providerDataStr,
		ConnectedAt:             &now,
		LastSyncedAt:            &now,
	}
	sc.TeamID = teamID
	sc.UserID = userID

	if err := s.Repos().SourceControl().Create(ctx, sc); err != nil {
		return err
	}

	// Sync repositories in the background
	go func() {
		bgCtx := context.Background()
		if err := s.SyncRepositories(bgCtx, sc); err != nil {
			s.Logger.Warn().Err(err).Str("source_control_id", sc.ID).Msg("Failed to sync repositories during installation sync")
		}
	}()

	return nil
}

// SyncRepositoriesForInstallation syncs repositories for all teams with the installation (webhook handler)
func (s *SourceControlService) SyncRepositoriesForInstallation(ctx context.Context, installationID string) error {
	sourceControls, err := s.Repos().SourceControl().FindByInstallationID(ctx, installationID)
	if err != nil {
		return err
	}

	for _, sc := range sourceControls {
		if err := s.SyncRepositories(ctx, &sc); err != nil {
			s.Logger.Warn().Err(err).Str("source_control_id", sc.ID).Msg("Failed to sync repositories for installation")
		}
	}

	return nil
}

// GetInstallationsWithRepositoryCounts gets installations with their repository counts
func (s *SourceControlService) GetInstallationsWithRepositoryCounts(ctx context.Context, teamID, userID string) (map[string][]dto.InstallationSummaryData, error) {
	result := make(map[string][]dto.InstallationSummaryData)

	for _, providerType := range enums.AllGitProviders() {
		sourceControls, err := s.Repos().SourceControl().GetInstallations(ctx, providerType, contracts.WithUserID(userID), contracts.RequireInstallationID())
		if err != nil {
			continue
		}

		summaries := make([]dto.InstallationSummaryData, len(sourceControls))
		for i, sc := range sourceControls {
			summaries[i] = dto.InstallationSummaryFromSourceControl(&sc)
		}

		result[providerType.String()] = summaries
	}

	return result, nil
}

// DeleteByInstallationID deletes all source controls by installation ID (webhook handler)
func (s *SourceControlService) DeleteByInstallationID(ctx context.Context, installationID string) error {
	_, err := s.Repos().SourceControl().DeleteByInstallationID(ctx, installationID)
	return err
}

// GetSourceControlByInstallation finds a source control by provider and installation
func (s *SourceControlService) GetSourceControlByInstallation(ctx context.Context, providerType enums.GitProviderType, installationID string, opts ...contracts.InstallationQueryOption) (*models.SourceControl, error) {
	opts = append(opts, contracts.WithProviderID(installationID))
	return s.Repos().SourceControl().GetFirstInstallation(ctx, providerType, opts...)
}

// TestConnection tests the connection to a provider
func (s *SourceControlService) TestConnection(ctx context.Context, providerType enums.GitProviderType) error {
	provider, err := s.ProviderFactory().GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return err
	}

	return provider.TestConnection(ctx)
}

// GetSourceControlRepo returns the source control repository for webhook handlers
func (s *SourceControlService) GetSourceControlRepo() *repositories.SourceControlRepository {
	return s.Repos().SourceControl()
}

// GetRepositoriesBySourceControlID gets all repositories for a source control
func (s *SourceControlService) GetRepositoriesBySourceControlID(ctx context.Context, sourceControlID, teamID string) ([]models.SourceControlRepository, error) {
	// Verify the source control belongs to the team
	if _, err := s.Repos().SourceControl().FindByIDAndTeam(ctx, sourceControlID, teamID); err != nil {
		return nil, err
	}

	return s.Repos().SourceControlRepo().FindRepositoriesBySourceControlID(ctx, sourceControlID)
}

// SaveRepository fetches a repository from the git provider and saves it to the database
func (s *SourceControlService) SaveRepository(ctx context.Context, sourceControlID, repoFullName string) (*models.SourceControlRepository, error) {
	// Get the source control
	sc, err := s.Repos().SourceControl().FindByID(ctx, sourceControlID)
	if err != nil {
		return nil, fmt.Errorf("failed to find source control: %w", err)
	}

	if sc.InstallationID == nil || *sc.InstallationID == "" {
		return nil, ErrNoInstallationID
	}

	// Get the provider
	provider, err := s.ProviderFactory().GetProvider(providers.GitProviderType(sc.Provider))
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	// Parse owner and repo from full_name (e.g., "owner/repo")
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository full name: %s", repoFullName)
	}
	owner, repoName := parts[0], parts[1]

	// Fetch repository from provider
	repoData, err := provider.GetRepository(ctx, *sc.InstallationID, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository from provider: %w", err)
	}

	// Convert to RepositoryData
	data := dto.RepositoryDataFromAPIResponse(repoData)

	// Upsert the repository
	repo, err := s.Repos().SourceControlRepo().UpsertRepository(ctx, sourceControlID, data)
	if err != nil {
		return nil, fmt.Errorf("failed to save repository: %w", err)
	}

	return repo, nil
}

// fromProviderAppInstallationData converts providers.AppInstallationData to dto.AppInstallationData
func fromProviderAppInstallationData(p *providers.AppInstallationData) *dto.AppInstallationData {
	if p == nil {
		return nil
	}

	return &dto.AppInstallationData{
		ID:                      p.ID,
		AccountID:               p.AccountID,
		AccountLogin:            p.AccountLogin,
		AccountType:             p.AccountType,
		AccountAvatarURL:        p.AccountAvatarURL,
		Permissions:             p.Permissions,
		Repositories:            p.Repositories,
		TargetType:              p.TargetType,
		HTMLURL:                 p.HTMLURL,
		CreatedAt:               p.CreatedAt,
		UpdatedAt:               p.UpdatedAt,
		SuspendedAt:             p.SuspendedAt,
		Events:                  p.Events,
		SingleFileName:          p.SingleFileName,
		HasMultipleRepositories: p.HasMultipleRepositories,
		RepositorySelection:     p.RepositorySelection,
		AppSlug:                 p.AppSlug,
		AppID:                   p.AppID,
	}
}
