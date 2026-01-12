#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Supervisor"

waitForAptUnlock
sudo apt-get install -y supervisor
