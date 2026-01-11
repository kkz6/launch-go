package git

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/queue"
)

var (
	ErrProviderNotSupported = errors.New("provider not supported")
	ErrNoInstallationID     = errors.New("no installation ID found")
	ErrHasSites             = errors.New("cannot delete source control with associated sites")
)

// Service handles git-related business logic
type Service struct {
	repo            *Repository
	providerFactory *providers.ProviderFactory
	queue           *queue.Client
	logger          *zerolog.Logger
}

// NewService creates a new git service
func NewService(
	repo *Repository,
	providerFactory *providers.ProviderFactory,
	queue *queue.Client,
	logger *zerolog.Logger,
) *Service {
	return &Service{
		repo:            repo,
		providerFactory: providerFactory,
		queue:           queue,
		logger:          logger,
	}
}

// ListSourceControls lists all source controls for a team
func (s *Service) ListSourceControls(ctx context.Context, teamID string) ([]SourceControl, error) {
	return s.repo.FindAllByTeam(ctx, teamID)
}

// GetSourceControl gets a source control by ID
func (s *Service) GetSourceControl(ctx context.Context, id, teamID string) (*SourceControl, error) {
	return s.repo.FindByIDAndTeam(ctx, id, teamID)
}

// Connect connects a git provider using an installation ID
func (s *Service) Connect(ctx context.Context, userID, teamID string, providerType GitProviderType, installationID string) (*SourceControl, error) {
	provider, err := s.providerFactory.GetProvider(providers.GitProviderType(providerType))
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

	sc := &SourceControl{
		UserID:                  userID,
		TeamID:                  teamID,
		Provider:                providerType,
		ProviderID:              &installationID,
		ProviderData:            installation.ToMap(),
		ProviderAccountID:       &installation.AccountID,
		Login:                   &installation.AccountLogin,
		Name:                    &installation.AccountLogin,
		Type:                    &installation.AccountType,
		AvatarURL:               &installation.AccountAvatarURL,
		HTMLURL:                 &installation.HTMLURL,
		InstallationID:          &installationID,
		Permissions:             installation.Permissions,
		RepositorySelection:     &repoSelection,
		HasMultipleRepositories: installation.HasMultipleRepositories,
		RepositoryCount:         0,
		ConnectedAt:             &now,
		LastSyncedAt:            &now,
	}

	// Check if already exists and update, or create new
	existing, err := s.repo.FindByProviderAndInstallationAndTeam(ctx, providerType, installationID, teamID)
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
		existing.LastSyncedAt = &now

		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}

		sc = existing
	} else if errors.Is(err, ErrSourceControlNotFound) {
		// Create new
		if err := s.repo.Create(ctx, sc); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	// Sync repositories in the background
	go func() {
		bgCtx := context.Background()
		if err := s.SyncRepositories(bgCtx, sc); err != nil {
			s.logger.Warn().Err(err).Str("source_control_id", sc.ID).Msg("Failed to sync repositories during connection")
		}
	}()

	return sc, nil
}

// Disconnect disconnects a source control
func (s *Service) Disconnect(ctx context.Context, id, teamID string) error {
	sc, err := s.repo.FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	// TODO: Check if there are associated sites
	// if hasSites {
	//     return ErrHasSites
	// }

	// Delete repositories first
	if err := s.repo.DeleteRepositoriesBySourceControlID(ctx, sc.ID); err != nil {
		return err
	}

	return s.repo.Delete(ctx, sc.ID)
}

// GetInstallationURL gets the installation URL for a provider
func (s *Service) GetInstallationURL(providerType GitProviderType) (string, error) {
	provider, err := s.providerFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return "", err
	}

	return provider.GetInstallationURL()
}

// GetInstallations gets all installations for a provider
func (s *Service) GetInstallations(ctx context.Context, providerType GitProviderType) ([]AppInstallationData, error) {
	provider, err := s.providerFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return nil, err
	}

	providerInstallations, err := provider.GetAllInstallations(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to local types
	installations := make([]AppInstallationData, len(providerInstallations))
	for i, inst := range providerInstallations {
		installations[i] = *fromProviderAppInstallationData(&inst)
	}

	return installations, nil
}

// GetInstallationRepositoriesFromAPI gets repositories from the provider API
func (s *Service) GetInstallationRepositoriesFromAPI(ctx context.Context, providerType GitProviderType, installationID string) ([]map[string]interface{}, error) {
	provider, err := s.providerFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return nil, err
	}

	return provider.GetInstallationRepositories(ctx, installationID)
}

// GetCachedRepositories gets repositories from the database cache
func (s *Service) GetCachedRepositories(ctx context.Context, providerType GitProviderType, installationID, teamID string) ([]SourceControlRepository, error) {
	return s.repo.GetInstallationRepositories(ctx, providerType, installationID, teamID)
}

// SyncRepositories syncs repositories for a source control
func (s *Service) SyncRepositories(ctx context.Context, sc *SourceControl) error {
	if sc.InstallationID == nil || *sc.InstallationID == "" {
		return ErrNoInstallationID
	}

	provider, err := s.providerFactory.GetProvider(providers.GitProviderType(sc.Provider))
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
func (s *Service) SyncInstallationRepositories(ctx context.Context, sc *SourceControl, repositories []map[string]interface{}) error {
	// Get existing repositories
	existingRepos, err := s.repo.FindRepositoriesBySourceControlID(ctx, sc.ID)
	if err != nil {
		return err
	}

	// Build map of API repository IDs
	apiRepoIDs := make(map[string]bool)
	for _, repo := range repositories {
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
		if repo.AdditionalData != nil {
			if id, ok := repo.AdditionalData["id"]; ok {
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
						reposToDelete = append(reposToDelete, repo.ID)
					}
				}
			}
		}
	}

	// Delete repositories not in API response
	if len(reposToDelete) > 0 {
		if err := s.repo.DeleteRepositoriesByIDs(ctx, reposToDelete); err != nil {
			return err
		}
	}

	// Create or update repositories from API response
	for _, repoData := range repositories {
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

		data := RepositoryDataFromAPIResponse(repoData)
		if _, err := s.repo.UpsertRepository(ctx, sc.ID, data); err != nil {
			s.logger.Warn().Err(err).Str("repo", data.FullName).Msg("Failed to upsert repository")
			continue
		}
	}

	// Update repository count
	now := time.Now()
	if err := s.repo.UpdateFields(ctx, sc.ID, map[string]interface{}{
		"repository_count": len(repositories),
		"last_synced_at":   now,
	}); err != nil {
		return err
	}

	return nil
}

// RefreshInstallationRepositories refreshes repositories for an installation
func (s *Service) RefreshInstallationRepositories(ctx context.Context, sc *SourceControl) error {
	return s.SyncRepositories(ctx, sc)
}

// SyncUserInstallation syncs a user's installation
func (s *Service) SyncUserInstallation(ctx context.Context, providerType GitProviderType, installationID, teamID, userID string) error {
	provider, err := s.providerFactory.GetProvider(providers.GitProviderType(providerType))
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

	sc := &SourceControl{
		TeamID:                  teamID,
		UserID:                  userID,
		Provider:                providerType,
		ProviderID:              &installation.ID,
		ProviderAccountID:       &installation.AccountID,
		Login:                   &installation.AccountLogin,
		Name:                    &installation.AccountLogin,
		Type:                    &installation.AccountType,
		AvatarURL:               &installation.AccountAvatarURL,
		HTMLURL:                 &installation.HTMLURL,
		InstallationID:          &installation.ID,
		Permissions:             installation.Permissions,
		RepositorySelection:     &repoSelection,
		HasMultipleRepositories: installation.HasMultipleRepositories,
		ProviderData:            providerData,
		ConnectedAt:             &now,
		LastSyncedAt:            &now,
	}

	if err := s.repo.Create(ctx, sc); err != nil {
		return err
	}

	// Sync repositories in the background
	go func() {
		bgCtx := context.Background()
		if err := s.SyncRepositories(bgCtx, sc); err != nil {
			s.logger.Warn().Err(err).Str("source_control_id", sc.ID).Msg("Failed to sync repositories during installation sync")
		}
	}()

	return nil
}

// SyncRepositoriesForInstallation syncs repositories for all teams with the installation (webhook handler)
func (s *Service) SyncRepositoriesForInstallation(ctx context.Context, installationID string) error {
	sourceControls, err := s.repo.FindByInstallationID(ctx, installationID)
	if err != nil {
		return err
	}

	for _, sc := range sourceControls {
		if err := s.SyncRepositories(ctx, &sc); err != nil {
			s.logger.Warn().Err(err).Str("source_control_id", sc.ID).Msg("Failed to sync repositories for installation")
		}
	}

	return nil
}

// GetInstallationsWithRepositoryCounts gets installations with their repository counts
func (s *Service) GetInstallationsWithRepositoryCounts(ctx context.Context, teamID, userID string) (map[string][]InstallationSummaryData, error) {
	result := make(map[string][]InstallationSummaryData)

	for _, providerType := range AllGitProviders() {
		sourceControls, err := s.repo.GetInstallations(ctx, providerType, WithUserID(userID), RequireInstallationID())
		if err != nil {
			continue
		}

		summaries := make([]InstallationSummaryData, len(sourceControls))
		for i, sc := range sourceControls {
			summaries[i] = InstallationSummaryFromSourceControl(&sc)
		}

		result[providerType.String()] = summaries
	}

	return result, nil
}

// DeleteByInstallationID deletes all source controls by installation ID (webhook handler)
func (s *Service) DeleteByInstallationID(ctx context.Context, installationID string) error {
	_, err := s.repo.DeleteByInstallationID(ctx, installationID)
	return err
}

// GetSourceControlByInstallation finds a source control by provider and installation
func (s *Service) GetSourceControlByInstallation(ctx context.Context, providerType GitProviderType, installationID string, opts ...InstallationQueryOption) (*SourceControl, error) {
	opts = append(opts, WithProviderID(installationID))
	return s.repo.GetFirstInstallation(ctx, providerType, opts...)
}

// TestConnection tests the connection to a provider
func (s *Service) TestConnection(ctx context.Context, providerType GitProviderType) error {
	provider, err := s.providerFactory.GetProvider(providers.GitProviderType(providerType))
	if err != nil {
		return err
	}

	return provider.TestConnection(ctx)
}

// fromProviderAppInstallationData converts providers.AppInstallationData to git.AppInstallationData
func fromProviderAppInstallationData(p *providers.AppInstallationData) *AppInstallationData {
	if p == nil {
		return nil
	}
	return &AppInstallationData{
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

// fromProviderCommitData converts providers.CommitData to git.CommitData
func fromProviderCommitData(p *providers.CommitData) *CommitData {
	if p == nil {
		return nil
	}
	return &CommitData{
		CommitID: p.CommitID,
		SHA:      p.SHA,
		Name:     p.Name,
		Email:    p.Email,
		Message:  p.Message,
		URL:      p.URL,
		Branch:   p.Branch,
	}
}
