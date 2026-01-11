{{/*
  Install Caddy 2 web server on Ubuntu server.
  This is a Go template file - use with text/template.

  Required template variables:
  - .Username: Server username for Caddy service configuration
  - .PublicIPv4: Server's public IPv4 address
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
# Install Caddy 2
# ==============================================================================

echo "Install Caddy webserver"

waitForAptUnlock
sudo rm -f /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --batch --yes --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
waitForAptUnlock
sudo apt-get update
waitForAptUnlock
sudo apt-get install -y caddy=2.*

echo "Install default Caddyfile"

# Set up the Caddyfile for the user
sudo tee /etc/caddy/Sites.caddy > /dev/null <<EOF
# import /home/{{ .Username }}/example.com/Caddyfile
EOF

sudo tee /etc/caddy/Caddyfile > /dev/null <<EOF
{{ .PublicIPv4 }}:80 {
    root * /home/{{ .Username }}/default
    file_server
}

# Do not remove this Sites.caddy import
import /etc/caddy/Sites.caddy
EOF

echo "Update Caddy service config to run as user"

sudo service caddy stop
sudo mkdir -p /etc/systemd/system/caddy.service.d

sudo tee /etc/systemd/system/caddy.service.d/override.conf > /dev/null <<EOF
[Service]
User={{ .Username }}
Group={{ .Username }}
EOF

# Reload systemd configuration and start Caddy service
sudo systemctl daemon-reload
sudo service caddy start

# Add caddy reload command to sudoers file without requiring password
sudo tee -a /etc/sudoers.d/caddy > /dev/null <<EOF
{{ .Username }} ALL=(root) NOPASSWD: /usr/sbin/service caddy reload
EOF

echo "Caddy 2 installation complete."
