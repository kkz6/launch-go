package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	pkgjobs.BaseJob[*JobContext, CaddyfilePayload]
}

// NewInstallCaddyfileJob creates a new InstallCaddyfileJob
func NewInstallCaddyfileJob(ctx *JobContext, payload CaddyfilePayload) *InstallCaddyfileJob {
	return &InstallCaddyfileJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the install Caddyfile job
func (j *InstallCaddyfileJob) Handle(ctx context.Context) error {
	j.Ctx.LogInfo("InstallCaddyfile job started", "site_id", j.Payload.SiteID)

	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Get redirects for Caddyfile generation (installed + pending)
	installedRedirects, _ := j.Ctx.RedirectRepo.FindBySiteForCaddy(ctx, site.ID)
	pendingRedirects, _ := j.Ctx.RedirectRepo.FindPendingBySite(ctx, site.ID)
	allRedirects := append(installedRedirects, pendingRedirects...)

	// Generate Caddyfile content
	caddyfileContent := j.generateCaddyfileContent(site, allRedirects)
	caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)

	// Create update Caddyfile task
	task := tasks.UpdateCaddyfile(tasks.UpdateCaddyfileConfig{
		CaddyfilePath:    caddyfilePath,
		CaddyfileContent: caddyfileContent,
	})

	// Execute the task on the server
	result, err := j.Ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to install Caddyfile", "site_id", site.ID)
		return err
	}

	exitCode := result.GetExitCode()
	if exitCode != 0 {
		j.Ctx.LogError(nil, "Caddyfile installation failed", "site_id", site.ID, "exit_code", exitCode)
		return fmt.Errorf("caddyfile installation failed with exit code %d", exitCode)
	}

	// Update site imports
	if err := j.updateSiteImports(ctx, server, site); err != nil {
		j.Ctx.LogError(err, "Failed to update site imports", "site_id", site.ID)
	}

	// Mark site as installed and clear pending flags
	now := time.Now()
	site.InstalledAt = &now
	site.InstallationFailedAt = nil
	site.PendingCaddyfileUpdateSince = nil
	if err := j.Ctx.SiteRepo.Update(ctx, site); err != nil {
		j.Ctx.LogError(err, "Failed to update site installed status")
	}

	// Mark pending redirects as installed
	if err := j.Ctx.RedirectRepo.UpdateStatusBySite(ctx, site.ID, "pending", "installed"); err != nil {
		j.Ctx.LogError(err, "Failed to update redirect statuses")
	}

	// Broadcast site.installed event
	j.Ctx.BroadcastServerEvent(server, "site.installed", map[string]interface{}{
		"team_id":   server.TeamID,
		"site_id":   site.ID,
		"server_id": server.ID,
		"address":   site.Address,
	})

	j.Ctx.LogInfo("Caddyfile installed successfully", "site_id", site.ID)

	return nil
}

// Failed handles job failure
func (j *InstallCaddyfileJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Install Caddyfile job failed", "site_id", j.Payload.SiteID)

	// Mark site installation as failed
	site, findErr := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if findErr != nil {
		return
	}

	now := time.Now()
	site.InstalledAt = nil
	site.InstallationFailedAt = &now
	_ = j.Ctx.SiteRepo.Update(ctx, site)

	// Broadcast site.installation_failed event
	server, serverErr := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if serverErr == nil {
		j.Ctx.BroadcastServerEvent(server, "site.installation_failed", map[string]interface{}{
			"team_id":   server.TeamID,
			"site_id":   site.ID,
			"server_id": server.ID,
			"address":   site.Address,
			"error":     err.Error(),
		})
	}
}

// generateCaddyfileContent generates the Caddyfile content for a site
func (j *InstallCaddyfileJob) generateCaddyfileContent(site *models.Site, redirects []models.Redirect) string {
	return generateCaddyfile(site, redirects)
}

// generateCaddyfile generates the Caddyfile content for a site (shared function)
func generateCaddyfile(site *models.Site, redirects []models.Redirect) string {
	var builder strings.Builder
	port := site.GetPort()

	// WWW redirect handling
	if site.StartsWithWww() {
		// Site starts with www, redirect non-www to www
		nonWwwAddress := strings.TrimPrefix(site.Address, "www.")
		builder.WriteString(fmt.Sprintf("%s:%d {\n", nonWwwAddress, port))
		builder.WriteString("\tredir {scheme}://www.{host}{uri}\n")
		builder.WriteString("}\n\n")
	} else {
		// Site doesn't start with www, redirect www to non-www
		builder.WriteString(fmt.Sprintf("www.%s:%d {\n", site.Address, port))
		builder.WriteString(fmt.Sprintf("\tredir {scheme}://%s{uri}\n", site.Address))
		builder.WriteString("}\n\n")
	}

	// TLS snippet
	builder.WriteString("# Do not remove this tls-* snippet\n")
	builder.WriteString(generateTLSSnippet(site))
	builder.WriteString("\n")

	// Main server block
	builder.WriteString(fmt.Sprintf("%s:%d {\n", site.Address, port))
	builder.WriteString(fmt.Sprintf("root * %s\n", site.GetWebDirectory()))
	builder.WriteString("encode zstd gzip\n\n")

	// Import TLS snippet
	builder.WriteString(fmt.Sprintf("import tls-%s\n\n", site.ID))

	// Security headers
	builder.WriteString("header {\n")
	builder.WriteString("\t-Server\n")
	builder.WriteString("\tX-Content-Type-Options nosniff\n")
	builder.WriteString("\tX-Frame-Options SAMEORIGIN\n")
	builder.WriteString("\tX-Powered-By \"Launch\"\n")
	builder.WriteString("\tX-XSS-Protection \"1; mode=block\"\n")
	builder.WriteString("}\n\n")

	// PHP FastCGI for non-static sites
	if site.Type != sitetypes.SiteTypeStatic && site.PhpVersion != nil && site.PhpVersion.IsValid() {
		phpSocket := site.PhpVersion.SocketPath()
		builder.WriteString(fmt.Sprintf("php_fastcgi unix/%s {\n", phpSocket))
		builder.WriteString("\tresolve_root_symlink\n")
		builder.WriteString("\ttry_files {path} {path}/index.html {path}/index.htm index.php\n")
		builder.WriteString("}\n\n")
	}

	// WordPress-specific rules
	if site.Type == sitetypes.SiteTypeWordpress {
		builder.WriteString("@disallowed {\n")
		builder.WriteString("\tpath /xmlrpc.php\n")
		builder.WriteString("\tpath *.sql\n")
		builder.WriteString("\tpath /wp-content/uploads/*.php\n")
		builder.WriteString("}\n\n")
		builder.WriteString("rewrite @disallowed '/index.php'\n\n")
	}

	// Custom redirects
	if len(redirects) > 0 {
		builder.WriteString("# Custom redirects\n")
		for _, r := range redirects {
			builder.WriteString(generateRedirectDirective(&r))
		}
		builder.WriteString("\n")
	}

	// File server
	builder.WriteString("file_server\n\n")

	// Logging with rotation
	builder.WriteString("log {\n")
	builder.WriteString(fmt.Sprintf("\toutput file %s/caddy.log {\n", site.Path+"/logs"))
	builder.WriteString("\t\troll_size 100mb\n")
	builder.WriteString("\t\troll_keep 30\n")
	builder.WriteString("\t\troll_keep_for 720h\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n")

	builder.WriteString("}\n")

	return builder.String()
}

// generateRedirectDirective generates a Caddy redirect directive for a redirect rule
func generateRedirectDirective(r *models.Redirect) string {
	var builder strings.Builder

	// Determine redirect type
	redirectType := ""
	if r.IsPermanent() {
		redirectType = " permanent"
	}

	// Check if this is a pattern redirect (contains * or {path})
	if strings.Contains(r.From, "*") || strings.Contains(r.From, "{") {
		// Pattern-based redirect with matcher
		matcherName := fmt.Sprintf("@redirect_%s", r.ID[:8])
		builder.WriteString(fmt.Sprintf("%s path %s\n", matcherName, r.From))
		builder.WriteString(fmt.Sprintf("redir %s %s%s\n", matcherName, r.To, redirectType))
	} else {
		// Exact path redirect
		builder.WriteString(fmt.Sprintf("redir %s %s%s\n", r.From, r.To, redirectType))
	}

	return builder.String()
}

// generateTLSSnippet generates the TLS snippet for a site
func generateTLSSnippet(site *models.Site) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("(tls-%s) {\n", site.ID))

	switch site.TLSSetting {
	case sitetypes.TLSSettingCustom:
		// TODO: Get active certificate and use its paths
		// For now, just add a placeholder comment
		builder.WriteString("\t# Custom TLS certificate\n")
	case sitetypes.TLSSettingInternal:
		builder.WriteString("\ttls internal\n")
	default:
		// Auto TLS or Off - no specific TLS config needed
		builder.WriteString("\t#\n")
	}

	builder.WriteString("}\n")

	return builder.String()
}

// updateSiteImports updates the global Caddy site imports file
func (j *InstallCaddyfileJob) updateSiteImports(ctx context.Context, server any, site *models.Site) error {
	// Get all sites for this server
	sites, err := j.Ctx.SiteRepo.FindByServer(ctx, site.ServerID)
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
		serverModel, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
		if err != nil {
			return err
		}
		_, err = j.Ctx.RunTaskOnServer(serverModel, task).AsRoot().Dispatch(ctx)
		return err
	}

	_, _ = serverModel, task
	return nil
}

// UpdateCaddyfileJob handles site Caddyfile updates
type UpdateCaddyfileJob struct {
	pkgjobs.BaseJob[*JobContext, CaddyfilePayload]
}

// NewUpdateCaddyfileJob creates a new UpdateCaddyfileJob
func NewUpdateCaddyfileJob(ctx *JobContext, payload CaddyfilePayload) *UpdateCaddyfileJob {
	return &UpdateCaddyfileJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the update Caddyfile job
func (j *UpdateCaddyfileJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Get redirects for Caddyfile generation (installed + pending)
	installedRedirects, _ := j.Ctx.RedirectRepo.FindBySiteForCaddy(ctx, site.ID)
	pendingRedirects, _ := j.Ctx.RedirectRepo.FindPendingBySite(ctx, site.ID)
	allRedirects := append(installedRedirects, pendingRedirects...)

	// Generate Caddyfile content
	caddyfileContent := j.generateCaddyfileContent(site, allRedirects)
	caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)

	// Create update Caddyfile task
	task := tasks.UpdateCaddyfile(tasks.UpdateCaddyfileConfig{
		CaddyfilePath:    caddyfilePath,
		CaddyfileContent: caddyfileContent,
	})

	// Execute the task on the server
	result, err := j.Ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to update Caddyfile", "site_id", site.ID)
		return err
	}

	exitCode := result.GetExitCode()
	if exitCode != 0 {
		j.Ctx.LogError(nil, "Caddyfile update failed", "site_id", site.ID, "exit_code", exitCode)
		return fmt.Errorf("caddyfile update failed with exit code %d", exitCode)
	}

	// Clear pending Caddyfile update flag
	site.PendingCaddyfileUpdateSince = nil
	if err := j.Ctx.SiteRepo.Update(ctx, site); err != nil {
		j.Ctx.LogError(err, "Failed to clear pending Caddyfile update flag")
	}

	// Mark pending redirects as installed
	if err := j.Ctx.RedirectRepo.UpdateStatusBySite(ctx, site.ID, "pending", "installed"); err != nil {
		j.Ctx.LogError(err, "Failed to update redirect statuses")
	}

	j.Ctx.LogInfo("Caddyfile updated successfully", "site_id", site.ID)

	return nil
}

// Failed handles job failure
func (j *UpdateCaddyfileJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Update Caddyfile job failed", "site_id", j.Payload.SiteID)
}

// generateCaddyfileContent generates the Caddyfile content for a site
func (j *UpdateCaddyfileJob) generateCaddyfileContent(site *models.Site, redirects []models.Redirect) string {
	return generateCaddyfile(site, redirects)
}

// UninstallCaddyfileJob handles site Caddyfile uninstallation
type UninstallCaddyfileJob struct {
	pkgjobs.BaseJob[*JobContext, CaddyfilePayload]
}

// NewUninstallCaddyfileJob creates a new UninstallCaddyfileJob
func NewUninstallCaddyfileJob(ctx *JobContext, payload CaddyfilePayload) *UninstallCaddyfileJob {
	return &UninstallCaddyfileJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the uninstall Caddyfile job
func (j *UninstallCaddyfileJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Ctx.SiteRepo.FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}

	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)

	// Create task to remove Caddyfile
	task := tasks.RemoveCaddyfile(tasks.RemoveCaddyfileConfig{
		CaddyfilePath: caddyfilePath,
	})

	// Execute the task on the server
	result, err := j.Ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Ctx.LogError(err, "Failed to uninstall Caddyfile", "site_id", site.ID)
		return err
	}

	exitCode := result.GetExitCode()
	if exitCode != 0 {
		j.Ctx.LogError(nil, "Caddyfile uninstallation failed", "site_id", site.ID, "exit_code", exitCode)
		return fmt.Errorf("caddyfile uninstallation failed with exit code %d", exitCode)
	}

	// Update site imports to remove this site
	if err := j.updateSiteImportsAfterRemoval(ctx, site); err != nil {
		j.Ctx.LogError(err, "Failed to update site imports after removal", "site_id", site.ID)
	}

	j.Ctx.LogInfo("Caddyfile uninstalled successfully", "site_id", site.ID)

	return nil
}

// Failed handles job failure
func (j *UninstallCaddyfileJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Uninstall Caddyfile job failed", "site_id", j.Payload.SiteID)
}

// updateSiteImportsAfterRemoval updates site imports after a site is removed
func (j *UninstallCaddyfileJob) updateSiteImportsAfterRemoval(ctx context.Context, removedSite *models.Site) error {
	// Get server
	server, err := j.Ctx.ServerRepos.Server().FindByID(ctx, removedSite.ServerID)
	if err != nil {
		return err
	}

	// Get all sites for this server (excluding the removed one)
	sites, err := j.Ctx.SiteRepo.FindByServer(ctx, removedSite.ServerID)
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
	_, err = j.Ctx.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	return err
}

// NewInstallCaddyfileTask creates an install Caddyfile job
// Uses TaskID for deduplication to prevent duplicate installs
func NewInstallCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	}, asynq.TaskID(fmt.Sprintf("install_caddyfile:%s", siteID)))
}

// NewUpdateCaddyfileTask creates an update Caddyfile job
// Uses TaskID for deduplication to prevent duplicate updates
func NewUpdateCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUpdateCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	}, asynq.TaskID(fmt.Sprintf("update_caddyfile:%s", siteID)))
}

// NewUninstallCaddyfileTask creates an uninstall Caddyfile job
// Uses TaskID for deduplication to prevent duplicate uninstalls
func NewUninstallCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	}, asynq.TaskID(fmt.Sprintf("uninstall_caddyfile:%s", siteID)))
}
