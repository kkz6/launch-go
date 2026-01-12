#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove MySQL"

waitForAptUnlock
sudo systemctl stop mysql 2>/dev/null || true
sudo apt-get purge -y mysql-server mysql-client mysql-common mysql-community-server mysql-community-client
sudo apt-get autoremove -y
sudo rm -rf /var/lib/mysql
sudo rm -rf /etc/mysql

echo "MySQL removed successfully."
