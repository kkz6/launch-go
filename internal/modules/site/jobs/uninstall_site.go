package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UninstallSiteJob handles complete site uninstallation
type UninstallSiteJob struct {
	SiteJobBase
	Payload UninstallSitePayload
}

// Type returns the job type
func (j *UninstallSiteJob) Type() string {
	return TypeUninstallSite
}

// Handle executes the uninstall site job
func (j *UninstallSiteJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.Ctx.ServerRepo.FindServerByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.LogInfo("Starting site uninstallation",
		"site_id", site.ID,
		"server_id", server.ID,
		"address", site.Address,
	)

	// Step 1: Uninstall all queue workers for this site
	queues, err := j.Ctx.QueueRepo.FindBySite(ctx, site.ID)
	if err != nil {
		j.LogError(err, "Failed to get site queues")
	} else {
		for _, q := range queues {
			// Delete queue config and stop process
			deleteTask := tasks.DeleteQueueConfig(q.GetPath(), q.ID)
			if _, err := j.RunTaskOnServer(server, deleteTask).AsRoot().Dispatch(ctx); err != nil {
				j.LogError(err, "Failed to uninstall queue", "queue_id", q.ID)
			}
			// Delete queue record
			if err := j.Ctx.QueueRepo.Delete(ctx, q.ID); err != nil {
				j.LogError(err, "Failed to delete queue record", "queue_id", q.ID)
			}
		}
	}

	// Step 2: Update Caddyfile to remove site imports
	allSites, err := j.Ctx.SiteRepo.FindByServer(ctx, server.ID)
	if err != nil {
		j.LogError(err, "Failed to get server sites for Caddyfile update")
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
		if _, err := j.RunTaskOnServer(server, updateTask).AsRoot().Dispatch(ctx); err != nil {
			j.LogError(err, "Failed to update Caddyfile site imports")
		}
	}

	// Step 3: Delete site files from server
	deleteFilesScript := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

# Remove the site directory
if [ -d "%s" ]; then
    rm -rf "%s"
    echo "Site directory deleted"
else
    echo "Site directory not found, skipping"
fi
`, site.Path, site.Path)

	deleteFilesTask := taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Site Files"),
		taskrunner.WithScript(deleteFilesScript),
		taskrunner.WithTimeoutSeconds(60),
	)

	result, err := j.RunTaskOnServer(server, deleteFilesTask).AsRoot().Dispatch(ctx)
	if err != nil {
		j.LogError(err, "Failed to delete site files")
	} else if result.GetExitCode() != 0 {
		j.LogError(nil, "Site files deletion failed", "exit_code", result.GetExitCode())
	}

	// Step 4: Delete related database records
	// Delete deployments
	if err := j.deleteDeployments(ctx, site.ID); err != nil {
		j.LogError(err, "Failed to delete deployments")
	}

	// Delete certificates
	if err := j.deleteCertificates(ctx, site.ID); err != nil {
		j.LogError(err, "Failed to delete certificates")
	}

	// Delete commands
	if err := j.deleteCommands(ctx, site.ID); err != nil {
		j.LogError(err, "Failed to delete commands")
	}

	// Delete redirects
	if err := j.deleteRedirects(ctx, site.ID); err != nil {
		j.LogError(err, "Failed to delete redirects")
	}

	// Delete releases
	if err := j.deleteReleases(ctx, site.ID); err != nil {
		j.LogError(err, "Failed to delete releases")
	}

	// Step 5: Delete site record
	if err := j.Ctx.SiteRepo.Delete(ctx, site.ID); err != nil {
		return fmt.Errorf("failed to delete site: %w", err)
	}

	// Step 6: Broadcast event
	j.BroadcastToServer(server.ID, "site.deleted", map[string]interface{}{
		"site_id":   site.ID,
		"server_id": server.ID,
		"address":   site.Address,
	})

	j.LogInfo("Site uninstalled successfully",
		"site_id", site.ID,
		"address", site.Address,
	)

	return nil
}

// Failed handles job failure
func (j *UninstallSiteJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Uninstall site job failed",
		"site_id", j.Payload.SiteID,
		"server_id", j.Payload.ServerID,
	)
}

func (j *UninstallSiteJob) deleteDeployments(ctx context.Context, siteID string) error {
	return j.DB.Where("site_id = ?", siteID).Delete(&struct {
		ID string `gorm:"primaryKey"`
	}{}).Error
}

func (j *UninstallSiteJob) deleteCertificates(ctx context.Context, siteID string) error {
	return j.DB.Table("certificates").Where("site_id = ?", siteID).Delete(&struct {
		ID string `gorm:"primaryKey"`
	}{}).Error
}

func (j *UninstallSiteJob) deleteCommands(ctx context.Context, siteID string) error {
	return j.DB.Table("commands").Where("site_id = ?", siteID).Delete(&struct {
		ID string `gorm:"primaryKey"`
	}{}).Error
}

func (j *UninstallSiteJob) deleteRedirects(ctx context.Context, siteID string) error {
	return j.DB.Table("redirects").Where("site_id = ?", siteID).Delete(&struct {
		ID string `gorm:"primaryKey"`
	}{}).Error
}

func (j *UninstallSiteJob) deleteReleases(ctx context.Context, siteID string) error {
	return j.DB.Table("releases").Where("site_id = ?", siteID).Delete(&struct {
		ID string `gorm:"primaryKey"`
	}{}).Error
}
