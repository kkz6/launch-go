#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

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
