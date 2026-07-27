package services

import (
	"context"
	"sort"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	authrepos "github.com/kkz6/launch-go/internal/modules/auth/repositories"
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/dashboard/dto"
	dnsrepos "github.com/kkz6/launch-go/internal/modules/dns/repositories"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	notificationrepos "github.com/kkz6/launch-go/internal/modules/notification/repositories"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

const (
	maxServers        = 8
	maxRecentActivity = 6
)

// Repositories holds all repositories needed by the dashboard service
type Repositories struct {
	User                *authrepos.UserRepository
	ServerProvider      *serverrepos.ServerProviderRepository
	SourceControl       *gitrepos.SourceControlRepository
	DomainProvider      *dnsrepos.DomainProviderRepository
	StorageProvider     *backuprepos.StorageProviderRepository
	NotificationChannel *notificationrepos.NotificationChannelRepository
}

// DashboardService handles dashboard data retrieval
type DashboardService struct {
	db     *gorm.DB
	logger *zerolog.Logger
	repos  *Repositories
}

// NewDashboardService creates a new dashboard service
func NewDashboardService(db *gorm.DB, logger *zerolog.Logger, repos *Repositories) *DashboardService {
	return &DashboardService{
		db:     db,
		logger: logger,
		repos:  repos,
	}
}

// GetDashboard returns dashboard data for a team
func (s *DashboardService) GetDashboard(ctx context.Context, teamID string) (*dto.DashboardResponse, error) {
	servers, err := s.getServers(ctx, teamID)
	if err != nil {
		return nil, err
	}

	recentActivity, err := s.getRecentActivity(ctx, teamID)
	if err != nil {
		return nil, err
	}

	return &dto.DashboardResponse{
		Servers:        servers,
		RecentActivity: recentActivity,
	}, nil
}

// ActiveActions combines the active site and Docker deployment tables. They
// intentionally remain separate queries: their status vocabularies and target
// relationships differ, while the result is a small, bounded top-bar list.
func (s *DashboardService) ActiveActions(ctx context.Context, teamID string) ([]dto.ActiveAction, error) {
	const siteStatuses = "('pending', 'installing', 'running')"
	const dockerStatuses = "('pending', 'building', 'deploying', 'running')"

	var actions []dto.ActiveAction
	if err := s.db.WithContext(ctx).Raw(`
		SELECT d.id, 'deployment' AS kind, d.status, sites.address AS label,
		       sites.server_id, '' AS project_id, 'site' AS target_type, d.site_id AS target_id,
		       d.task_id, NULL AS started_at, d.created_at
		FROM deployments d JOIN sites ON sites.id = d.site_id
		WHERE d.team_id = ? AND d.status IN `+siteStatuses+`
	`, teamID).Scan(&actions).Error; err != nil {
		return nil, err
	}

	var dockerActions []dto.ActiveAction
	if err := s.db.WithContext(ctx).Raw(`
		SELECT d.id, 'deployment' AS kind, d.status,
		       COALESCE(app.name, compose.name, d.target_type) AS label,
		       d.server_id, COALESCE(app.project_id, compose.project_id, '') AS project_id,
		       d.target_type, d.target_id, d.task_id, d.started_at, d.created_at
		FROM docker_deployments d
		LEFT JOIN docker_applications app ON d.target_type = 'application' AND app.id = d.target_id
		LEFT JOIN docker_composes compose ON d.target_type = 'compose' AND compose.id = d.target_id
		WHERE d.team_id = ? AND d.status IN `+dockerStatuses+`
	`, teamID).Scan(&dockerActions).Error; err != nil {
		return nil, err
	}
	actions = append(actions, dockerActions...)
	sort.Slice(actions, func(i, j int) bool { return actions[i].CreatedAt.After(actions[j].CreatedAt) })
	return actions, nil
}

// sitesCountSubquery returns a GORM subquery for counting sites per server (database-agnostic)
func (s *DashboardService) sitesCountSubquery() *gorm.DB {
	return s.db.Model(&sitemodels.Site{}).
		Select("COUNT(*)").
		Where("sites.server_id = servers.id")
}

// workloadsCountSubquery counts live docker workloads (applications +
// composes + managed databases) for the server. Docker servers don't
// have rows in the `sites` table, so the existing sites_count is
// always 0 for them — the dashboard card needs this count to show
// something meaningful instead.
//
// We reference tables by name rather than importing docker/models to
// keep the dependency direction one-way (docker → dashboard via
// dataflow, never the reverse). Soft-deleted rows are filtered out
// the same way the docker module's repos do.
func (s *DashboardService) workloadsCountSubquery() *gorm.DB {
	return s.db.Raw(`
		(SELECT COUNT(*) FROM docker_applications
		    WHERE docker_applications.server_id = servers.id
		      AND docker_applications.deleted_at IS NULL)
		+ (SELECT COUNT(*) FROM docker_composes
		    WHERE docker_composes.server_id = servers.id
		      AND docker_composes.deleted_at IS NULL)
		+ (SELECT COUNT(*) FROM docker_databases
		    WHERE docker_databases.server_id = servers.id
		      AND docker_databases.deleted_at IS NULL)
	`)
}

// dashboardServerRow is a scratch struct used to project the joined
// counts onto a server. servermodels.Server already has SitesCount;
// we add WorkloadsCount as a sibling read-only column for this query.
type dashboardServerRow struct {
	servermodels.Server
	WorkloadsCount int64 `gorm:"column:workloads_count;->"`
}

// getServers returns up to 8 servers for the team
func (s *DashboardService) getServers(ctx context.Context, teamID string) ([]*dto.DashboardServerResponse, error) {
	var rows []dashboardServerRow

	err := s.db.WithContext(ctx).
		Model(&servermodels.Server{}).
		Select(
			"servers.*, (?) as sites_count, (?) as workloads_count",
			s.sitesCountSubquery(),
			s.workloadsCountSubquery(),
		).
		Scopes(repository.WithTeamID(teamID), repository.WithActive()).
		Order("created_at DESC").
		Limit(maxServers).
		Find(&rows).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.DashboardServerResponse, len(rows))
	for i, row := range rows {
		status := "disconnected"
		if row.Connected {
			status = "connected"
		}

		typeStr := ""
		if row.Type != nil {
			typeStr = *row.Type
		}

		result[i] = &dto.DashboardServerResponse{
			ID:             row.ID,
			Name:           row.Name,
			Status:         status,
			Provider:       row.Provider.String(),
			Type:           typeStr,
			SitesCount:     row.SitesCount,
			WorkloadsCount: row.WorkloadsCount,
		}
	}

	return result, nil
}

// recentActivityResult represents a row from the recent activity query
type recentActivityResult struct {
	sitemodels.Deployment
	SiteName   string  `gorm:"column:site_name"`
	ServerID   string  `gorm:"column:server_id"`
	ServerName string  `gorm:"column:server_name"`
	UserName   *string `gorm:"column:user_name"`
}

// getRecentActivity returns up to 6 recent deployments for the team
func (s *DashboardService) getRecentActivity(ctx context.Context, teamID string) ([]*dto.DashboardActivityResponse, error) {
	var results []recentActivityResult

	err := s.db.WithContext(ctx).
		Model(&sitemodels.Deployment{}).
		Select(
			"deployments.*",
			"sites.address as site_name",
			"sites.server_id as server_id",
			"servers.name as server_name",
			"users.name as user_name",
		).
		Joins("JOIN sites ON deployments.site_id = sites.id").
		Joins("JOIN servers ON sites.server_id = servers.id").
		Joins("LEFT JOIN users ON deployments.user_id = users.id").
		Where("sites.team_id = ?", teamID).
		Order("deployments.created_at DESC").
		Limit(maxRecentActivity).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	activity := make([]*dto.DashboardActivityResponse, len(results))
	for i, r := range results {
		var user *dto.DashboardUserResponse
		if r.UserName != nil && *r.UserName != "" {
			user = &dto.DashboardUserResponse{
				Name: *r.UserName,
			}
		}

		status := mapDeploymentStatus(string(r.Status))

		activity[i] = &dto.DashboardActivityResponse{
			ID:            r.ID,
			SiteName:      r.SiteName,
			SiteID:        r.SiteID,
			ServerID:      r.ServerID,
			ServerName:    r.ServerName,
			Status:        status,
			CreatedAt:     r.CreatedAt,
			CommitSha:     r.GetShortGitHash(),
			CommitMessage: r.CommitMessage(),
			User:          user,
		}
	}

	return activity, nil
}

// mapDeploymentStatus maps internal status to API status
func mapDeploymentStatus(status string) string {
	switch status {
	case "pending", "queued":
		return "pending"
	case "installing":
		return "deploying"
	case "finished":
		return "finished"
	case "failed", "timeout":
		return "failed"
	default:
		return status
	}
}

// Ensure User model is imported for the query
var _ = authmodels.User{}

// GetOnboardingStatus returns onboarding status for a user
func (s *DashboardService) GetOnboardingStatus(ctx context.Context, userID string) (*dto.OnboardingStatusResponse, error) {
	user, err := s.repos.User.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, nil
	}

	var (
		hasServerProvider      bool
		hasSourceControl       bool
		hasDomainProvider      bool
		hasStorageProvider     bool
		hasNotificationChannel bool
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		result, err := s.hasServerProvider(ctx, userID)
		if err != nil {
			return err
		}
		hasServerProvider = result
		return nil
	})

	g.Go(func() error {
		result, err := s.hasSourceControl(ctx, userID)
		if err != nil {
			return err
		}
		hasSourceControl = result
		return nil
	})

	g.Go(func() error {
		result, err := s.hasDomainProvider(ctx, userID)
		if err != nil {
			return err
		}
		hasDomainProvider = result
		return nil
	})

	g.Go(func() error {
		result, err := s.hasStorageProvider(ctx, userID)
		if err != nil {
			return err
		}
		hasStorageProvider = result
		return nil
	})

	g.Go(func() error {
		result, err := s.hasNotificationChannel(ctx, userID)
		if err != nil {
			return err
		}
		hasNotificationChannel = result
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &dto.OnboardingStatusResponse{
		Onboarded:              user.Onboarded,
		HasServerProvider:      hasServerProvider,
		HasSourceControl:       hasSourceControl,
		HasDomainProvider:      hasDomainProvider,
		HasStorageProvider:     hasStorageProvider,
		HasNotificationChannel: hasNotificationChannel,
	}, nil
}

// CompleteOnboarding marks the user as onboarded
func (s *DashboardService) CompleteOnboarding(ctx context.Context, userID string) error {
	return s.repos.User.SetOnboarded(ctx, userID, true)
}

// hasServerProvider checks if user has any server provider connected
func (s *DashboardService) hasServerProvider(ctx context.Context, userID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("server_providers").
		Where("user_id = ?", userID).
		Limit(1).
		Count(&count).Error

	return count > 0, err
}

// hasSourceControl checks if user has any source control connected
func (s *DashboardService) hasSourceControl(ctx context.Context, userID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("source_controls").
		Where("user_id = ?", userID).
		Limit(1).
		Count(&count).Error

	return count > 0, err
}

// hasDomainProvider checks if user has any domain provider connected
func (s *DashboardService) hasDomainProvider(ctx context.Context, userID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("domain_providers").
		Where("user_id = ?", userID).
		Limit(1).
		Count(&count).Error

	return count > 0, err
}

// hasStorageProvider checks if user has any storage provider connected
func (s *DashboardService) hasStorageProvider(ctx context.Context, userID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("storage_providers").
		Where("user_id = ?", userID).
		Limit(1).
		Count(&count).Error

	return count > 0, err
}

// hasNotificationChannel checks if user has any notification channel configured
func (s *DashboardService) hasNotificationChannel(ctx context.Context, userID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("notification_channels").
		Where("user_id = ?", userID).
		Limit(1).
		Count(&count).Error

	return count > 0, err
}
