#!/bin/bash
{{ shellDefaults }}

echo "Configure firewall with SSH port, HTTP and HTTPS"

sudo ufw allow {{ .SSHPort }}
sudo ufw allow 80
sudo ufw allow 443
sudo ufw --force enable
sudo service ufw restart
