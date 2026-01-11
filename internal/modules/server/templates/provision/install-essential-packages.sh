echo "Install essential packages"

sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
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
