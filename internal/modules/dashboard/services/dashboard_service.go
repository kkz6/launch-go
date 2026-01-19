package services

import (
	"context"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/dashboard/dto"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
)

const (
	maxServers        = 8
	maxRecentActivity = 6
)

// DashboardService handles dashboard data retrieval
type DashboardService struct {
	db     *gorm.DB
	logger *zerolog.Logger
}

// NewDashboardService creates a new dashboard service
func NewDashboardService(db *gorm.DB, logger *zerolog.Logger) *DashboardService {
	return &DashboardService{
		db:     db,
		logger: logger,
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
		Where("team_id = ? AND archived_at IS NULL", teamID).
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
		Where("deployments.team_id = ?", teamID).
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

		// Map deployment status to API status
		status := mapDeploymentStatus(string(r.Status))

		activity[i] = &dto.DashboardActivityResponse{
			ID:         r.ID,
			SiteName:   r.SiteName,
			SiteID:     r.SiteID,
			ServerID:   r.ServerID,
			ServerName: r.ServerName,
			Status:     status,
			CreatedAt:  r.CreatedAt,
			CommitSha:  r.GetShortGitHash(),
			User:       user,
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
