#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove PostgreSQL"

waitForAptUnlock
sudo systemctl stop postgresql 2>/dev/null || true
aptGet purge -y postgresql postgresql-* postgresql-contrib-*
aptGet autoremove -y
sudo rm -rf /var/lib/postgresql
sudo rm -rf /etc/postgresql

echo "PostgreSQL removed successfully."
