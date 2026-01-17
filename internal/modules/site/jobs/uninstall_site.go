package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

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
	ctx     *JobContext
	Payload UninstallSitePayload
}

// NewUninstallSiteJob creates a new UninstallSiteJob with the given context and payload
func NewUninstallSiteJob(ctx *JobContext, payload UninstallSitePayload) *UninstallSiteJob {
	return &UninstallSiteJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the uninstall site job
func (j *UninstallSiteJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.ctx.LogInfo("Starting site uninstallation",
		"site_id", site.ID,
		"server_id", server.ID,
		"address", site.Address,
	)

	// Create site task factory for convenient task creation
	siteFactory := tasks.NewFactory(site)

	// Step 1: Uninstall all queue workers for this site
	queues, err := j.ctx.QueueRepo.FindBySite(ctx, site.ID)
	if err != nil {
		j.ctx.LogError(err, "Failed to get site queues")
	} else {
		for _, q := range queues {
			// Delete queue config and stop process
			deleteTask := siteFactory.DeleteQueueConfig(q.GetPath(), q.ID)
			if _, err := j.ctx.RunTaskOnServer(server, deleteTask).AsRoot().Dispatch(ctx); err != nil {
				j.ctx.LogError(err, "Failed to uninstall queue", "queue_id", q.ID)
			}
			// Delete queue record
			if err := j.ctx.QueueRepo.Delete(ctx, q.ID); err != nil {
				j.ctx.LogError(err, "Failed to delete queue record", "queue_id", q.ID)
			}
		}
	}

	// Step 2: Update Caddyfile to remove site imports
	allSites, err := j.ctx.SiteRepo.FindByServer(ctx, server.ID)
	if err != nil {
		j.ctx.LogError(err, "Failed to get server sites for Caddyfile update")
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
		if _, err := j.ctx.RunTaskOnServer(server, updateTask).AsRoot().Dispatch(ctx); err != nil {
			j.ctx.LogError(err, "Failed to update Caddyfile site imports")
		}
	}

	// Step 3: Delete site files from server using factory
	deleteFilesTask := siteFactory.DeleteFiles()

	result, err := j.ctx.RunTaskOnServer(server, deleteFilesTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.ctx.LogError(err, "Failed to delete site files")
	} else if result.GetExitCode() != 0 {
		j.ctx.LogError(nil, "Site files deletion failed", "exit_code", result.GetExitCode())
	}

	// Step 4: Delete related database records using repositories
	if err := j.ctx.DeploymentRepo.DeleteBySite(ctx, site.ID); err != nil {
		j.ctx.LogError(err, "Failed to delete deployments")
	}

	if err := j.ctx.CertificateRepo.DeleteBySite(ctx, site.ID); err != nil {
		j.ctx.LogError(err, "Failed to delete certificates")
	}

	if err := j.ctx.CommandRepo.DeleteBySite(ctx, site.ID); err != nil {
		j.ctx.LogError(err, "Failed to delete commands")
	}

	if err := j.ctx.RedirectRepo.DeleteBySite(ctx, site.ID); err != nil {
		j.ctx.LogError(err, "Failed to delete redirects")
	}

	// Step 5: Delete site record
	if err := j.ctx.SiteRepo.Delete(ctx, site.ID); err != nil {
		return fmt.Errorf("failed to delete site: %w", err)
	}

	// Step 6: Broadcast event
	j.ctx.BroadcastServerEvent(server, "site.deleted", map[string]interface{}{
		"team_id":   server.TeamID,
		"site_id":   site.ID,
		"server_id": server.ID,
		"address":   site.Address,
	})

	j.ctx.LogInfo("Site uninstalled successfully",
		"site_id", site.ID,
		"address", site.Address,
	)

	return nil
}

// Failed handles job failure and marks the site uninstallation as failed
func (j *UninstallSiteJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Uninstall site job failed",
		"site_id", j.Payload.SiteID,
		"server_id", j.Payload.ServerID,
	)

	// Mark uninstallation as failed
	site, findErr := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr != nil {
		j.ctx.LogError(findErr, "Failed to find site for marking uninstallation failed")
		return
	}

	site.MarkUninstallationFailed()
	if updateErr := j.ctx.SiteRepo.Update(ctx, site); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to mark site uninstallation as failed")
	}
}

// NewUninstallSiteTask creates an uninstall site job
func NewUninstallSiteTask(siteID, serverID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallSite, UninstallSitePayload{
		SiteID:   siteID,
		ServerID: serverID,
		UserID:   userID,
	})
}
