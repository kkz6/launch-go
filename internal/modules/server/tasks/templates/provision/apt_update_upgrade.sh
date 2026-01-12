#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Update package repositories and upgrade packages"

# This ensures that the cloud-init bundle is not overwritten by a different version when running apt-get upgrade.
waitForAptUnlock
sudo DEBIAN_FRONTEND=noninteractive apt-mark hold cloud-init

# Remove ALL MySQL repository files to prevent expired GPG key errors
# The MySQL installation script will properly configure the repository later
sudo rm -f /etc/apt/sources.list.d/*mysql*

# Update package repositories
waitForAptUnlock
sudo DEBIAN_FRONTEND=noninteractive apt-get update -y

# Install software-properties-common
waitForAptUnlock
sudo DEBIAN_FRONTEND=noninteractive apt-get install software-properties-common -y

# Add universe repository
waitForAptUnlock
sudo DEBIAN_FRONTEND=noninteractive add-apt-repository universe -y

# Update package repositories
waitForAptUnlock
sudo DEBIAN_FRONTEND=noninteractive apt-get update -y

# Upgrade packages
waitForAptUnlock
sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -y
