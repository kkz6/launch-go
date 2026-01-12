#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove Redis"

waitForAptUnlock
sudo apt-get purge -y redis-server
sudo apt-get autoremove -y

echo "Redis removed successfully."
