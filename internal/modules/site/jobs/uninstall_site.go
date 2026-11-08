package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

const TypeUninstallSite = "site:uninstall"

// UninstallSitePayload holds data for site uninstallation
type UninstallSitePayload struct {
	SiteID      string     `json:"site_id"`
	ServerID    string     `json:"server_id"`
	UserID      *string    `json:"user_id,omitempty"`
	TeamID      string     `json:"team_id,omitempty"`
	RequestedAt *time.Time `json:"requested_at,omitempty"`
}

// UninstallSiteScope binds new requests to their owning team and pending intent.
// Older queued payloads remain supported through the stored site/server scope.
type UninstallSiteScope struct {
	TeamID      string
	RequestedAt *time.Time
}

// UninstallSiteJob handles complete site uninstallation
type UninstallSiteJob struct {
	Deps    *JobDeps
	Payload UninstallSitePayload

	// Model fields for Failed() callback
	site     *models.Site
	server   *servermodels.Server
	rejected bool
}

// NewUninstallSiteJob creates a new UninstallSiteJob with the given payload
func NewUninstallSiteJob(p UninstallSitePayload) pkgjobs.Handler {
	return &UninstallSiteJob{Deps: deps, Payload: p}
}

// Handle executes the uninstall site job
func (j *UninstallSiteJob) Handle(ctx context.Context) error {
	// Wait for the service's enqueue transaction before trusting pending state.
	var site models.Site
	err := j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&site, "id = ?", j.Payload.SiteID).Error; err != nil {
			return err
		}
		if site.ServerID != j.Payload.ServerID ||
			(j.Payload.TeamID != "" && site.TeamID != j.Payload.TeamID) ||
			site.UninstallationRequestedAt == nil ||
			(j.Payload.RequestedAt != nil && !site.UninstallationRequestedAt.Equal(*j.Payload.RequestedAt)) {
			j.rejected = true
			return errors.New("site deletion request no longer matches the site")
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil // A completed uninstall may be delivered again.
	}
	if err != nil {
		return fmt.Errorf("failed to load site deletion request: %w", err)
	}
	j.site = &site

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server
	if server.TeamID != site.TeamID {
		j.rejected = true
		return errors.New("site deletion server ownership does not match")
	}

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("server_id", server.ID).
		Str("address", site.Address).
		Msg("Starting site uninstallation")

	// Create site task factory for convenient task creation
	siteFactory := tasks.NewFactory(&site)

	// Step 1: Uninstall all queue workers for this site
	queues, err := j.Deps.Repos.Queue().FindBySite(ctx, site.ID)
	if err != nil {
		return fmt.Errorf("failed to get site queues: %w", err)
	}
	for _, q := range queues {
		deleteTask := siteFactory.DeleteQueueConfig(q.GetPath(), q.ID)
		if err := j.runCleanup(ctx, deleteTask); err != nil {
			return fmt.Errorf("failed to uninstall queue: %w", err)
		}
	}

	// Step 2: Update Caddyfile to remove site imports
	allSites, err := j.Deps.Repos.Site().FindByServer(ctx, server.ID)
	if err != nil {
		return fmt.Errorf("failed to get server sites: %w", err)
	}
	imports := make([]tasks.SiteImport, 0, len(allSites))
	for _, s := range allSites {
		if s.ID != site.ID && s.InstalledAt != nil {
			imports = append(imports, tasks.SiteImport{Path: s.Path})
		}
	}
	updateTask := tasks.UpdateCaddySiteImports(tasks.UpdateCaddySiteImportsConfig{Sites: imports})
	if err := j.runCleanup(ctx, updateTask); err != nil {
		return fmt.Errorf("failed to update Caddyfile site imports: %w", err)
	}

	// Step 3: Delete site files from server using factory
	deleteFilesTask := siteFactory.DeleteFiles()

	if err := j.runCleanup(ctx, deleteFilesTask); err != nil {
		return fmt.Errorf("failed to delete site files: %w", err)
	}

	// Preserve every database record until all remote cleanup succeeds, then
	// remove related records and the site together.
	if err := j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repos := repositories.NewRegistry(tx)
		for _, remove := range []func(context.Context, string) error{
			repos.Deployment().DeleteBySite, repos.Certificate().DeleteBySite,
			repos.Command().DeleteBySite, repos.Redirect().DeleteBySite,
		} {
			if err := remove(ctx, site.ID); err != nil {
				return err
			}
		}
		if err := tx.Where("site_id = ?", site.ID).Delete(&models.Queue{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND server_id = ? AND team_id = ? AND uninstallation_requested_at = ?",
			site.ID, site.ServerID, site.TeamID, *site.UninstallationRequestedAt).Delete(&models.Site{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("site deletion request changed during cleanup")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to delete site records: %w", err)
	}
	activity.RecordEventPtr(ctx, "deleted", j.Payload.UserID, &site, "Site deleted")

	// Step 6: Broadcast event
	j.Deps.BroadcastServerEvent(server, "site.deleted", map[string]interface{}{
		"team_id":   server.TeamID,
		"site_id":   site.ID,
		"server_id": server.ID,
		"address":   site.Address,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Str("address", site.Address).Msg("Site uninstalled successfully")

	return nil
}

// Failed handles job failure and marks the site uninstallation as failed
func (j *UninstallSiteJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("server_id", j.Payload.ServerID).
		Msg("Uninstall site job failed")

	if j.rejected {
		return
	}
	requestedAt := j.Payload.RequestedAt
	if requestedAt == nil && j.site != nil {
		requestedAt = j.site.UninstallationRequestedAt
	}
	if requestedAt == nil {
		return
	}
	// A canceled worker must still release its pending request so an operator
	// can retry. Bound this finalization independently of the failed job.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	query := j.Deps.DB.WithContext(ctx).Model(&models.Site{}).
		Where("id = ? AND server_id = ? AND uninstallation_requested_at = ?", j.Payload.SiteID, j.Payload.ServerID, *requestedAt)
	if j.Payload.TeamID != "" {
		query = query.Where("team_id = ?", j.Payload.TeamID)
	}
	result := query.Updates(map[string]any{
		"uninstallation_requested_at": nil,
		"uninstallation_failed_at":    time.Now().UTC(),
	})
	if result.Error != nil {
		j.Deps.Logger.Error().Err(result.Error).Msg("Failed to mark site uninstallation as failed")
		return
	}
	if result.RowsAffected == 0 {
		return
	}
	site, findErr := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if findErr == nil {
		j.Deps.BroadcastToTeam(site.TeamID, "site.deletion_failed", map[string]any{
			"site_id": site.ID, "server_id": site.ServerID,
		})
		j.Deps.BroadcastToTeam(site.TeamID, "site.updated", map[string]any{
			"team_id": site.TeamID, "server_id": site.ServerID,
			"site_id": site.ID, "site": dto.ToSiteResponse(site),
		})
	}
}

func (j *UninstallSiteJob) runCleanup(ctx context.Context, task taskrunner.Task) error {
	result, err := j.Deps.RunTask(j.server, task).ForSite(j.site.ID).AsRoot().Dispatch(ctx)
	if err != nil {
		return err
	}
	if result == nil || !result.IsSuccessful() {
		return errors.New("remote cleanup did not complete successfully")
	}
	return nil
}

// NewUninstallSiteTask uses a fresh ID for each accepted deletion request so a
// previously failed archived task cannot silently suppress an explicit retry.
func NewUninstallSiteTask(siteID, serverID string, userID *string, scope ...UninstallSiteScope) (*asynq.Task, error) {
	payload := UninstallSitePayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	}
	if len(scope) > 0 {
		payload.TeamID = scope[0].TeamID
		payload.RequestedAt = scope[0].RequestedAt
	}
	return pkgjobs.Task(TypeUninstallSite, payload, asynq.TaskID(pkgjobs.Dedup("uninstall_site", siteID, util.NewULID())))
}
