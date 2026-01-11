{{/*
  Install MySQL 8.0 on Ubuntu server.
  This is a Go template file - use with text/template.

  Required template variables:
  - .DatabasePassword: Root password for MySQL
  - .DatabaseName: Default database name to create
  - .PublicIPv4: Server's public IPv4 address
  - .MysqlMaxConnections: Maximum number of MySQL connections
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

# ==============================================================================
# Install MySQL 8.0
# ==============================================================================

echo "Install MySQL 8.0"

waitForAptUnlock

# Remove any existing MySQL repository configuration to prevent GPG key issues
sudo rm -f /etc/apt/sources.list.d/mysql.list
sudo rm -f /etc/apt/sources.list.d/mysql-apt-config.list

# Set up MySQL GPG key from Ubuntu's keyserver (most reliable method)
# Falls back to Ubuntu's mysql-server package if MySQL repo setup fails
sudo rm -f /usr/share/keyrings/mysql-archive-keyring.gpg
MYSQL_REPO_SETUP_SUCCESS=false

if sudo gpg --batch --yes --keyserver keyserver.ubuntu.com --recv-keys B7B3B788A8D3785C 2>/dev/null; then
    sudo gpg --batch --yes --export B7B3B788A8D3785C | sudo tee /usr/share/keyrings/mysql-archive-keyring.gpg > /dev/null
    echo "deb [signed-by=/usr/share/keyrings/mysql-archive-keyring.gpg] http://repo.mysql.com/apt/ubuntu $(lsb_release -cs) mysql-8.0" | sudo tee /etc/apt/sources.list.d/mysql.list
    waitForAptUnlock
    if sudo apt-get update 2>/dev/null; then
        MYSQL_REPO_SETUP_SUCCESS=true
    fi
fi

if [ "$MYSQL_REPO_SETUP_SUCCESS" = false ]; then
    echo "MySQL official repo setup failed, falling back to Ubuntu's mysql-server package"
    sudo rm -f /etc/apt/sources.list.d/mysql.list
    waitForAptUnlock
    sudo apt-get update
fi

ROOT_PASSWORD="{{ .DatabasePassword }}"
DATABASE_NAME="{{ .DatabaseName }}"
IP="{{ .PublicIPv4 }}"

waitForAptUnlock

if [ "$MYSQL_REPO_SETUP_SUCCESS" = true ]; then
    sudo debconf-set-selections <<< "mysql-community-server mysql-community-server/data-dir select ''"
    sudo debconf-set-selections <<< "mysql-community-server mysql-community-server/root-pass password ${ROOT_PASSWORD}"
    sudo debconf-set-selections <<< "mysql-community-server mysql-community-server/re-root-pass password ${ROOT_PASSWORD}"
    sudo apt-get install -y mysql-community-server
else
    sudo debconf-set-selections <<< "mysql-server mysql-server/root_password password ${ROOT_PASSWORD}"
    sudo debconf-set-selections <<< "mysql-server mysql-server/root_password_again password ${ROOT_PASSWORD}"
    sudo apt-get install -y mysql-server
fi

# Configure MySQL settings (idempotent - only add if not present)
if ! sudo grep -q "default_password_lifetime" /etc/mysql/mysql.conf.d/mysqld.cnf 2>/dev/null; then
    echo "default_password_lifetime = 0" | sudo tee -a /etc/mysql/mysql.conf.d/mysqld.cnf
fi

# Remove invalid mysql_native_password=ON if present (was incorrect syntax)
sudo sed -i '/mysql_native_password=ON/d' /etc/mysql/my.cnf 2>/dev/null || true

# Add disable_log_bin to my.cnf if not present
if ! sudo grep -q "disable_log_bin" /etc/mysql/my.cnf 2>/dev/null; then
    if ! sudo grep -q "\[mysqld\]" /etc/mysql/my.cnf 2>/dev/null; then
        echo "" | sudo tee -a /etc/mysql/my.cnf
        echo "[mysqld]" | sudo tee -a /etc/mysql/my.cnf
    fi
    echo "disable_log_bin" | sudo tee -a /etc/mysql/my.cnf
fi

# Set max_connections
if sudo grep -q "^max_connections" /etc/mysql/my.cnf 2>/dev/null; then
    sudo sed -i "s/^max_connections.*=.*/max_connections={{ .MysqlMaxConnections }}/" /etc/mysql/my.cnf
fi

# Configure bind-address
if sudo grep -q "^bind-address" /etc/mysql/mysql.conf.d/mysqld.cnf 2>/dev/null; then
    sudo sed -i '/^bind-address/s/bind-address.*=.*/bind-address = */' /etc/mysql/mysql.conf.d/mysqld.cnf
elif ! sudo grep -q "bind-address = \*" /etc/mysql/mysql.conf.d/mysqld.cnf 2>/dev/null; then
    echo "bind-address = *" | sudo tee -a /etc/mysql/mysql.conf.d/mysqld.cnf
fi

sudo mysql --user="root" --password="${ROOT_PASSWORD}" -e "CREATE USER IF NOT EXISTS 'root'@'${IP}' IDENTIFIED BY '${ROOT_PASSWORD}';"
sudo mysql --user="root" --password="${ROOT_PASSWORD}" -e "CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED BY '${ROOT_PASSWORD}';"
sudo mysql --user="root" --password="${ROOT_PASSWORD}" -e "GRANT ALL PRIVILEGES ON *.* TO root@'%' WITH GRANT OPTION;"
sudo mysql --user="root" --password="${ROOT_PASSWORD}" -e "GRANT ALL PRIVILEGES ON *.* TO root@'${IP}' WITH GRANT OPTION;"
sudo service mysql restart

sudo mysql --user="root" --password="${ROOT_PASSWORD}" -e "CREATE USER IF NOT EXISTS '${DATABASE_NAME}'@'${IP}' IDENTIFIED BY '${ROOT_PASSWORD}';"
sudo mysql --user="root" --password="${ROOT_PASSWORD}" -e "CREATE USER IF NOT EXISTS '${DATABASE_NAME}'@'%' IDENTIFIED BY '${ROOT_PASSWORD}';"
sudo mysql --user="root" --password="${ROOT_PASSWORD}" -e "GRANT ALL PRIVILEGES ON *.* TO '${DATABASE_NAME}'@'%' WITH GRANT OPTION;"
sudo mysql --user="root" --password="${ROOT_PASSWORD}" -e "FLUSH PRIVILEGES;"

sudo mysql --user="root" --password="${ROOT_PASSWORD}" -e "CREATE DATABASE IF NOT EXISTS ${DATABASE_NAME} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

sudo service mysql restart

echo "MySQL 8.0 installation complete."
