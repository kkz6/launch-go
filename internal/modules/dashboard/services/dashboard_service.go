package services

import (
	"context"

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

// getServers returns up to 8 servers for the team
func (s *DashboardService) getServers(ctx context.Context, teamID string) ([]*dto.DashboardServerResponse, error) {
	var servers []servermodels.Server

	err := s.db.WithContext(ctx).
		Select("servers.*, (SELECT COUNT(*) FROM sites WHERE sites.server_id = servers.id) as sites_count").
		Scopes(repository.WithTeamID(teamID), repository.WithActive()).
		Order("created_at DESC").
		Limit(maxServers).
		Find(&servers).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.DashboardServerResponse, len(servers))
	for i, server := range servers {
		status := "disconnected"
		if server.Connected {
			status = "connected"
		}

		result[i] = &dto.DashboardServerResponse{
			ID:         server.ID,
			Name:       server.Name,
			Status:     status,
			Provider:   server.Provider.String(),
			SitesCount: server.SitesCount,
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
		Table("deployments").
		Select(`
			deployments.*,
			sites.address as site_name,
			sites.server_id as server_id,
			servers.name as server_name,
			users.name as user_name
		`).
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
