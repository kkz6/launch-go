#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove Redis"

sudo systemctl disable --now redis-server 2>/dev/null || true

waitForAptUnlock
aptGet purge -y redis-server
aptGet autoremove -y
aptGet autoclean -y

sudo rm -rf /etc/redis
sudo rm -rf /var/lib/redis
sudo rm -rf /var/log/redis
sudo rm -rf /var/run/redis

echo "Redis removed successfully."
