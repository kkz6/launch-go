#!/bin/bash
{{ shellDefaults }}

echo "Install essential packages"

# The provision script's preamble (quiesceAptForProvisioning) has already
# waited for cloud-init and stopped+masked the apt-daily/unattended-
# upgrades units, so the dpkg lock should be free. We still call
# waitForAptUnlock here as a safety net AND pass APT_LOCK_WAIT_OPTS so
# apt itself will retry the lock for up to 120s if anything races us.
waitForAptUnlock

# Refresh package metadata before install. The historical "apt update +
# upgrade" provision step was removed (see ForFreshServer); this is the
# first apt-get update on a fresh box.
sudo apt-get ${APT_LOCK_WAIT_OPTS} update -y -qq

waitForAptUnlock

sudo apt-get ${APT_LOCK_WAIT_OPTS} install -y -qq \
    acl \
    apt-transport-https \
    ca-certificates \
    cron \
    curl \
    debian-archive-keyring \
    debian-keyring \
    fail2ban \
    gcc \
    git \
    lsb-release \
    make \
    procps \
    software-properties-common \
    ufw \
    unattended-upgrades \
    unzip \
    wget \
    zip
