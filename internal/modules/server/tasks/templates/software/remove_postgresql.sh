#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove PostgreSQL"

waitForAptUnlock
sudo systemctl stop postgresql 2>/dev/null || true
sudo apt-get purge -y postgresql postgresql-* postgresql-contrib-*
sudo apt-get autoremove -y
sudo rm -rf /var/lib/postgresql
sudo rm -rf /etc/postgresql

echo "PostgreSQL removed successfully."
