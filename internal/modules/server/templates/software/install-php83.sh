{{/*
  Install PHP 8.3 on Ubuntu server.
  This is a Go template file - use with text/template.

  Required template variables:
  - .Username: Server username for PHP-FPM pool configuration
  - .MaxChildrenPhpPool: Maximum number of PHP-FPM child processes
*/}}
set -e
export DEBIAN_FRONTEND=noninteractive

# ==============================================================================
# Common Functions
# ==============================================================================

# Wait for apt to be unlocked
function waitForAptUnlock()
{
    while ps -C apt,apt-get,dpkg >/dev/null 2>&1; do
        echo "apt, apt-get or dpkg is running..."
        sleep 5
    done

    while fuser /var/{lib/{dpkg,apt/lists},cache/apt/archives}/{lock,lock-frontend} >/dev/null 2>&1; do
        echo "Waiting: apt is locked..."
        sleep 5
    done

    if [ -f /var/log/unattended-upgrades/unattended-upgrades.log ]; then
        while fuser /var/log/unattended-upgrades/unattended-upgrades.log >/dev/null 2>&1; do
        echo "Waiting: unattended-upgrades is locked..."
        sleep 5
        done
    fi
}

# Ensure PHP PPA is installed
function ensurePhpPpaInstalled()
{
    if ! grep -q "ondrej/php" /etc/apt/sources.list.d/*.list 2>/dev/null && ! grep -q "ondrej/php" /etc/apt/sources.list.d/*.sources 2>/dev/null; then
        echo "Adding ondrej/php PPA..."
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive apt-get install -y software-properties-common
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive add-apt-repository ppa:ondrej/php -y
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
        echo "ondrej/php PPA installed successfully"
    else
        echo "ondrej/php PPA already installed, refreshing package lists..."
        waitForAptUnlock
        sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
    fi
}

# ==============================================================================
# Install PHP 8.3
# ==============================================================================

echo "Install PHP 8.3"

ensurePhpPpaInstalled

waitForAptUnlock

sudo DEBIAN_FRONTEND=noninteractive apt-get install -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" -y --allow-downgrades --allow-remove-essential --allow-change-held-packages \
    php8.3-bcmath \
    php8.3-cli \
    php8.3-curl \
    php8.3-dev \
    php8.3-fpm \
    php8.3-gd \
    php8.3-imap \
    php8.3-intl \
    php8.3-mbstring \
    php8.3-mysql \
    php8.3-sqlite3 \
    php8.3-common \
    php8.3-xml \
    php8.3-zip

echo "Install Redis for PHP 8.3"

waitForAptUnlock
yes '' | sudo DEBIAN_FRONTEND=noninteractive apt-get -y install php8.3-redis || echo "php8.3-redis not available, skipping..."

echo "Install Memcached for PHP 8.3"

waitForAptUnlock
yes '' | sudo DEBIAN_FRONTEND=noninteractive apt-get -y install php8.3-memcached || echo "php8.3-memcached not available, skipping..."

# ==============================================================================
# Update PHP Configuration
# ==============================================================================

echo "Update PHP CLI config"

# Update PHP CLI settings in php.ini
sudo sed -i "s/error_reporting = .*/error_reporting = E_ALL/" /etc/php/8.3/cli/php.ini
sudo sed -i "s/display_errors = .*/display_errors = On/" /etc/php/8.3/cli/php.ini
sudo sed -i "s/memory_limit = .*/memory_limit = 512M/" /etc/php/8.3/cli/php.ini
sudo sed -i "s/;date.timezone.*/date.timezone = UTC/" /etc/php/8.3/cli/php.ini
sudo sed -i "s/;cgi.fix_pathinfo=1/cgi.fix_pathinfo=0/" /etc/php/8.3/cli/php.ini

echo "Update PHP FPM config"

# Update PHP FPM settings in php.ini
sudo sed -i "s/error_reporting = .*/error_reporting = E_ALL/" /etc/php/8.3/fpm/php.ini
sudo sed -i "s/display_errors = .*/display_errors = Off/" /etc/php/8.3/fpm/php.ini
sudo sed -i "s/memory_limit = .*/memory_limit = 512M/" /etc/php/8.3/fpm/php.ini
sudo sed -i "s/;date.timezone.*/date.timezone = UTC/" /etc/php/8.3/fpm/php.ini
sudo sed -i "s/;cgi.fix_pathinfo=1/cgi.fix_pathinfo=0/" /etc/php/8.3/fpm/php.ini

echo "Update PHP FPM pool config"

# Update FPM pool configuration
sudo sed -i "s/;request_terminate_timeout.*/request_terminate_timeout = 60/" /etc/php/8.3/fpm/pool.d/www.conf
sudo sed -i "s/^user = www-data/user = {{ .Username }}/" /etc/php/8.3/fpm/pool.d/www.conf
sudo sed -i "s/^group = www-data/group = {{ .Username }}/" /etc/php/8.3/fpm/pool.d/www.conf
sudo sed -i "s/;listen\.owner.*/listen.owner = {{ .Username }}/" /etc/php/8.3/fpm/pool.d/www.conf
sudo sed -i "s/;listen\.group.*/listen.group = {{ .Username }}/" /etc/php/8.3/fpm/pool.d/www.conf
sudo sed -i "s/;listen\.mode.*/listen.mode = 0666/" /etc/php/8.3/fpm/pool.d/www.conf
sudo sed -i "s/^pm.max_children.*=.*/pm.max_children = {{ .MaxChildrenPhpPool }}/" /etc/php/8.3/fpm/pool.d/www.conf

echo "Update PHP session config"

# Update session directory permissions
sudo chmod 733 /var/lib/php/sessions

# Set the sticky bit on the session directory to prevent other users from deleting session files
sudo chmod +t /var/lib/php/sessions

echo "PHP configuration updates complete."

sudo service php8.3-fpm restart > /dev/null 2>&1

echo "{{ .Username }} ALL=NOPASSWD: /usr/sbin/service php8.3-fpm reload" | sudo tee /etc/sudoers.d/php-fpm > /dev/null

echo "PHP 8.3 installation complete."
