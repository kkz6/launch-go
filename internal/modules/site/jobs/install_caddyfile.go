package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const (
	TypeInstallCaddyfile   = "site:install_caddyfile"
	TypeUpdateCaddyfile    = "site:update_caddyfile"
	TypeUninstallCaddyfile = "site:uninstall_caddyfile"
)

// CaddyfilePayload holds data for Caddyfile operations
type CaddyfilePayload struct {
	SiteID string  `json:"site_id"`
	UserID *string `json:"user_id,omitempty"`
}

// InstallCaddyfileJob handles site Caddyfile installation
type InstallCaddyfileJob struct {
	ctx     *JobContext
	Payload CaddyfilePayload
}

// NewInstallCaddyfileJob creates a new InstallCaddyfileJob
func NewInstallCaddyfileJob(ctx *JobContext, payload CaddyfilePayload) *InstallCaddyfileJob {
	return &InstallCaddyfileJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the install Caddyfile job
func (j *InstallCaddyfileJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Generate Caddyfile content
	caddyfileContent := j.generateCaddyfileContent(site)
	caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)

	// Create update Caddyfile task
	task := tasks.UpdateCaddyfile(tasks.UpdateCaddyfileConfig{
		CaddyfilePath:    caddyfilePath,
		CaddyfileContent: caddyfileContent,
	})

	// Execute the task on the server
	result, err := j.ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.ctx.LogError(err, "Failed to install Caddyfile", "site_id", site.ID)
		return err
	}

	exitCode := result.GetExitCode()
	if exitCode != 0 {
		j.ctx.LogError(nil, "Caddyfile installation failed", "site_id", site.ID, "exit_code", exitCode)
		return fmt.Errorf("caddyfile installation failed with exit code %d", exitCode)
	}

	// Update site imports
	if err := j.updateSiteImports(ctx, server, site); err != nil {
		j.ctx.LogError(err, "Failed to update site imports", "site_id", site.ID)
	}

	// Mark site as installed and clear pending flags
	now := time.Now()
	site.InstalledAt = &now
	site.InstallationFailedAt = nil
	site.PendingCaddyfileUpdateSince = nil
	if err := j.ctx.SiteRepo.Update(ctx, site); err != nil {
		j.ctx.LogError(err, "Failed to update site installed status")
	}

	j.ctx.LogInfo("Caddyfile installed successfully", "site_id", site.ID)

	return nil
}

// Failed handles job failure
func (j *InstallCaddyfileJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Install Caddyfile job failed", "site_id", j.Payload.SiteID)

	// Mark site installation as failed
	site, findErr := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr != nil {
		return
	}

	now := time.Now()
	site.InstalledAt = nil
	site.InstallationFailedAt = &now
	_ = j.ctx.SiteRepo.Update(ctx, site)
}

// generateCaddyfileContent generates the Caddyfile content for a site
func (j *InstallCaddyfileJob) generateCaddyfileContent(site *models.Site) string {
	var builder strings.Builder

	// Add site address and aliases
	addresses := []string{site.Address}
	if len(site.Aliases) > 0 {
		addresses = append(addresses, site.Aliases...)
	}

	builder.WriteString(strings.Join(addresses, ", "))
	builder.WriteString(" {\n")

	// Root directive
	builder.WriteString(fmt.Sprintf("\troot * %s\n", site.GetWebDirectory()))

	// Encode directive
	builder.WriteString("\tencode gzip\n")

	// PHP handling if PHP version is set
	if site.PhpVersion != nil && *site.PhpVersion != "" {
		phpFpmSocket := fmt.Sprintf("unix//run/php/php%s-fpm.sock", *site.PhpVersion)
		builder.WriteString(fmt.Sprintf("\tphp_fastcgi %s\n", phpFpmSocket))
	}

	// File server
	builder.WriteString("\tfile_server\n")

	// Logs
	builder.WriteString(fmt.Sprintf("\tlog {\n\t\toutput file %s/access.log\n\t}\n", site.GetLogsDirectory()))

	// TLS settings
	switch site.TlsSetting {
	case "off":
		// No TLS block needed
	case "internal":
		builder.WriteString("\ttls internal\n")
	case "custom":
		// Custom TLS would be configured via certificate
	default:
		// Auto TLS is the default (no config needed)
	}

	builder.WriteString("}\n")

	return builder.String()
}

// updateSiteImports updates the global Caddy site imports file
func (j *InstallCaddyfileJob) updateSiteImports(ctx context.Context, server any, site *models.Site) error {
	// Get all sites for this server
	sites, err := j.ctx.SiteRepo.FindByServer(ctx, site.ServerID)
	if err != nil {
		return err
	}

	// Build site imports list
	imports := make([]tasks.SiteImport, 0, len(sites))
	for _, s := range sites {
		if s.InstalledAt != nil {
			imports = append(imports, tasks.SiteImport{
				Path: s.Path,
			})
		}
	}

	// Add current site if not yet installed
	siteIncluded := false
	for _, imp := range imports {
		if imp.Path == site.Path {
			siteIncluded = true
			break
		}
	}
	if !siteIncluded {
		imports = append(imports, tasks.SiteImport{
			Path: site.Path,
		})
	}

	// Create update imports task
	task := tasks.UpdateCaddySiteImports(tasks.UpdateCaddySiteImportsConfig{
		Sites: imports,
	})

	// Execute on server - need to get server model
	serverModel, ok := server.(*models.Site)
	if !ok {
		// Fetch server again
		serverModel, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
		if err != nil {
			return err
		}
		_, err = j.ctx.RunTaskOnServer(serverModel, task).AsRoot().Dispatch(ctx)
		return err
	}

	_, _ = serverModel, task
	return nil
}

// UpdateCaddyfileJob handles site Caddyfile updates
type UpdateCaddyfileJob struct {
	ctx     *JobContext
	Payload CaddyfilePayload
}

// NewUpdateCaddyfileJob creates a new UpdateCaddyfileJob
func NewUpdateCaddyfileJob(ctx *JobContext, payload CaddyfilePayload) *UpdateCaddyfileJob {
	return &UpdateCaddyfileJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the update Caddyfile job
func (j *UpdateCaddyfileJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Generate Caddyfile content
	caddyfileContent := j.generateCaddyfileContent(site)
	caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)

	// Create update Caddyfile task
	task := tasks.UpdateCaddyfile(tasks.UpdateCaddyfileConfig{
		CaddyfilePath:    caddyfilePath,
		CaddyfileContent: caddyfileContent,
	})

	// Execute the task on the server
	result, err := j.ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.ctx.LogError(err, "Failed to update Caddyfile", "site_id", site.ID)
		return err
	}

	exitCode := result.GetExitCode()
	if exitCode != 0 {
		j.ctx.LogError(nil, "Caddyfile update failed", "site_id", site.ID, "exit_code", exitCode)
		return fmt.Errorf("caddyfile update failed with exit code %d", exitCode)
	}

	// Clear pending Caddyfile update flag
	site.PendingCaddyfileUpdateSince = nil
	if err := j.ctx.SiteRepo.Update(ctx, site); err != nil {
		j.ctx.LogError(err, "Failed to clear pending Caddyfile update flag")
	}

	j.ctx.LogInfo("Caddyfile updated successfully", "site_id", site.ID)

	return nil
}

// Failed handles job failure
func (j *UpdateCaddyfileJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Update Caddyfile job failed", "site_id", j.Payload.SiteID)
}

// generateCaddyfileContent generates the Caddyfile content for a site
func (j *UpdateCaddyfileJob) generateCaddyfileContent(site *models.Site) string {
	var builder strings.Builder

	// Add site address and aliases
	addresses := []string{site.Address}
	if len(site.Aliases) > 0 {
		addresses = append(addresses, site.Aliases...)
	}

	builder.WriteString(strings.Join(addresses, ", "))
	builder.WriteString(" {\n")

	// Root directive
	builder.WriteString(fmt.Sprintf("\troot * %s\n", site.GetWebDirectory()))

	// Encode directive
	builder.WriteString("\tencode gzip\n")

	// PHP handling if PHP version is set
	if site.PhpVersion != nil && *site.PhpVersion != "" {
		phpFpmSocket := fmt.Sprintf("unix//run/php/php%s-fpm.sock", *site.PhpVersion)
		builder.WriteString(fmt.Sprintf("\tphp_fastcgi %s\n", phpFpmSocket))
	}

	// File server
	builder.WriteString("\tfile_server\n")

	// Logs
	builder.WriteString(fmt.Sprintf("\tlog {\n\t\toutput file %s/access.log\n\t}\n", site.GetLogsDirectory()))

	// TLS settings
	switch site.TlsSetting {
	case "off":
		// No TLS block needed
	case "internal":
		builder.WriteString("\ttls internal\n")
	case "custom":
		// Custom TLS would be configured via certificate
	default:
		// Auto TLS is the default (no config needed)
	}

	builder.WriteString("}\n")

	return builder.String()
}

// UninstallCaddyfileJob handles site Caddyfile uninstallation
type UninstallCaddyfileJob struct {
	ctx     *JobContext
	Payload CaddyfilePayload
}

// NewUninstallCaddyfileJob creates a new UninstallCaddyfileJob
func NewUninstallCaddyfileJob(ctx *JobContext, payload CaddyfilePayload) *UninstallCaddyfileJob {
	return &UninstallCaddyfileJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the uninstall Caddyfile job
func (j *UninstallCaddyfileJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)

	// Create task to remove Caddyfile
	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

# Remove the Caddyfile
rm -f %s

echo "Caddyfile removed"
`, caddyfilePath)

	task := taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Caddyfile"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)

	// Execute the task on the server
	result, err := j.ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.ctx.LogError(err, "Failed to uninstall Caddyfile", "site_id", site.ID)
		return err
	}

	exitCode := result.GetExitCode()
	if exitCode != 0 {
		j.ctx.LogError(nil, "Caddyfile uninstallation failed", "site_id", site.ID, "exit_code", exitCode)
		return fmt.Errorf("caddyfile uninstallation failed with exit code %d", exitCode)
	}

	// Update site imports to remove this site
	if err := j.updateSiteImportsAfterRemoval(ctx, site); err != nil {
		j.ctx.LogError(err, "Failed to update site imports after removal", "site_id", site.ID)
	}

	j.ctx.LogInfo("Caddyfile uninstalled successfully", "site_id", site.ID)

	return nil
}

// Failed handles job failure
func (j *UninstallCaddyfileJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Uninstall Caddyfile job failed", "site_id", j.Payload.SiteID)
}

// updateSiteImportsAfterRemoval updates site imports after a site is removed
func (j *UninstallCaddyfileJob) updateSiteImportsAfterRemoval(ctx context.Context, removedSite *models.Site) error {
	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, removedSite.ServerID)
	if err != nil {
		return err
	}

	// Get all sites for this server (excluding the removed one)
	sites, err := j.ctx.SiteRepo.FindByServer(ctx, removedSite.ServerID)
	if err != nil {
		return err
	}

	// Build site imports list (excluding removed site)
	imports := make([]tasks.SiteImport, 0, len(sites))
	for _, s := range sites {
		if s.ID != removedSite.ID && s.InstalledAt != nil {
			imports = append(imports, tasks.SiteImport{
				Path: s.Path,
			})
		}
	}

	// Create update imports task
	task := tasks.UpdateCaddySiteImports(tasks.UpdateCaddySiteImportsConfig{
		Sites: imports,
	})

	// Execute on server
	_, err = j.ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	return err
}

// NewInstallCaddyfileTask creates an install Caddyfile job
func NewInstallCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	})
}

// NewUpdateCaddyfileTask creates an update Caddyfile job
func NewUpdateCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUpdateCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	})
}

// NewUninstallCaddyfileTask creates an uninstall Caddyfile job
func NewUninstallCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	})
}
