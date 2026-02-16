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
{
    log {
        output file /var/log/caddy/access.log {
            roll_size 100mb
            roll_keep 10
        }
    }
}

{{ .PublicIPv4 }}:80 {
    root * /home/{{ .Username }}/default
    file_server
}

# Do not remove this Sites.caddy import
import /etc/caddy/Sites.caddy
EOF

# Create log directory
sudo mkdir -p /var/log/caddy
sudo chown {{ .Username }}:{{ .Username }} /var/log/caddy

echo "Update Caddy service config to run as user"

sudo service caddy stop
sudo mkdir -p /etc/systemd/system/caddy.service.d

sudo tee /etc/systemd/system/caddy.service.d/override.conf > /dev/null <<EOF
[Service]
User={{ .Username }}
Group={{ .Username }}
StandardOutput=append:/var/log/caddy/caddy.log
StandardError=append:/var/log/caddy/caddy.log
EOF

# Reload systemd configuration and start Caddy service
sudo systemctl daemon-reload
sudo service caddy start

# Add caddy reload command to sudoers file without requiring password
sudo tee -a /etc/sudoers.d/caddy > /dev/null <<EOF
{{ .Username }} ALL=(root) NOPASSWD: /usr/sbin/service caddy reload
EOF
