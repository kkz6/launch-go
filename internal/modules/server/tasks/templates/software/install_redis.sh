#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

{{ commonFuncs }}

echo "Install Redis"

waitForAptUnlock
aptGet install -y redis-server
sudo sed -i 's/^bind 127.0.0.1/bind 0.0.0.0/' /etc/redis/redis.conf
sudo service redis-server restart
sudo systemctl enable redis-server

echo "Redis installation complete."
