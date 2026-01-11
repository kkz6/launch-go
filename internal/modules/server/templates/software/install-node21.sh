{{/*
  Install Node.js 21 on Ubuntu server.
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
# Install Node 21
# ==============================================================================

echo "Install Node 21"

waitForAptUnlock
sudo apt-get remove --purge -y nodejs
waitForAptUnlock
sudo curl --silent --location https://deb.nodesource.com/setup_21.x | sudo bash -
sudo apt-get update
waitForAptUnlock

sudo apt-get install -y --allow-downgrades --allow-remove-essential --allow-change-held-packages nodejs

echo "Install Node Packages"

sudo npm install -g fx n pm2 svgo yarn zx

echo "Node 21 installation complete."
