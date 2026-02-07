#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Docker Engine"

# Remove old versions
waitForAptUnlock
sudo apt-get remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true

# Add Docker official GPG key
waitForAptUnlock
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Add repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker
waitForAptUnlock
sudo apt-get update
waitForAptUnlock
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin

# Add user to docker group
sudo usermod -aG docker {{ .Username }}

# Enable and start Docker
sudo systemctl enable docker
sudo systemctl start docker

# Create shared network for all containers
docker network create launch-network 2>/dev/null || true

echo "Docker Engine installed successfully"
