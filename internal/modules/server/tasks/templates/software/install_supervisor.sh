#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Supervisor"

waitForAptUnlock
aptGet install -y supervisor
