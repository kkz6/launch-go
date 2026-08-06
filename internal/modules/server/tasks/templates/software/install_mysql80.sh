#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install MySQL 8.0"

waitForAptUnlock

# Remove any existing MySQL repository configuration to prevent GPG key issues
sudo rm -f /etc/apt/sources.list.d/mysql.list
sudo rm -f /etc/apt/sources.list.d/mysql-apt-config.list

MYSQL_KEYRING=/usr/share/keyrings/mysql-archive-keyring.gpg
MYSQL_REPO_SETUP_SUCCESS=false

# MySQL signs its repo with one long-lived key whose expiry is extended in
# place rather than rotated. Public keyservers serve a stale copy of that
# self-signature — keyserver.ubuntu.com hands back a *lapsed* version of the
# very same key ID — so apt rejects the repo with EXPKEYSIG and silently
# falls back to whatever index it already had. Fetch from MySQL's own
# published key file, which carries the current expiry.
#
# Those files are year-suffixed and MySQL adds a new one periodically, so try
# newest first and verify the key is actually valid before accepting it.
sudo rm -f "${MYSQL_KEYRING}"
for keyUrl in \
    https://repo.mysql.com/RPM-GPG-KEY-mysql-2025 \
    https://repo.mysql.com/RPM-GPG-KEY-mysql-2023 \
    https://repo.mysql.com/RPM-GPG-KEY-mysql-2022; do
    if ! curl -fsSL "${keyUrl}" | sudo gpg --batch --yes --dearmor -o "${MYSQL_KEYRING}" 2>/dev/null; then
        sudo rm -f "${MYSQL_KEYRING}"
        continue
    fi
    if sudo gpg --show-keys "${MYSQL_KEYRING}" 2>/dev/null | grep -q "expired:"; then
        echo "MySQL signing key at ${keyUrl} has expired, trying an older key file..."
        sudo rm -f "${MYSQL_KEYRING}"
        continue
    fi
    echo "Installed MySQL signing key from ${keyUrl}"
    break
done

if [ -s "${MYSQL_KEYRING}" ]; then
    echo "deb [signed-by=${MYSQL_KEYRING}] http://repo.mysql.com/apt/ubuntu $(lsb_release -cs) mysql-8.0" | sudo tee /etc/apt/sources.list.d/mysql.list
    waitForAptUnlock
    aptGet update || true

    # apt-get update exits 0 even when a repository fails signature
    # verification — it only warns and reuses the previous index. Ask apt
    # whether the package is actually resolvable from repo.mysql.com rather
    # than trusting that exit code, which is how a broken repo previously
    # got this far and then failed at install time.
    if apt-cache policy mysql-community-server 2>/dev/null | grep -q "repo.mysql.com"; then
        MYSQL_REPO_SETUP_SUCCESS=true
    else
        echo "MySQL repo did not yield an installable mysql-community-server"
    fi
fi

if [ "$MYSQL_REPO_SETUP_SUCCESS" = false ]; then
    echo "MySQL official repo setup failed, falling back to Ubuntu's mysql-server package"
    sudo rm -f /etc/apt/sources.list.d/mysql.list
    waitForAptUnlock
    aptGet update
fi

ROOT_PASSWORD="{{ .RootPassword }}"
DATABASE_NAME="{{ .DatabaseName }}"
IP="{{ .PublicIPv4 }}"

waitForAptUnlock

if [ "$MYSQL_REPO_SETUP_SUCCESS" = true ]; then
    sudo debconf-set-selections <<< "mysql-community-server mysql-community-server/data-dir select ''"
    sudo debconf-set-selections <<< "mysql-community-server mysql-community-server/root-pass password ${ROOT_PASSWORD}"
    sudo debconf-set-selections <<< "mysql-community-server mysql-community-server/re-root-pass password ${ROOT_PASSWORD}"
    aptGet install -y mysql-community-server
else
    sudo debconf-set-selections <<< "mysql-server mysql-server/root_password password ${ROOT_PASSWORD}"
    sudo debconf-set-selections <<< "mysql-server mysql-server/root_password_again password ${ROOT_PASSWORD}"
    aptGet install -y mysql-server
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
    sudo sed -i "s/^max_connections.*=.*/max_connections={{ .MaxConnections }}/" /etc/mysql/my.cnf
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
