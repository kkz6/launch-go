#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove Supervisor"

sudo service supervisor stop 2>/dev/null || true

waitForAptUnlock
aptGet purge -y supervisor
aptGet autoremove -y

sudo rm -rf /etc/supervisor
sudo rm -rf /var/log/supervisor
sudo rm -rf /var/run/supervisor

echo "Supervisor removed successfully."
