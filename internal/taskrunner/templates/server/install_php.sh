{{/* PHP installation script */}}
{{define "server_install_php"}}
#!/bin/bash
set -e

{{template "helpers"}}

# ============================================================================
# PHP Installation Script
# Version: {{.PHPVersion}}
# ============================================================================

print_info "Installing PHP {{.PHPVersion}}..."

# Add PHP repository
if [ ! -f /etc/apt/sources.list.d/ondrej-ubuntu-php-*.list ]; then
    add-apt-repository -y ppa:ondrej/php
fi

wait_for_apt
apt-get update -y

# Install PHP and extensions
apt-get install -y \
    php{{.PHPVersion}}-fpm \
    php{{.PHPVersion}}-cli \
    php{{.PHPVersion}}-common \
    php{{.PHPVersion}}-mysql \
    php{{.PHPVersion}}-pgsql \
    php{{.PHPVersion}}-sqlite3 \
    php{{.PHPVersion}}-redis \
    php{{.PHPVersion}}-memcached \
    php{{.PHPVersion}}-gd \
    php{{.PHPVersion}}-imagick \
    php{{.PHPVersion}}-curl \
    php{{.PHPVersion}}-zip \
    php{{.PHPVersion}}-xml \
    php{{.PHPVersion}}-mbstring \
    php{{.PHPVersion}}-bcmath \
    php{{.PHPVersion}}-intl \
    php{{.PHPVersion}}-soap \
    php{{.PHPVersion}}-readline

# Configure PHP-FPM
print_info "Configuring PHP-FPM..."

PHP_INI="/etc/php/{{.PHPVersion}}/fpm/php.ini"
backup_file "$PHP_INI"

sed -i "s/;cgi.fix_pathinfo=1/cgi.fix_pathinfo=0/" "$PHP_INI"
sed -i "s/upload_max_filesize = 2M/upload_max_filesize = {{.UploadMaxFilesize}}/" "$PHP_INI"
sed -i "s/post_max_size = 8M/post_max_size = {{.PostMaxSize}}/" "$PHP_INI"
sed -i "s/memory_limit = 128M/memory_limit = {{.MemoryLimit}}/" "$PHP_INI"
sed -i "s/max_execution_time = 30/max_execution_time = {{.MaxExecutionTime}}/" "$PHP_INI"

{{if .OpcacheEnabled}}
# Configure OPcache
print_info "Configuring OPcache..."
cat >> "$PHP_INI" << 'EOF'

[opcache]
opcache.enable=1
opcache.memory_consumption={{.OpcacheMemory}}
opcache.interned_strings_buffer=16
opcache.max_accelerated_files=10000
opcache.validate_timestamps={{if .OpcacheValidateTimestamps}}1{{else}}0{{end}}
opcache.revalidate_freq=0
opcache.save_comments=1
EOF
{{end}}

# Restart PHP-FPM
systemctl restart php{{.PHPVersion}}-fpm
systemctl enable php{{.PHPVersion}}-fpm

{{if .SetAsDefault}}
# Set as default PHP version
print_info "Setting PHP {{.PHPVersion}} as default..."
update-alternatives --set php /usr/bin/php{{.PHPVersion}}
update-alternatives --set phar /usr/bin/phar{{.PHPVersion}}
update-alternatives --set phar.phar /usr/bin/phar.phar{{.PHPVersion}}
{{end}}

print_success "PHP {{.PHPVersion}} installed successfully"

# Verify installation
php{{.PHPVersion}} -v
{{end}}
