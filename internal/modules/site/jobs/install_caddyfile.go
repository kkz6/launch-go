package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
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
	Deps    *JobDeps
	Payload CaddyfilePayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewInstallCaddyfileJob creates a new InstallCaddyfileJob
func NewInstallCaddyfileJob(p CaddyfilePayload) pkgjobs.Handler {
	return &InstallCaddyfileJob{Deps: deps, Payload: p}
}

// Handle executes the install Caddyfile job
func (j *InstallCaddyfileJob) Handle(ctx context.Context) error {
	j.Deps.Logger.Info().Str("site_id", j.Payload.SiteID).Msg("InstallCaddyfile job started")

	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	// Get redirects for Caddyfile generation (installed + pending)
	installedRedirects, _ := j.Deps.Repos.Redirect().FindBySiteForCaddy(ctx, site.ID)
	pendingRedirects, _ := j.Deps.Repos.Redirect().FindPendingBySite(ctx, site.ID)
	allRedirects := append(installedRedirects, pendingRedirects...)

	// Resolve load balancer IP if site is behind a load balancer
	loadBalancerIP := j.resolveLoadBalancerIP(ctx, site)

	// Generate Caddyfile content
	caddyfileContent := j.generateCaddyfileContent(site, allRedirects, loadBalancerIP)
	caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)

	// Create update Caddyfile task
	task := tasks.UpdateCaddyfile(tasks.UpdateCaddyfileConfig{
		CaddyfilePath:    caddyfilePath,
		CaddyfileContent: caddyfileContent,
		SiteUser:         site.User,
	})

	// Execute the task on the server
	result, err := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to install Caddyfile")
		return err
	}

	exitCode := result.GetExitCode()
	if exitCode != 0 {
		j.Deps.Logger.Error().Str("site_id", site.ID).Int("exit_code", exitCode).Msg("Caddyfile installation failed")
		return fmt.Errorf("caddyfile installation failed with exit code %d", exitCode)
	}

	// Update site imports
	if err := j.updateSiteImports(ctx, server, site); err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to update site imports")
	}

	// Mark site as installed and clear pending flags
	now := time.Now()
	site.InstalledAt = &now
	site.InstallationFailedAt = nil
	site.PendingCaddyfileUpdateSince = nil
	if err := j.Deps.Repos.Site().Update(ctx, site); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update site installed status")
	}

	// Mark pending redirects as installed
	if err := j.Deps.Repos.Redirect().UpdateStatusBySite(ctx, site.ID, "pending", "installed"); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update redirect statuses")
	}

	// Broadcast site.installed event
	j.Deps.BroadcastServerEvent(server, "site.installed", map[string]interface{}{
		"team_id":   server.TeamID,
		"site_id":   site.ID,
		"server_id": server.ID,
		"address":   site.Address,
	})

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Caddyfile installed successfully")

	return nil
}

// Failed handles job failure
func (j *InstallCaddyfileJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Install Caddyfile job failed")

	// Mark site installation as failed
	site, findErr := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if findErr != nil {
		return
	}

	now := time.Now()
	site.InstalledAt = nil
	site.InstallationFailedAt = &now
	_ = j.Deps.Repos.Site().Update(ctx, site)

	// Broadcast site.installation_failed event
	server, serverErr := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if serverErr == nil {
		j.Deps.BroadcastServerEvent(server, "site.installation_failed", map[string]interface{}{
			"team_id":   server.TeamID,
			"site_id":   site.ID,
			"server_id": server.ID,
			"address":   site.Address,
			"error":     err.Error(),
		})
	}
}

// generateCaddyfileContent generates the Caddyfile content for a site
func (j *InstallCaddyfileJob) generateCaddyfileContent(site *models.Site, redirects []models.Redirect, loadBalancerIP string) string {
	return generateCaddyfile(site, redirects, loadBalancerIP)
}

// generateCaddyfile generates the Caddyfile content for a site (shared function).
// When loadBalancerIP is non-empty and the site is load balanced, a dedicated
// port-8080 Caddyfile is generated with HTTP only and IP restriction.
func generateCaddyfile(site *models.Site, redirects []models.Redirect, loadBalancerIP string) string {
	if site.IsLoadBalanced() && loadBalancerIP != "" {
		return generateLoadBalancedCaddyfile(site, redirects, loadBalancerIP)
	}

	return generateStandardCaddyfile(site, redirects)
}

// generateLoadBalancedCaddyfile generates a Caddyfile for a site behind a load balancer.
// Uses dedicated port 8080 with HTTP only (no TLS) and IP restriction from the LB.
func generateLoadBalancedCaddyfile(site *models.Site, redirects []models.Redirect, loadBalancerIP string) string {
	var builder strings.Builder

	// Load balanced: HTTP only on dedicated port 8080
	builder.WriteString(fmt.Sprintf("http://%s:8080 {\n", site.Address))

	// IP restriction — only accept requests from the load balancer
	builder.WriteString(fmt.Sprintf("\t@notlb not remote_ip %s\n", loadBalancerIP))
	builder.WriteString("\trespond @notlb \"Forbidden\" 403\n\n")

	// Root directory
	builder.WriteString(fmt.Sprintf("\troot * %s\n", site.GetWebDirectory()))
	builder.WriteString("\tencode zstd gzip\n\n")

	// Security headers
	builder.WriteString("\theader {\n")
	builder.WriteString("\t\t-Server\n")
	builder.WriteString("\t\tX-Content-Type-Options nosniff\n")
	builder.WriteString("\t\tX-Frame-Options SAMEORIGIN\n")
	builder.WriteString("\t\tX-Powered-By \"Launch\"\n")
	builder.WriteString("\t\tX-XSS-Protection \"1; mode=block\"\n")
	builder.WriteString("\t}\n\n")

	// PHP FastCGI for non-static sites
	if site.Type != sitetypes.SiteTypeStatic && site.PhpVersion != nil && site.PhpVersion.IsValid() {
		phpSocket := site.PhpVersion.SocketPath()
		builder.WriteString(fmt.Sprintf("\tphp_fastcgi unix/%s {\n", phpSocket))
		builder.WriteString("\t\tresolve_root_symlink\n")
		builder.WriteString("\t\ttry_files {path} {path}/index.html {path}/index.htm index.php\n")
		builder.WriteString("\t}\n\n")
	}

	// WordPress-specific rules
	if site.Type == sitetypes.SiteTypeWordpress {
		builder.WriteString("\t@disallowed {\n")
		builder.WriteString("\t\tpath /xmlrpc.php\n")
		builder.WriteString("\t\tpath *.sql\n")
		builder.WriteString("\t\tpath /wp-content/uploads/*.php\n")
		builder.WriteString("\t}\n\n")
		builder.WriteString("\trewrite @disallowed '/index.php'\n\n")
	}

	// Custom redirects
	if len(redirects) > 0 {
		builder.WriteString("\t# Custom redirects\n")
		for _, r := range redirects {
			builder.WriteString("\t" + generateRedirectDirective(&r))
		}
		builder.WriteString("\n")
	}

	// File server
	builder.WriteString("\tfile_server\n\n")

	// Logging with rotation
	builder.WriteString("\tlog {\n")
	builder.WriteString(fmt.Sprintf("\t\toutput file %s/caddy.log {\n", site.Path+"/logs"))
	builder.WriteString("\t\t\troll_size 100mb\n")
	builder.WriteString("\t\t\troll_keep 30\n")
	builder.WriteString("\t\t\troll_keep_for 720h\n")
	builder.WriteString("\t\t}\n")
	builder.WriteString("\t}\n")

	builder.WriteString("}\n")

	return builder.String()
}

// generateStandardCaddyfile generates the standard Caddyfile for a directly-served site.
func generateStandardCaddyfile(site *models.Site, redirects []models.Redirect) string {
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

	// phpMyAdmin-specific security rules
	if site.Type == sitetypes.SiteTypePhpMyAdmin {
		builder.WriteString("@phpmyadmin_blocked {\n")
		builder.WriteString("\tpath /setup/*\n")
		builder.WriteString("\tpath /config.inc.php\n")
		builder.WriteString("\tpath /libraries/*\n")
		builder.WriteString("\tpath /templates/*\n")
		builder.WriteString("}\n\n")
		builder.WriteString("respond @phpmyadmin_blocked 403\n\n")
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
func (j *InstallCaddyfileJob) updateSiteImports(ctx context.Context, server *servermodels.Server, site *models.Site) error {
	// Get all sites for this server
	sites, err := j.Deps.Repos.Site().FindByServer(ctx, site.ServerID)
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

	// Execute on server
	_, err = j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	return err
}

// resolveLoadBalancerIP looks up the load balancer server's public IP for a load-balanced site.
// Returns empty string if the site is not load balanced or the IP cannot be resolved.
func (j *InstallCaddyfileJob) resolveLoadBalancerIP(ctx context.Context, site *models.Site) string {
	if !site.IsLoadBalanced() {
		return ""
	}

	upstream, err := j.Deps.ServerRepos.LoadBalancerUpstream().FindByIDWithBackends(ctx, *site.LoadBalancedUpstreamID)
	if err != nil || upstream == nil || upstream.Server == nil {
		j.Deps.Logger.Warn().Str("site_id", site.ID).Msg("Could not resolve load balancer IP for site")
		return ""
	}

	if upstream.Server.PublicIPv4 == nil {
		return ""
	}

	return *upstream.Server.PublicIPv4
}

// UpdateCaddyfileJob handles site Caddyfile updates
type UpdateCaddyfileJob struct {
	Deps    *JobDeps
	Payload CaddyfilePayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewUpdateCaddyfileJob creates a new UpdateCaddyfileJob
func NewUpdateCaddyfileJob(p CaddyfilePayload) pkgjobs.Handler {
	return &UpdateCaddyfileJob{Deps: deps, Payload: p}
}

// Handle executes the update Caddyfile job
func (j *UpdateCaddyfileJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	// Get redirects for Caddyfile generation (installed + pending)
	installedRedirects, _ := j.Deps.Repos.Redirect().FindBySiteForCaddy(ctx, site.ID)
	pendingRedirects, _ := j.Deps.Repos.Redirect().FindPendingBySite(ctx, site.ID)
	allRedirects := append(installedRedirects, pendingRedirects...)

	// Resolve load balancer IP if site is behind a load balancer
	loadBalancerIP := j.resolveLoadBalancerIP(ctx, site)

	// Generate Caddyfile content
	caddyfileContent := j.generateCaddyfileContent(site, allRedirects, loadBalancerIP)
	caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)

	// Create update Caddyfile task
	task := tasks.UpdateCaddyfile(tasks.UpdateCaddyfileConfig{
		CaddyfilePath:    caddyfilePath,
		CaddyfileContent: caddyfileContent,
		SiteUser:         site.User,
	})

	// Execute the task on the server
	result, err := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to update Caddyfile")
		return err
	}

	exitCode := result.GetExitCode()
	if exitCode != 0 {
		j.Deps.Logger.Error().Str("site_id", site.ID).Int("exit_code", exitCode).Msg("Caddyfile update failed")
		return fmt.Errorf("caddyfile update failed with exit code %d", exitCode)
	}

	// Clear pending Caddyfile update flag
	site.PendingCaddyfileUpdateSince = nil
	if err := j.Deps.Repos.Site().Update(ctx, site); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to clear pending Caddyfile update flag")
	}

	// Mark pending redirects as installed
	if err := j.Deps.Repos.Redirect().UpdateStatusBySite(ctx, site.ID, "pending", "installed"); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to update redirect statuses")
	}

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Caddyfile updated successfully")

	return nil
}

// Failed handles job failure
func (j *UpdateCaddyfileJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Update Caddyfile job failed")
}

// generateCaddyfileContent generates the Caddyfile content for a site
func (j *UpdateCaddyfileJob) generateCaddyfileContent(site *models.Site, redirects []models.Redirect, loadBalancerIP string) string {
	return generateCaddyfile(site, redirects, loadBalancerIP)
}

// resolveLoadBalancerIP looks up the load balancer server's public IP for a load-balanced site.
func (j *UpdateCaddyfileJob) resolveLoadBalancerIP(ctx context.Context, site *models.Site) string {
	if !site.IsLoadBalanced() {
		return ""
	}

	upstream, err := j.Deps.ServerRepos.LoadBalancerUpstream().FindByIDWithBackends(ctx, *site.LoadBalancedUpstreamID)
	if err != nil || upstream == nil || upstream.Server == nil {
		j.Deps.Logger.Warn().Str("site_id", site.ID).Msg("Could not resolve load balancer IP for site")
		return ""
	}

	if upstream.Server.PublicIPv4 == nil {
		return ""
	}

	return *upstream.Server.PublicIPv4
}

// UninstallCaddyfileJob handles site Caddyfile uninstallation
type UninstallCaddyfileJob struct {
	Deps    *JobDeps
	Payload CaddyfilePayload

	// Model fields for Failed() callback
	site   *models.Site
	server *servermodels.Server
}

// NewUninstallCaddyfileJob creates a new UninstallCaddyfileJob
func NewUninstallCaddyfileJob(p CaddyfilePayload) pkgjobs.Handler {
	return &UninstallCaddyfileJob{Deps: deps, Payload: p}
}

// Handle executes the uninstall Caddyfile job
func (j *UninstallCaddyfileJob) Handle(ctx context.Context) error {
	// Get site
	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)

	// Create task to remove Caddyfile
	task := tasks.RemoveCaddyfile(tasks.RemoveCaddyfileConfig{
		CaddyfilePath: caddyfilePath,
	})

	// Execute the task on the server
	result, err := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to uninstall Caddyfile")
		return err
	}

	exitCode := result.GetExitCode()
	if exitCode != 0 {
		j.Deps.Logger.Error().Str("site_id", site.ID).Int("exit_code", exitCode).Msg("Caddyfile uninstallation failed")
		return fmt.Errorf("caddyfile uninstallation failed with exit code %d", exitCode)
	}

	// Update site imports to remove this site
	if err := j.updateSiteImportsAfterRemoval(ctx, site); err != nil {
		j.Deps.Logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to update site imports after removal")
	}

	j.Deps.Logger.Info().Str("site_id", site.ID).Msg("Caddyfile uninstalled successfully")

	return nil
}

// Failed handles job failure
func (j *UninstallCaddyfileJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Str("site_id", j.Payload.SiteID).Msg("Uninstall Caddyfile job failed")
}

// updateSiteImportsAfterRemoval updates site imports after a site is removed
func (j *UninstallCaddyfileJob) updateSiteImportsAfterRemoval(ctx context.Context, removedSite *models.Site) error {
	// Get server
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, removedSite.ServerID)
	if err != nil {
		return err
	}

	// Get all sites for this server (excluding the removed one)
	sites, err := j.Deps.Repos.Site().FindByServer(ctx, removedSite.ServerID)
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
	_, err = j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	return err
}

// NewInstallCaddyfileTask creates an install Caddyfile job
// Uses TaskID for deduplication to prevent duplicate installs
func NewInstallCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	}, asynq.TaskID(pkgjobs.Dedup("install_caddyfile", siteID)))
}

// NewUpdateCaddyfileTask creates an update Caddyfile job
// Uses TaskID for deduplication to prevent duplicate updates
func NewUpdateCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUpdateCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	}, asynq.TaskID(pkgjobs.Dedup("update_caddyfile", siteID)))
}

// NewUninstallCaddyfileTask creates an uninstall Caddyfile job
// Uses TaskID for deduplication to prevent duplicate uninstalls
func NewUninstallCaddyfileTask(siteID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeUninstallCaddyfile, CaddyfilePayload{
		SiteID: siteID,
		UserID: userID,
	}, asynq.TaskID(pkgjobs.Dedup("uninstall_caddyfile", siteID)))
}
