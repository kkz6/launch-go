#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install PostgreSQL 16"

waitForAptUnlock

DB_PASSWORD="{{ .DatabasePassword }}"
DATABASE_NAME="{{ .DatabaseName }}"
LAUNCH_USER="{{ .DatabaseName }}"

# Add PostgreSQL official repository
sudo apt-get install -y wget gnupg2 lsb-release

# Import PostgreSQL repository signing key (remove existing if present)
sudo rm -f /usr/share/keyrings/postgresql-archive-keyring.gpg
wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | sudo gpg --batch --dearmor -o /usr/share/keyrings/postgresql-archive-keyring.gpg

# Add PostgreSQL repository
echo "deb [signed-by=/usr/share/keyrings/postgresql-archive-keyring.gpg] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" | sudo tee /etc/apt/sources.list.d/pgdg.list

waitForAptUnlock
sudo apt-get update

waitForAptUnlock
sudo apt-get install -y postgresql-16 postgresql-contrib-16

# Wait for PostgreSQL to start
sleep 2

# Configure PostgreSQL authentication (pg_hba.conf)
PG_HBA_CONF=$(sudo -u postgres psql -t -P format=unaligned -c "SHOW hba_file")

# Backup original configuration
sudo cp "$PG_HBA_CONF" "${PG_HBA_CONF}.backup"

# Update pg_hba.conf to use md5 authentication for all connections
sudo bash -c "cat > $PG_HBA_CONF << 'EOF'
# PostgreSQL Client Authentication Configuration File
# TYPE  DATABASE        USER            ADDRESS                 METHOD

# Local connections
local   all             postgres                                peer
local   all             all                                     md5

# IPv4 local connections
host    all             all             127.0.0.1/32            md5
host    all             all             0.0.0.0/0               md5

# IPv6 local connections
host    all             all             ::1/128                 md5
host    all             all             ::/0                    md5
EOF"

# Configure PostgreSQL to listen on all interfaces
POSTGRESQL_CONF=$(sudo -u postgres psql -t -P format=unaligned -c "SHOW config_file")
sudo sed -i "s/#listen_addresses = 'localhost'/listen_addresses = '*'/" "$POSTGRESQL_CONF"
sudo sed -i "s/listen_addresses = 'localhost'/listen_addresses = '*'/" "$POSTGRESQL_CONF"

# Set max connections
if sudo grep -q "^max_connections" "$POSTGRESQL_CONF" 2>/dev/null; then
    sudo sed -i "s/^max_connections.*=.*/max_connections = 200/" "$POSTGRESQL_CONF"
fi

# Restart PostgreSQL to apply changes
sudo systemctl restart postgresql

# Wait for PostgreSQL to be ready
sleep 3

# Create launch superuser with password
sudo -u postgres psql -c "CREATE USER ${LAUNCH_USER} WITH SUPERUSER CREATEDB CREATEROLE PASSWORD '${DB_PASSWORD}';"

# Create default database
sudo -u postgres psql -c "CREATE DATABASE ${DATABASE_NAME} OWNER ${LAUNCH_USER};"

# Grant privileges
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${DATABASE_NAME} TO ${LAUNCH_USER};"

# Enable and start PostgreSQL service
sudo systemctl enable postgresql
sudo systemctl restart postgresql

echo "PostgreSQL 16 installation completed"
