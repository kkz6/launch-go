#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Node 21"

waitForAptUnlock
sudo apt-get remove --purge -y nodejs
waitForAptUnlock
sudo curl --silent --location https://deb.nodesource.com/setup_21.x | sudo bash -
sudo apt-get update
waitForAptUnlock

sudo apt-get install -y --allow-downgrades --allow-remove-essential --allow-change-held-packages nodejs

echo "Install Node Packages"

sudo npm install -g fx n pm2 svgo yarn zx
