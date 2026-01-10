package taskrunner

import (
	"bytes"
	"text/template"
)

// ScriptTemplates contains reusable bash script templates
var ScriptTemplates = map[string]string{
	// Wrapper script for background execution with callbacks
	"background_wrapper": `#!/bin/bash
set -e

# HTTP POST helper function
httpPostSilently() {
    local url="$1"
    local data="${2:-}"

    if command -v curl &> /dev/null; then
        if [ -n "$data" ]; then
            curl -s -X POST -H "Content-Type: application/json" -d "$data" "$url" > /dev/null 2>&1 || true
        else
            curl -s -X POST "$url" > /dev/null 2>&1 || true
        fi
    elif command -v wget &> /dev/null; then
        if [ -n "$data" ]; then
            wget -q --post-data="$data" --header="Content-Type: application/json" -O /dev/null "$url" 2>&1 || true
        else
            wget -q --post-data="" -O /dev/null "$url" 2>&1 || true
        fi
    fi
}

# Create temp script with actual task content
TASK_SCRIPT=$(mktemp)
cat > "$TASK_SCRIPT" << '{{.EOF}}'
{{.Script}}
{{.EOF}}

# Execute with timeout
timeout {{.Timeout}}s bash "$TASK_SCRIPT"
EXIT_CODE=$?

# Clean up
rm -f "$TASK_SCRIPT"

# Call appropriate webhook based on exit code
if [ $EXIT_CODE -eq 0 ]; then
    httpPostSilently "{{.FinishedURL}}"
elif [ $EXIT_CODE -eq 124 ]; then
    httpPostSilently "{{.TimeoutURL}}"
else
    httpPostSilently "{{.FailedURL}}" "{\"exit_code\":$EXIT_CODE}"
fi

exit $EXIT_CODE
`,

	// Server provisioning base script
	"provision_base": `#!/bin/bash
set -e

echo "Starting server provisioning..."

# Update system
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get upgrade -y

# Install essential packages
apt-get install -y \
    curl \
    wget \
    git \
    unzip \
    zip \
    htop \
    ncdu \
    software-properties-common \
    apt-transport-https \
    ca-certificates \
    gnupg \
    lsb-release

# Configure timezone
timedatectl set-timezone {{.Timezone}}

# Configure swap if needed
{{if .SwapSize}}
if [ ! -f /swapfile ]; then
    fallocate -l {{.SwapSize}} /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi
{{end}}

echo "Base provisioning complete"
`,

	// PHP installation script
	"install_php": `#!/bin/bash
set -e

PHP_VERSION="{{.PHPVersion}}"

echo "Installing PHP $PHP_VERSION..."

# Add PHP repository
add-apt-repository -y ppa:ondrej/php

apt-get update -y

# Install PHP and common extensions
apt-get install -y \
    php${PHP_VERSION}-fpm \
    php${PHP_VERSION}-cli \
    php${PHP_VERSION}-common \
    php${PHP_VERSION}-mysql \
    php${PHP_VERSION}-pgsql \
    php${PHP_VERSION}-sqlite3 \
    php${PHP_VERSION}-redis \
    php${PHP_VERSION}-memcached \
    php${PHP_VERSION}-gd \
    php${PHP_VERSION}-imagick \
    php${PHP_VERSION}-curl \
    php${PHP_VERSION}-zip \
    php${PHP_VERSION}-xml \
    php${PHP_VERSION}-mbstring \
    php${PHP_VERSION}-bcmath \
    php${PHP_VERSION}-intl \
    php${PHP_VERSION}-soap \
    php${PHP_VERSION}-readline

# Configure PHP-FPM
sed -i "s/;cgi.fix_pathinfo=1/cgi.fix_pathinfo=0/" /etc/php/${PHP_VERSION}/fpm/php.ini

# Restart PHP-FPM
systemctl restart php${PHP_VERSION}-fpm
systemctl enable php${PHP_VERSION}-fpm

echo "PHP $PHP_VERSION installed successfully"
`,

	// Caddy installation script
	"install_caddy": `#!/bin/bash
set -e

echo "Installing Caddy..."

# Install Caddy
apt-get install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt-get update -y
apt-get install -y caddy

# Enable and start Caddy
systemctl enable caddy
systemctl start caddy

echo "Caddy installed successfully"
`,

	// Deploy site script
	"deploy_site": `#!/bin/bash
set -e

SITE_PATH="{{.SitePath}}"
RELEASES_PATH="${SITE_PATH}/releases"
SHARED_PATH="${SITE_PATH}/shared"
CURRENT_PATH="${SITE_PATH}/current"
RELEASE="{{.Release}}"
RELEASE_PATH="${RELEASES_PATH}/${RELEASE}"
REPO_URL="{{.RepoURL}}"
BRANCH="{{.Branch}}"

echo "Starting deployment..."

# Create directories
mkdir -p "$RELEASES_PATH" "$SHARED_PATH/storage" "$SHARED_PATH"

# Clone repository
echo "Cloning repository..."
git clone --depth 1 --branch "$BRANCH" "$REPO_URL" "$RELEASE_PATH"

cd "$RELEASE_PATH"

# Link shared directories
echo "Linking shared directories..."
rm -rf "$RELEASE_PATH/storage"
ln -s "$SHARED_PATH/storage" "$RELEASE_PATH/storage"

# Link .env if exists
if [ -f "$SHARED_PATH/.env" ]; then
    ln -sf "$SHARED_PATH/.env" "$RELEASE_PATH/.env"
fi

# Install Composer dependencies
if [ -f "composer.json" ]; then
    echo "Installing Composer dependencies..."
    composer install --no-dev --optimize-autoloader --no-interaction
fi

# Install NPM dependencies and build
if [ -f "package.json" ]; then
    echo "Installing NPM dependencies..."
    npm ci

    if grep -q '"build"' package.json; then
        echo "Building assets..."
        npm run build
    fi
fi

# Laravel specific commands
if [ -f "artisan" ]; then
    echo "Running Laravel optimizations..."
    php artisan config:cache
    php artisan route:cache
    php artisan view:cache

    {{if .RunMigrations}}
    echo "Running migrations..."
    php artisan migrate --force
    {{end}}
fi

# Update symlink
echo "Updating current symlink..."
ln -sfn "$RELEASE_PATH" "$CURRENT_PATH"

# Restart PHP-FPM
echo "Restarting PHP-FPM..."
systemctl reload php{{.PHPVersion}}-fpm

echo "Deployment complete!"
`,

	// Rollback script
	"rollback": `#!/bin/bash
set -e

SITE_PATH="{{.SitePath}}"
CURRENT_PATH="${SITE_PATH}/current"
RELEASE_PATH="{{.ReleasePath}}"

echo "Rolling back to release: {{.Release}}"

# Verify release exists
if [ ! -d "$RELEASE_PATH" ]; then
    echo "Error: Release directory does not exist"
    exit 1
fi

# Update symlink
ln -sfn "$RELEASE_PATH" "$CURRENT_PATH"

# Restart PHP-FPM
systemctl reload php{{.PHPVersion}}-fpm

echo "Rollback complete"
`,
}

// RenderScript renders a script template with the given data
func RenderScript(templateName string, data map[string]interface{}) (string, error) {
	tmplStr, ok := ScriptTemplates[templateName]
	if !ok {
		return "", nil
	}

	tmpl, err := template.New(templateName).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// CreateBackgroundWrapper wraps a script with callback handling
func CreateBackgroundWrapper(script string, timeout int, callbacks CallbackURLs) (string, error) {
	data := map[string]interface{}{
		"Script":      script,
		"Timeout":     timeout,
		"FinishedURL": callbacks.Finished,
		"FailedURL":   callbacks.Failed,
		"TimeoutURL":  callbacks.Timeout,
		"EOF":         "LAUNCH_TASK_EOF",
	}

	return RenderScript("background_wrapper", data)
}
