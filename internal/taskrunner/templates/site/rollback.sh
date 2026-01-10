{{/* Site rollback script */}}
{{define "site_rollback"}}
#!/bin/bash
set -e

{{template "helpers"}}

# ============================================================================
# Site Rollback Script
# Site: {{.SiteName}}
# Rolling back to: {{.ReleaseID}}
# ============================================================================

SITE_PATH="{{.SitePath}}"
CURRENT_PATH="${SITE_PATH}/current"
RELEASE_PATH="{{.ReleasePath}}"
RELEASE_ID="{{.ReleaseID}}"

print_info "Starting rollback for {{.SiteName}}"
print_info "Target release: ${RELEASE_ID}"

# ----------------------------------------------------------------------------
# Verify Release
# ----------------------------------------------------------------------------
if [ ! -d "$RELEASE_PATH" ]; then
    print_error "Release directory does not exist: ${RELEASE_PATH}"
    exit 1
fi

# Verify it's a valid Laravel installation (if applicable)
{{if .IsLaravel}}
if [ ! -f "${RELEASE_PATH}/artisan" ]; then
    print_error "Invalid release: artisan file not found"
    exit 1
fi
{{end}}

# ----------------------------------------------------------------------------
# Pre-Rollback Hook
# ----------------------------------------------------------------------------
{{if .PreRollbackScript}}
print_info "Running pre-rollback script..."
cd "$RELEASE_PATH"
{{.PreRollbackScript}}
{{end}}

# ----------------------------------------------------------------------------
# Switch Symlink
# ----------------------------------------------------------------------------
print_info "Switching to release ${RELEASE_ID}..."

# Get current release for logging
PREVIOUS_RELEASE=$(readlink -f "$CURRENT_PATH" 2>/dev/null || echo "none")

# Atomic symlink switch
ln -sfn "$RELEASE_PATH" "${SITE_PATH}/current.tmp"
mv -Tf "${SITE_PATH}/current.tmp" "$CURRENT_PATH"

print_success "Symlink updated"
print_info "Previous: ${PREVIOUS_RELEASE}"
print_info "Current: ${RELEASE_PATH}"

# ----------------------------------------------------------------------------
# Clear Caches
# ----------------------------------------------------------------------------
{{if .IsLaravel}}
print_info "Clearing Laravel caches..."
cd "$CURRENT_PATH"
php artisan optimize:clear || true
php artisan config:cache || true
php artisan route:cache || true
php artisan view:cache || true
{{end}}

# ----------------------------------------------------------------------------
# Restart Services
# ----------------------------------------------------------------------------
print_info "Restarting services..."

# Restart PHP-FPM
systemctl reload php{{.PHPVersion}}-fpm

{{if .RestartQueue}}
print_info "Restarting queue workers..."
{{if .UseSupervisor}}
supervisorctl restart {{.QueueWorkerName}}:*
{{else}}
php artisan queue:restart
{{end}}
{{end}}

# ----------------------------------------------------------------------------
# Post-Rollback Hook
# ----------------------------------------------------------------------------
{{if .PostRollbackScript}}
print_info "Running post-rollback script..."
cd "$CURRENT_PATH"
{{.PostRollbackScript}}
{{end}}

# ----------------------------------------------------------------------------
# Health Check
# ----------------------------------------------------------------------------
{{if .HealthCheckURL}}
print_info "Running health check..."
sleep 2  # Give services time to restart

HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "{{.HealthCheckURL}}" || echo "000")
if [ "$HTTP_STATUS" = "200" ]; then
    print_success "Health check passed"
else
    print_warning "Health check returned status: $HTTP_STATUS"
fi
{{end}}

print_success "Rollback completed successfully!"
print_info "Site is now running release: ${RELEASE_ID}"
{{end}}
