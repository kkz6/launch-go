#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Docker Engine + Compose plugin"

# Reject snap-installed Docker — its containerd path differs and breaks
# the launcher user's docker group access.
if command -v snap >/dev/null 2>&1 && snap list 2>/dev/null | grep -q '^docker '; then
    echo "ERROR: Docker installed via snap is not supported. Remove it first: sudo snap remove docker" >&2
    exit 1
fi

waitForAptUnlock
sudo apt-get install -y ca-certificates curl gnupg

# Add Docker's official GPG key + apt repository.
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
    sudo gpg --dearmor --batch --yes -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

. /etc/os-release
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" | \
    sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

waitForAptUnlock
sudo apt-get update
waitForAptUnlock
sudo apt-get install -y \
    docker-ce \
    docker-ce-cli \
    containerd.io \
    docker-buildx-plugin \
    docker-compose-plugin

sudo systemctl enable --now docker

# Allow the launch user to run docker without sudo.
sudo usermod -aG docker {{ .Username }}

# Launch directory tree (used by Traefik + future app deploys).
sudo mkdir -p /etc/launch/traefik/dynamic
sudo mkdir -p /etc/launch/apps
sudo mkdir -p /etc/launch/compose
sudo mkdir -p /etc/launch/logs
sudo chown -R {{ .Username }}:{{ .Username }} /etc/launch
sudo chmod 750 /etc/launch

# Attachable bridge network shared by Traefik and user containers.
# Idempotent: only create if it does not already exist.
if ! sudo docker network inspect launch-network >/dev/null 2>&1; then
    sudo docker network create --driver bridge launch-network
fi

echo "Docker installation complete."
