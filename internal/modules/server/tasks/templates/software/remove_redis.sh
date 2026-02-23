#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove Redis"

sudo service redis stop 2>/dev/null || true

waitForAptUnlock
sudo apt-get purge -y redis-server
sudo apt-get autoremove -y
sudo apt-get autoclean -y

sudo rm -rf /etc/redis
sudo rm -rf /var/lib/redis
sudo rm -rf /var/log/redis
sudo rm -rf /var/run/redis

echo "Redis removed successfully."
