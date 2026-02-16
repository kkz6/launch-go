#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Caddy webserver (Load Balancer mode)"

waitForAptUnlock
sudo rm -f /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --batch --yes --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
waitForAptUnlock
sudo apt-get update
waitForAptUnlock
sudo apt-get install -y caddy=2.*

echo "Configure Caddy for Load Balancer mode"

# Create upstreams directory
sudo mkdir -p /etc/caddy/upstreams

# Set up the Upstreams import file
sudo tee /etc/caddy/Upstreams.caddy > /dev/null <<EOF
# Load balancer upstream configurations
# import /etc/caddy/upstreams/example_upstream.caddy
EOF

# Create load balancer Caddyfile
sudo tee /etc/caddy/Caddyfile > /dev/null <<EOF
{
    # Global options for load balancer
    admin off

    # Enable access logging
    log {
        output file /var/log/caddy/access.log {
            roll_size 100mb
            roll_keep 10
        }
    }
}

# Default handler for server IP (health check endpoint)
{{ .PublicIPv4 }}:80 {
    respond /health "OK" 200
    respond "Launch Load Balancer" 200
}

# Import all upstream configurations
import /etc/caddy/Upstreams.caddy
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

echo "Caddy Load Balancer installation complete"
