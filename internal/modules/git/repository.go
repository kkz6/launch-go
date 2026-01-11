package git

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrSourceControlNotFound = errors.New("source control not found")
	ErrRepositoryNotFound    = errors.New("repository not found")
)

// Repository handles database operations for git-related entities
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new git repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create creates a new source control record
func (r *Repository) Create(ctx context.Context, sc *SourceControl) error {
	return r.db.WithContext(ctx).Create(sc).Error
}

// Update updates an existing source control record
func (r *Repository) Update(ctx context.Context, sc *SourceControl) error {
	return r.db.WithContext(ctx).Save(sc).Error
}

// UpdateFields updates specific fields of a source control record
func (r *Repository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&SourceControl{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete soft-deletes a source control record
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&SourceControl{}, "id = ?", id).Error
}

// FindByID finds a source control by ID
func (r *Repository) FindByID(ctx context.Context, id string) (*SourceControl, error) {
	var sc SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		First(&sc, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSourceControlNotFound
	}

	return &sc, err
}

// FindByIDAndTeam finds a source control by ID and team ID
func (r *Repository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*SourceControl, error) {
	var sc SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		First(&sc, "id = ? AND team_id = ?", id, teamID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSourceControlNotFound
	}

	return &sc, err
}

// FindAllByTeam finds all source controls for a team
func (r *Repository) FindAllByTeam(ctx context.Context, teamID string) ([]SourceControl, error) {
	var sourceControls []SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&sourceControls).Error

	return sourceControls, err
}

// FindAllByUser finds all source controls for a user
func (r *Repository) FindAllByUser(ctx context.Context, userID string) ([]SourceControl, error) {
	var sourceControls []SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&sourceControls).Error

	return sourceControls, err
}

// FindByProvider finds all source controls for a specific provider
func (r *Repository) FindByProvider(ctx context.Context, provider GitProviderType) ([]SourceControl, error) {
	var sourceControls []SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ?", provider).
		Find(&sourceControls).Error

	return sourceControls, err
}

// FindByTeamAndProvider finds source controls for a team and provider
func (r *Repository) FindByTeamAndProvider(ctx context.Context, teamID string, provider GitProviderType) ([]SourceControl, error) {
	var sourceControls []SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("team_id = ? AND provider = ? AND installation_id IS NOT NULL", teamID, provider).
		Find(&sourceControls).Error

	return sourceControls, err
}

// FindByProviderAndInstallationAndTeam finds a source control by provider, installation ID, and team
func (r *Repository) FindByProviderAndInstallationAndTeam(
	ctx context.Context,
	provider GitProviderType,
	installationID string,
	teamID string,
) (*SourceControl, error) {
	var sc SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ? AND provider_id = ? AND team_id = ?", provider, installationID, teamID).
		First(&sc).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSourceControlNotFound
	}

	return &sc, err
}

// FirstOrCreateByProviderAndInstallationAndTeam finds or creates a source control
func (r *Repository) FirstOrCreateByProviderAndInstallationAndTeam(
	ctx context.Context,
	provider GitProviderType,
	installationID string,
	teamID string,
	defaults map[string]interface{},
) (*SourceControl, bool, error) {
	var sc SourceControl
	err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_id = ? AND team_id = ?", provider, installationID, teamID).
		First(&sc).Error

	if err == nil {
		return &sc, false, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	// Create new record
	sc = SourceControl{
		Provider:   provider,
		ProviderID: &installationID,
		TeamID:     teamID,
	}

	// Apply defaults
	if userID, ok := defaults["user_id"].(string); ok {
		sc.UserID = userID
	}

	if providerData, ok := defaults["provider_data"].(JSONMap); ok {
		sc.ProviderData = providerData
	}

	if err := r.db.WithContext(ctx).Create(&sc).Error; err != nil {
		return nil, false, err
	}

	return &sc, true, nil
}

// FindByInstallationID finds all source controls by installation ID
func (r *Repository) FindByInstallationID(ctx context.Context, installationID string) ([]SourceControl, error) {
	var sourceControls []SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("provider_id = ?", installationID).
		Find(&sourceControls).Error

	return sourceControls, err
}

// DeleteByInstallationID deletes all source controls and their repositories by installation ID
func (r *Repository) DeleteByInstallationID(ctx context.Context, installationID string) (int64, error) {
	// First, get all source control IDs
	var sourceControls []SourceControl
	if err := r.db.WithContext(ctx).
		Select("id").
		Where("provider_id = ?", installationID).
		Find(&sourceControls).Error; err != nil {
		return 0, err
	}

	if len(sourceControls) == 0 {
		return 0, nil
	}

	ids := make([]string, len(sourceControls))
	for i, sc := range sourceControls {
		ids[i] = sc.ID
	}

	// Delete repositories
	if err := r.db.WithContext(ctx).
		Where("source_control_id IN ?", ids).
		Delete(&SourceControlRepository{}).Error; err != nil {
		return 0, err
	}

	// Delete source controls
	result := r.db.WithContext(ctx).
		Where("provider_id = ?", installationID).
		Delete(&SourceControl{})

	return result.RowsAffected, result.Error
}

// GetInstallations gets installations with flexible filtering
func (r *Repository) GetInstallations(
	ctx context.Context,
	provider GitProviderType,
	opts ...InstallationQueryOption,
) ([]SourceControl, error) {
	query := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ?", provider)

	options := &installationQueryOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.userID != "" {
		query = query.Where("user_id = ?", options.userID)
	}

	if options.providerID != "" {
		query = query.Where("provider_id = ?", options.providerID)
	}

	if options.requireInstallationID {
		query = query.Where("installation_id IS NOT NULL")
	}

	var sourceControls []SourceControl
	err := query.Find(&sourceControls).Error

	return sourceControls, err
}

// GetFirstInstallation gets the first installation matching criteria
func (r *Repository) GetFirstInstallation(
	ctx context.Context,
	provider GitProviderType,
	opts ...InstallationQueryOption,
) (*SourceControl, error) {
	query := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ?", provider)

	options := &installationQueryOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.userID != "" {
		query = query.Where("user_id = ?", options.userID)
	}

	if options.providerID != "" {
		query = query.Where("provider_id = ?", options.providerID)
	}

	if options.requireInstallationID {
		query = query.Where("installation_id IS NOT NULL")
	}

	var sc SourceControl
	err := query.First(&sc).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSourceControlNotFound
	}

	return &sc, err
}

// installationQueryOptions holds options for installation queries
type installationQueryOptions struct {
	userID                string
	providerID            string
	requireInstallationID bool
}

// InstallationQueryOption is a functional option for installation queries
type InstallationQueryOption func(*installationQueryOptions)

// WithUserID filters installations by user ID
func WithUserID(userID string) InstallationQueryOption {
	return func(o *installationQueryOptions) {
		o.userID = userID
	}
}

// WithProviderID filters installations by provider ID
func WithProviderID(providerID string) InstallationQueryOption {
	return func(o *installationQueryOptions) {
		o.providerID = providerID
	}
}

// RequireInstallationID requires that installation_id is not null
func RequireInstallationID() InstallationQueryOption {
	return func(o *installationQueryOptions) {
		o.requireInstallationID = true
	}
}

// Repository methods for SourceControlRepository

// CreateRepository creates a new source control repository
func (r *Repository) CreateRepository(ctx context.Context, repo *SourceControlRepository) error {
	return r.db.WithContext(ctx).Create(repo).Error
}

// UpdateRepository updates an existing repository
func (r *Repository) UpdateRepository(ctx context.Context, repo *SourceControlRepository) error {
	return r.db.WithContext(ctx).Save(repo).Error
}

// DeleteRepository deletes a repository
func (r *Repository) DeleteRepository(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&SourceControlRepository{}, "id = ?", id).Error
}

// DeleteRepositoriesBySourceControlID deletes all repositories for a source control
func (r *Repository) DeleteRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) error {
	return r.db.WithContext(ctx).
		Where("source_control_id = ?", sourceControlID).
		Delete(&SourceControlRepository{}).Error
}

// DeleteRepositoriesByIDs deletes repositories by their IDs
func (r *Repository) DeleteRepositoriesByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Delete(&SourceControlRepository{}).Error
}

// FindRepositoryByID finds a repository by ID
func (r *Repository) FindRepositoryByID(ctx context.Context, id string) (*SourceControlRepository, error) {
	var repo SourceControlRepository
	err := r.db.WithContext(ctx).
		Preload("SourceControl").
		First(&repo, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRepositoryNotFound
	}

	return &repo, err
}

// FindRepositoryByFullName finds a repository by its full name
func (r *Repository) FindRepositoryByFullName(ctx context.Context, fullName string) (*SourceControlRepository, error) {
	var repo SourceControlRepository
	err := r.db.WithContext(ctx).
		Preload("SourceControl").
		First(&repo, "full_name = ?", fullName).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRepositoryNotFound
	}

	return &repo, err
}

// FindRepositoriesBySourceControlID finds all repositories for a source control
func (r *Repository) FindRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) ([]SourceControlRepository, error) {
	var repos []SourceControlRepository
	err := r.db.WithContext(ctx).
		Where("source_control_id = ?", sourceControlID).
		Order("full_name ASC").
		Find(&repos).Error

	return repos, err
}

// FindPublicRepositories finds all public repositories
func (r *Repository) FindPublicRepositories(ctx context.Context) ([]SourceControlRepository, error) {
	var repos []SourceControlRepository
	err := r.db.WithContext(ctx).
		Preload("SourceControl").
		Where("public = ?", true).
		Find(&repos).Error

	return repos, err
}

// GetInstallationRepositories gets repositories for a specific installation
func (r *Repository) GetInstallationRepositories(
	ctx context.Context,
	provider GitProviderType,
	installationID string,
	teamID string,
) ([]SourceControlRepository, error) {
	var sourceControls []SourceControl
	err := r.db.WithContext(ctx).
		Preload("Repositories").
		Where("provider = ? AND provider_id = ? AND team_id = ?", provider, installationID, teamID).
		Find(&sourceControls).Error

	if err != nil {
		return nil, err
	}

	var repos []SourceControlRepository
	for _, sc := range sourceControls {
		repos = append(repos, sc.Repositories...)
	}

	return repos, nil
}

// UpsertRepository creates or updates a repository
func (r *Repository) UpsertRepository(ctx context.Context, sourceControlID string, data *RepositoryData) (*SourceControlRepository, error) {
	var repo SourceControlRepository
	err := r.db.WithContext(ctx).
		Where("source_control_id = ? AND full_name = ?", sourceControlID, data.FullName).
		First(&repo).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new
		repo = SourceControlRepository{
			SourceControlID: sourceControlID,
			Name:            data.Name,
			FullName:        data.FullName,
			Public:          data.IsPublic,
			DefaultBranch:   &data.DefaultBranch,
			HTMLURL:         &data.HTMLURL,
			SSHURL:          &data.SSHURL,
			AdditionalData:  data.AdditionalData,
		}

		if err := r.db.WithContext(ctx).Create(&repo).Error; err != nil {
			return nil, err
		}

		return &repo, nil
	}

	if err != nil {
		return nil, err
	}

	// Update existing
	repo.Name = data.Name
	repo.Public = data.IsPublic
	repo.DefaultBranch = &data.DefaultBranch
	repo.HTMLURL = &data.HTMLURL
	repo.SSHURL = &data.SSHURL
	repo.AdditionalData = data.AdditionalData

	if err := r.db.WithContext(ctx).Save(&repo).Error; err != nil {
		return nil, err
	}

	return &repo, nil
}

// CountRepositoriesBySourceControlID counts repositories for a source control
func (r *Repository) CountRepositoriesBySourceControlID(ctx context.Context, sourceControlID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&SourceControlRepository{}).
		Where("source_control_id = ?", sourceControlID).
		Count(&count).Error

	return count, err
}
