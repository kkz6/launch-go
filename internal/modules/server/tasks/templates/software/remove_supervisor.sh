#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove Supervisor"

waitForAptUnlock
sudo apt-get purge -y supervisor
sudo apt-get autoremove -y

echo "Supervisor removed successfully."
