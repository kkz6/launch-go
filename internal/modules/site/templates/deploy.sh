{{/* Site deployment script */}}
{{define "site_deploy"}}
#!/bin/bash
set -e

{{template "helpers"}}

# ============================================================================
# Site Deployment Script
# Site: {{.SiteName}}
# Domain: {{.Domain}}
# Branch: {{.Branch}}
# ============================================================================

SITE_PATH="{{.SitePath}}"
RELEASES_PATH="${SITE_PATH}/releases"
SHARED_PATH="${SITE_PATH}/shared"
CURRENT_PATH="${SITE_PATH}/current"
RELEASE="{{.Release}}"
RELEASE_PATH="${RELEASES_PATH}/${RELEASE}"
REPO_URL="{{.RepoURL}}"
BRANCH="{{.Branch}}"

print_info "Starting deployment for {{.SiteName}}"
print_info "Release: ${RELEASE}"

# ----------------------------------------------------------------------------
# Prepare Directories
# ----------------------------------------------------------------------------
print_info "Preparing directories..."
ensure_dir "$RELEASES_PATH"
ensure_dir "$SHARED_PATH/storage/app/public"
ensure_dir "$SHARED_PATH/storage/framework/cache"
ensure_dir "$SHARED_PATH/storage/framework/sessions"
ensure_dir "$SHARED_PATH/storage/framework/views"
ensure_dir "$SHARED_PATH/storage/logs"

# ----------------------------------------------------------------------------
# Clone Repository
# ----------------------------------------------------------------------------
print_info "Cloning repository..."

{{if .DeployKey}}
# Setup SSH key for git
export GIT_SSH_COMMAND="ssh -i {{.DeployKeyPath}} -o StrictHostKeyChecking=no"
{{end}}

git clone --depth 1 --branch "$BRANCH" "$REPO_URL" "$RELEASE_PATH"

cd "$RELEASE_PATH"

print_info "Checked out commit: $(git rev-parse HEAD)"

# ----------------------------------------------------------------------------
# Link Shared Resources
# ----------------------------------------------------------------------------
print_info "Linking shared resources..."

# Remove storage directory and link to shared
rm -rf "$RELEASE_PATH/storage"
ln -s "$SHARED_PATH/storage" "$RELEASE_PATH/storage"

# Link .env if exists
if [ -f "$SHARED_PATH/.env" ]; then
    ln -sf "$SHARED_PATH/.env" "$RELEASE_PATH/.env"
else
    print_warning ".env file not found in shared directory"
fi

{{range .SharedDirs}}
# Link shared directory: {{.}}
rm -rf "$RELEASE_PATH/{{.}}"
ensure_dir "$SHARED_PATH/{{.}}"
ln -s "$SHARED_PATH/{{.}}" "$RELEASE_PATH/{{.}}"
{{end}}

# ----------------------------------------------------------------------------
# Install Composer Dependencies
# ----------------------------------------------------------------------------
{{if .HasComposer}}
print_info "Installing Composer dependencies..."
composer install \
    --no-dev \
    --optimize-autoloader \
    --no-interaction \
    --prefer-dist \
    --no-progress
{{end}}

# ----------------------------------------------------------------------------
# Install NPM Dependencies
# ----------------------------------------------------------------------------
{{if .HasNpm}}
print_info "Installing NPM dependencies..."
{{if .UseNpmCi}}
npm ci --no-audit --no-fund
{{else}}
npm install --no-audit --no-fund
{{end}}
{{end}}

# ----------------------------------------------------------------------------
# Build Assets
# ----------------------------------------------------------------------------
{{if .BuildAssets}}
print_info "Building assets..."
npm run {{.BuildCommand}}
{{end}}

# ----------------------------------------------------------------------------
# Laravel Specific Tasks
# ----------------------------------------------------------------------------
{{if .IsLaravel}}
print_info "Running Laravel optimizations..."

# Generate optimized class loader
php artisan optimize:clear

# Cache configuration
php artisan config:cache

# Cache routes
php artisan route:cache

# Cache views
php artisan view:cache

{{if .RunMigrations}}
# Run migrations
print_info "Running database migrations..."
php artisan migrate --force
{{end}}

{{if .RunSeeders}}
# Run seeders
print_info "Running database seeders..."
php artisan db:seed --force
{{end}}

# Link storage
php artisan storage:link || true
{{end}}

# ----------------------------------------------------------------------------
# Custom Deployment Script
# ----------------------------------------------------------------------------
{{if .CustomScript}}
print_info "Running custom deployment script..."
{{.CustomScript}}
{{end}}

# ----------------------------------------------------------------------------
# Update Symlink
# ----------------------------------------------------------------------------
print_info "Activating release..."

# Atomic symlink switch
ln -sfn "$RELEASE_PATH" "${SITE_PATH}/current.tmp"
mv -Tf "${SITE_PATH}/current.tmp" "$CURRENT_PATH"

print_success "Release ${RELEASE} is now live"

# ----------------------------------------------------------------------------
# Restart Services
# ----------------------------------------------------------------------------
print_info "Restarting services..."

# Restart PHP-FPM
systemctl reload php{{.PHPVersion}}-fpm

{{if .RestartQueue}}
# Restart queue workers
print_info "Restarting queue workers..."
{{if .UseSupervisor}}
supervisorctl restart {{.QueueWorkerName}}:*
{{else}}
php artisan queue:restart
{{end}}
{{end}}

{{if .RestartScheduler}}
# The scheduler runs via cron, no restart needed
print_info "Scheduler will pick up changes automatically"
{{end}}

# ----------------------------------------------------------------------------
# Cleanup Old Releases
# ----------------------------------------------------------------------------
{{if gt .ReleasesToKeep 0}}
print_info "Cleaning up old releases (keeping {{.ReleasesToKeep}})..."
cd "$RELEASES_PATH"
ls -t | tail -n +$(({{.ReleasesToKeep}} + 1)) | xargs -r rm -rf
{{end}}

# ----------------------------------------------------------------------------
# Health Check
# ----------------------------------------------------------------------------
{{if .HealthCheckURL}}
print_info "Running health check..."
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "{{.HealthCheckURL}}" || echo "000")
if [ "$HTTP_STATUS" = "200" ]; then
    print_success "Health check passed"
else
    print_warning "Health check returned status: $HTTP_STATUS"
fi
{{end}}

print_success "Deployment completed successfully!"
print_info "Release: ${RELEASE}"
print_info "Path: ${RELEASE_PATH}"
{{end}}
