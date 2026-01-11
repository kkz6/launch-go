{{/*
  Install Bun JavaScript runtime on Ubuntu server.
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
# Install Bun
# ==============================================================================

echo "Install Bun"

waitForAptUnlock

# Install dependencies
sudo apt-get install -y curl unzip

# Install Bun using official installer
# Install to /usr/local so it's available system-wide
curl -fsSL https://bun.sh/install | sudo BUN_INSTALL="/usr/local" bash

# Create symlink if bun is not in /usr/local/bin
if [ ! -f /usr/local/bin/bun ] && [ -f /usr/local/.bun/bin/bun ]; then
    sudo ln -sf /usr/local/.bun/bin/bun /usr/local/bin/bun
fi

# Add bun to PATH for all users if not already present
if ! grep -q "BUN_INSTALL" /etc/profile.d/bun.sh 2>/dev/null; then
    sudo bash -c 'cat > /etc/profile.d/bun.sh << "EOF"
export BUN_INSTALL="/usr/local"
export PATH="/usr/local/bin:$PATH"
EOF'
    sudo chmod +x /etc/profile.d/bun.sh
fi

# Verify installation
/usr/local/bin/bun --version || /usr/local/.bun/bin/bun --version || echo "Bun installed but path verification pending"

echo "Bun installation completed"
