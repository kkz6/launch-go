#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Node 21"

waitForAptUnlock
aptGet remove --purge -y nodejs
waitForAptUnlock
sudo curl --silent --location https://deb.nodesource.com/setup_21.x | sudo bash -
aptGet update
waitForAptUnlock

aptGet install -y --allow-downgrades --allow-remove-essential --allow-change-held-packages nodejs

echo "Install Node Packages"

sudo npm install -g fx n pm2 svgo yarn zx
