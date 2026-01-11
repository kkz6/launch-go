{{/*
  Install Redis on Ubuntu server.
  This is a Go template file - use with text/template.

  No template variables required.
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
# Install Redis
# ==============================================================================

echo "Install Redis"

waitForAptUnlock
sudo apt-get install -y redis-server
sudo sed -i 's/^bind 127.0.0.1/bind 0.0.0.0/' /etc/redis/redis.conf
sudo service redis-server restart
sudo systemctl enable redis-server

echo "Redis installation complete."
