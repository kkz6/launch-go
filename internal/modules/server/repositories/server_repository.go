package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ServerRepository handles server database operations
type ServerRepository struct {
	repository.Base[models.Server]
}

// NewServerRepository creates a new ServerRepository instance
func NewServerRepository(db *gorm.DB) *ServerRepository {
	return &ServerRepository{
		Base: repository.NewBase[models.Server](db, "Services"),
	}
}

// servicesByInstallOrder forces preloaded services to come back in the
// order they were installed. Without an explicit ORDER BY, Postgres is
// free to return rows in any order — usually heap order, but it shifts
// after VACUUM or row updates. That manifested in the UI as the
// installed-services list (Supervisor, Caddy, PHP, …) reshuffling
// between page loads of the provision-status sheet.
func servicesByInstallOrder(db *gorm.DB) *gorm.DB {
	return db.Order("services.created_at ASC")
}

// FindWithRelations finds a server with all relations
func (r *ServerRepository) FindWithRelations(ctx context.Context, id, teamID string) (*models.Server, error) {
	var server models.Server
	err := r.DB.WithContext(ctx).
		Preload("Services", servicesByInstallOrder).
		Preload("FirewallRules").
		Preload("Crons").
		Preload("Daemons").
		Preload("SSHKeys").
		First(&server, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &server, nil
}

// sitesCountSubquery returns a GORM subquery for counting sites per server (database-agnostic)
func (r *ServerRepository) sitesCountSubquery() *gorm.DB {
	return r.DB.Model(&sitemodels.Site{}).
		Select("COUNT(*)").
		Where("sites.server_id = servers.id")
}

// projectsCountSubquery returns a GORM subquery for counting live docker
// projects per server. Used to populate models.Server.ProjectsCount so the
// frontend can disable the Delete button on docker servers that still have
// projects (the DeleteServer service hard-blocks the same condition; this
// is just for proactive UX).
//
// The docker module's models package is intentionally NOT imported here —
// the dependency direction is docker -> server, never the reverse. We
// reference the table by name and filter on `deleted_at IS NULL` to match
// the soft-delete semantics docker_projects uses elsewhere.
func (r *ServerRepository) projectsCountSubquery() *gorm.DB {
	return r.DB.Table("docker_projects").
		Select("COUNT(*)").
		Where("docker_projects.server_id = servers.id AND docker_projects.deleted_at IS NULL")
}

// workloadsCountSubquery sums live docker workloads — applications +
// composes + databases — for the server. Same cross-module reference
// pattern as projectsCountSubquery: tables-by-name, no import of the
// docker package. Populates models.Server.WorkloadsCount so the
// Servers list card can render "X workloads" on docker servers
// (where SitesCount is always 0 since `sites` is Laravel-only).
func (r *ServerRepository) workloadsCountSubquery() *gorm.DB {
	return r.DB.Raw(`
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

// listSelect is the shared SELECT projection for the three list-
// style queries below. Keeps the four columns (sites_count,
// projects_count, workloads_count) in lockstep so adding another
// computed count later is a single-line change.
func (r *ServerRepository) listSelect() string {
	return "servers.*, (?) as sites_count, (?) as projects_count, (?) as workloads_count"
}

// FindAllByTeam finds all active (non-archived) servers for a team
func (r *ServerRepository) FindAllByTeam(ctx context.Context, teamID string) ([]models.Server, error) {
	var servers []models.Server
	err := r.DB.WithContext(ctx).
		Select(r.listSelect(), r.sitesCountSubquery(), r.projectsCountSubquery(), r.workloadsCountSubquery()).
		Preload("Services", servicesByInstallOrder).
		Scopes(repository.WithTeamID(teamID), repository.WithActive()).
		Order("created_at DESC").
		Find(&servers).Error
	return servers, err
}

// FindAllByTeamPaginated finds all servers for a team with pagination
func (r *ServerRepository) FindAllByTeamPaginated(ctx context.Context, teamID string, page, perPage int) (*repository.PaginatedResult[models.Server], error) {
	countQuery := r.DB.WithContext(ctx).
		Model(&models.Server{}).
		Scopes(repository.WithTeamID(teamID), repository.WithActive())

	dataQuery := r.DB.WithContext(ctx).
		Select(r.listSelect(), r.sitesCountSubquery(), r.projectsCountSubquery(), r.workloadsCountSubquery()).
		Preload("Services", servicesByInstallOrder).
		Scopes(repository.WithTeamID(teamID), repository.WithActive()).
		Order("created_at DESC")

	return repository.PaginateWithCount[models.Server](countQuery, dataQuery, page, perPage)
}

// FindArchivedByTeam finds all archived servers for a team
func (r *ServerRepository) FindArchivedByTeam(ctx context.Context, teamID string) ([]models.Server, error) {
	var servers []models.Server
	err := r.DB.WithContext(ctx).
		Select(r.listSelect(), r.sitesCountSubquery(), r.projectsCountSubquery(), r.workloadsCountSubquery()).
		Unscoped().
		Where("team_id = ? AND archived_at IS NOT NULL", teamID).
		Order("archived_at DESC").
		Find(&servers).Error
	return servers, err
}

// UpdateStatus updates only the server status
func (r *ServerRepository) UpdateStatus(ctx context.Context, id string, status types.ServerStatus) error {
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// UpdateProgress updates server provisioning progress
func (r *ServerRepository) UpdateProgress(ctx context.Context, id string, progress int, step string) error {
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"progress":      progress,
		"progress_step": step,
	})
}

// Archive archives a server
func (r *ServerRepository) Archive(ctx context.Context, id string) error {
	now := time.Now()
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"archived_at": now,
		"status":      types.ServerStatusArchived,
	})
}

// Unarchive unarchives a server
func (r *ServerRepository) Unarchive(ctx context.Context, id string) error {
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"archived_at": nil,
		"status":      types.ServerStatusStopped,
	})
}

// CountByTeam counts the total number of servers for a team
func (r *ServerRepository) CountByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Server{}).
		Where("team_id = ?", teamID).
		Count(&count).Error
	return count, err
}

// HasLaunchAgent checks if a server has the Launch Agent installed
func (r *ServerRepository) HasLaunchAgent(ctx context.Context, serverID string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.InstalledService{}).
		Where("server_id = ? AND type = ?", serverID, types.ServiceTypeLaunchAgent).
		Count(&count).Error
	return count > 0, err
}
