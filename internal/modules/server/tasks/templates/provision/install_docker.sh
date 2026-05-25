#!/bin/bash
{{ shellDefaults }}
{{ aptFunctions }}

echo "Installing Docker CE + Compose plugin"

if command -v docker >/dev/null 2>&1; then
    echo "Docker already installed: $(docker --version)"
else
    waitForAptUnlock
    sudo apt-get update -qq

    waitForAptUnlock
    sudo apt-get install -y -qq ca-certificates curl gnupg

    sudo install -m 0755 -d /etc/apt/keyrings
    if [ ! -f /etc/apt/keyrings/docker.gpg ]; then
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
            | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        sudo chmod a+r /etc/apt/keyrings/docker.gpg
    fi

    UBUNTU_CODENAME="$(. /etc/os-release && echo "${VERSION_CODENAME}")"
    ARCH="$(dpkg --print-architecture)"
    echo "deb [arch=${ARCH} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${UBUNTU_CODENAME} stable" \
        | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null

    waitForAptUnlock
    sudo apt-get update -qq

    waitForAptUnlock
    sudo apt-get install -y -qq \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin

    sudo systemctl enable --now docker
fi

# Allow the default user to run docker without sudo.
if id "{{ .Username }}" >/dev/null 2>&1; then
    sudo usermod -aG docker "{{ .Username }}"
fi

# Install the AWS CLI so the database-backup task can `aws s3 cp` dumps
# to the user's storage provider. Database backups run on this host; if
# awscli is missing every backup silently fails at "aws CLI not
# installed on this server" inside the backup script. Ubuntu's `awscli`
# package is AWS CLI v1, which is sufficient for our `s3 cp` / `s3 rm`
# usage. Idempotent — skipped when already installed.
echo "Installing AWS CLI"
if command -v aws >/dev/null 2>&1; then
    echo "AWS CLI already installed: $(aws --version 2>&1 | head -n1)"
else
    # If Docker was pre-installed (custom AMI, snap, etc.) the Docker
    # block above short-circuited and the apt cache may be stale, so
    # refresh before the install.
    waitForAptUnlock
    sudo apt-get update -qq
    waitForAptUnlock
    sudo apt-get install -y -qq awscli
fi
