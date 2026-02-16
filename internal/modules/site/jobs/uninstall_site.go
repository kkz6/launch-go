package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUninstallSite = "site:uninstall"

// UninstallSitePayload holds data for site uninstallation
type UninstallSitePayload struct {
	SiteID   string  `json:"site_id"`
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallSiteJob handles complete site uninstallation
type UninstallSiteJob struct {
	Deps    *JobDeps
	Payload UninstallSitePayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewUninstallSiteJob creates a new UninstallSiteJob with the given payload
func NewUninstallSiteJob(p UninstallSitePayload) pkgjobs.Handler {
	return &UninstallSiteJob{Deps: deps, Payload: p}
}

// Handle executes the uninstall site job
func (j *UninstallSiteJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	j.Deps.Logger.Info().
		Str("site_id", site.ID).
		Str("server_id", server.ID).
		Str("address", site.Address).
		Msg("Starting site uninstallation")

	// Create site task factory for convenient task creation
	siteFactory := tasks.NewFactory(site)

	// Step 1: Uninstall all queue workers for this site
	queues, err := j.Deps.Repos.Queue().FindBySite(ctx, site.ID)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to get site queues")
	} else {
		for _, q := range queues {
			// Delete queue config and stop process
			deleteTask := siteFactory.DeleteQueueConfig(q.GetPath(), q.ID)
			if _, err := j.Deps.RunTask(server, deleteTask).AsUser().Dispatch(ctx); err != nil {
				j.Deps.Logger.Error().Err(err).Str("queue_id", q.ID).Msg("Failed to uninstall queue")
			}
			// Delete queue record
			if err := j.Deps.Repos.Queue().Delete(ctx, q.ID); err != nil {
				j.Deps.Logger.Error().Err(err).Str("queue_id", q.ID).Msg("Failed to delete queue record")
			}
		}
	}

	// Step 2: Update Caddyfile to remove site imports
	allSites, err := j.Deps.Repos.Site().FindByServer(ctx, server.ID)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to get server sites for Caddyfile update")
	} else {
		// Build imports list excluding the site being deleted
		imports := make([]tasks.SiteImport, 0, len(allSites))
		for _, s := range allSites {
			if s.ID != site.ID && s.InstalledAt != nil {
				imports = append(imports, tasks.SiteImport{
					Path: s.Path,
				})
			}
		}

		// Update site imports
		updateTask := tasks.UpdateCaddySiteImports(tasks.UpdateCaddySiteImportsConfig{
			Sites: imports,
		})
		if _, err := j.Deps.RunTask(server, updateTask).AsUser().Dispatch(ctx); err != nil {
			j.Deps.Logger.Error().Err(err).Msg("Failed to update Caddyfile site imports")
		}
	}

	// Step 3: Delete site files from server using factory
	deleteFilesTask := siteFactory.DeleteFiles()

	result, err := j.Deps.RunTask(server, deleteFilesTask).AsUser().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to delete site files")
	} else if result.GetExitCode() != 0 {
		j.Deps.Logger.Error().Int("exit_code", result.GetExitCode()).Msg("Site files deletion failed")
	}

	// Step 4: Delete related database records using repositories
	if err := j.Deps.Repos.Deployment().DeleteBySite(ctx, site.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to delete deployments")
	}

	if err := j.Deps.Repos.Certificate().DeleteBySite(ctx, site.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to delete certificates")
	}

	if err := j.Deps.Repos.Command().DeleteBySite(ctx, site.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to delete commands")
	}

	if err := j.Deps.Repos.Redirect().DeleteBySite(ctx, site.ID); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to delete redirects")
	}

	// Step 5: Delete site record
	if err := j.Deps.Repos.Site().Delete(ctx, site.ID); err != nil {
		return fmt.Errorf("failed to delete site: %w", err)
	}

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

	// Mark uninstallation as failed
	site, findErr := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if findErr != nil {
		j.Deps.Logger.Error().Err(findErr).Msg("Failed to find site for marking uninstallation failed")
		return
	}

	if updateErr := j.Deps.Repos.Site().MarkUninstallationFailed(ctx, site.ID); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Msg("Failed to mark site uninstallation as failed")
	}
}

// NewUninstallSiteTask creates an uninstall site job
// Uses TaskID for deduplication to prevent duplicate uninstalls
func NewUninstallSiteTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUninstallSite, UninstallSitePayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	}, asynq.TaskID(pkgjobs.Dedup("uninstall_site", siteID)))
}
