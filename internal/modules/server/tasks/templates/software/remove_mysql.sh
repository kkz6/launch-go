#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove MySQL"

waitForAptUnlock
sudo systemctl stop mysql 2>/dev/null || true
aptGet purge -y mysql-server mysql-client mysql-common mysql-community-server mysql-community-client
aptGet autoremove -y
sudo rm -rf /var/lib/mysql
sudo rm -rf /etc/mysql

echo "MySQL removed successfully."
