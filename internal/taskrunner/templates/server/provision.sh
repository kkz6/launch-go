{{/* Server provisioning script */}}
{{define "server_provision"}}
#!/bin/bash
set -e

{{template "helpers"}}

# ============================================================================
# Server Provisioning Script
# Server: {{.ServerName}}
# Provider: {{.Provider}}
# ============================================================================

print_info "Starting server provisioning for {{.ServerName}}"

# ----------------------------------------------------------------------------
# System Update
# ----------------------------------------------------------------------------
print_info "Updating system packages..."
export DEBIAN_FRONTEND=noninteractive
wait_for_apt
apt-get update -y
apt-get upgrade -y

# ----------------------------------------------------------------------------
# Essential Packages
# ----------------------------------------------------------------------------
print_info "Installing essential packages..."
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
    lsb-release \
    fail2ban \
    ufw

# ----------------------------------------------------------------------------
# Timezone Configuration
# ----------------------------------------------------------------------------
print_info "Setting timezone to {{.Timezone}}"
timedatectl set-timezone {{.Timezone}}

# ----------------------------------------------------------------------------
# Swap Configuration
# ----------------------------------------------------------------------------
{{if .SwapSize}}
if [ ! -f /swapfile ]; then
    print_info "Creating swap file ({{.SwapSize}})..."
    fallocate -l {{.SwapSize}} /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    print_success "Swap file created"
else
    print_info "Swap file already exists"
fi
{{end}}

# ----------------------------------------------------------------------------
# SSH Configuration
# ----------------------------------------------------------------------------
print_info "Configuring SSH..."
backup_file /etc/ssh/sshd_config

# Disable password authentication if key is set
{{if .DisablePasswordAuth}}
sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
{{end}}

# Set SSH port
{{if ne .SSHPort 22}}
sed -i 's/#Port 22/Port {{.SSHPort}}/' /etc/ssh/sshd_config
sed -i 's/Port 22/Port {{.SSHPort}}/' /etc/ssh/sshd_config
{{end}}

systemctl restart sshd

# ----------------------------------------------------------------------------
# Firewall Configuration
# ----------------------------------------------------------------------------
print_info "Configuring firewall..."
ufw default deny incoming
ufw default allow outgoing
ufw allow {{.SSHPort}}/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

# ----------------------------------------------------------------------------
# Fail2ban Configuration
# ----------------------------------------------------------------------------
print_info "Configuring fail2ban..."
systemctl enable fail2ban
systemctl start fail2ban

# ----------------------------------------------------------------------------
# Create Launch User
# ----------------------------------------------------------------------------
{{if .CreateUser}}
print_info "Creating user: {{.Username}}"
if ! id "{{.Username}}" &>/dev/null; then
    useradd -m -s /bin/bash {{.Username}}
    usermod -aG sudo {{.Username}}

    # Setup SSH key for user
    mkdir -p /home/{{.Username}}/.ssh
    echo "{{.PublicKey}}" >> /home/{{.Username}}/.ssh/authorized_keys
    chmod 700 /home/{{.Username}}/.ssh
    chmod 600 /home/{{.Username}}/.ssh/authorized_keys
    chown -R {{.Username}}:{{.Username}} /home/{{.Username}}/.ssh

    # Allow sudo without password
    echo "{{.Username}} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/{{.Username}}
    chmod 440 /etc/sudoers.d/{{.Username}}

    print_success "User {{.Username}} created"
else
    print_info "User {{.Username}} already exists"
fi
{{end}}

# ----------------------------------------------------------------------------
# Install PHP
# ----------------------------------------------------------------------------
{{if .PHPVersion}}
print_info "Installing PHP {{.PHPVersion}}..."
add-apt-repository -y ppa:ondrej/php
apt-get update -y

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

# Configure PHP
sed -i "s/;cgi.fix_pathinfo=1/cgi.fix_pathinfo=0/" /etc/php/{{.PHPVersion}}/fpm/php.ini
sed -i "s/upload_max_filesize = 2M/upload_max_filesize = 100M/" /etc/php/{{.PHPVersion}}/fpm/php.ini
sed -i "s/post_max_size = 8M/post_max_size = 100M/" /etc/php/{{.PHPVersion}}/fpm/php.ini
sed -i "s/memory_limit = 128M/memory_limit = 512M/" /etc/php/{{.PHPVersion}}/fpm/php.ini

systemctl restart php{{.PHPVersion}}-fpm
systemctl enable php{{.PHPVersion}}-fpm

print_success "PHP {{.PHPVersion}} installed"
{{end}}

# ----------------------------------------------------------------------------
# Install Composer
# ----------------------------------------------------------------------------
{{if .PHPVersion}}
print_info "Installing Composer..."
curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer
print_success "Composer installed"
{{end}}

# ----------------------------------------------------------------------------
# Install Node.js
# ----------------------------------------------------------------------------
{{if .NodeVersion}}
print_info "Installing Node.js {{.NodeVersion}}..."
curl -fsSL https://deb.nodesource.com/setup_{{.NodeVersion}}.x | bash -
apt-get install -y nodejs
print_success "Node.js installed"
{{end}}

# ----------------------------------------------------------------------------
# Install Caddy
# ----------------------------------------------------------------------------
{{if .InstallCaddy}}
print_info "Installing Caddy..."
apt-get install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt-get update -y
apt-get install -y caddy

systemctl enable caddy
systemctl start caddy

print_success "Caddy installed"
{{end}}

# ----------------------------------------------------------------------------
# Install Database
# ----------------------------------------------------------------------------
{{if eq .DatabaseType "mysql"}}
print_info "Installing MySQL..."
apt-get install -y mysql-server

# Secure MySQL installation
mysql -e "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '{{.DatabaseRootPassword}}';"
mysql -u root -p'{{.DatabaseRootPassword}}' -e "DELETE FROM mysql.user WHERE User='';"
mysql -u root -p'{{.DatabaseRootPassword}}' -e "DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost', '127.0.0.1', '::1');"
mysql -u root -p'{{.DatabaseRootPassword}}' -e "DROP DATABASE IF EXISTS test;"
mysql -u root -p'{{.DatabaseRootPassword}}' -e "DELETE FROM mysql.db WHERE Db='test' OR Db='test\\_%';"
mysql -u root -p'{{.DatabaseRootPassword}}' -e "FLUSH PRIVILEGES;"

systemctl enable mysql
print_success "MySQL installed and secured"
{{end}}

{{if eq .DatabaseType "postgres"}}
print_info "Installing PostgreSQL..."
apt-get install -y postgresql postgresql-contrib

# Set postgres password
sudo -u postgres psql -c "ALTER USER postgres PASSWORD '{{.DatabaseRootPassword}}';"

systemctl enable postgresql
print_success "PostgreSQL installed"
{{end}}

# ----------------------------------------------------------------------------
# Install Redis
# ----------------------------------------------------------------------------
{{if .InstallRedis}}
print_info "Installing Redis..."
apt-get install -y redis-server

# Configure Redis
sed -i 's/supervised no/supervised systemd/' /etc/redis/redis.conf

systemctl enable redis-server
systemctl restart redis-server

print_success "Redis installed"
{{end}}

# ----------------------------------------------------------------------------
# Install Supervisor
# ----------------------------------------------------------------------------
{{if .InstallSupervisor}}
print_info "Installing Supervisor..."
apt-get install -y supervisor

systemctl enable supervisor
systemctl start supervisor

print_success "Supervisor installed"
{{end}}

# ----------------------------------------------------------------------------
# Final Cleanup
# ----------------------------------------------------------------------------
print_info "Cleaning up..."
apt-get autoremove -y
apt-get autoclean -y

print_success "Server provisioning completed successfully!"
{{end}}
