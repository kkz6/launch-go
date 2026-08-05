#!/bin/bash
{{ shellDefaults }}

echo "Install essential packages"

# The provision script's preamble (quiesceAptForProvisioning) has already
# waited for cloud-init and stopped+masked the apt-daily/unattended-
# upgrades units, so the dpkg lock should be free. This is a safety net.
waitForAptUnlock

# Refresh package metadata before install. The historical "apt update +
# upgrade" provision step was removed (see ForFreshServer); this is the
# first update on a fresh box.
aptGet update -y -qq

waitForAptUnlock

aptGet install -y -qq \
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
