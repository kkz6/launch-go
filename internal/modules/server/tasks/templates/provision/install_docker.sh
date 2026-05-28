#!/bin/bash
{{ shellDefaults }}
{{ aptFunctions }}

echo "Installing Docker CE + Compose plugin"

if command -v docker >/dev/null 2>&1; then
    echo "Docker already installed: $(docker --version)"
else
    # Detect the actual distro and codename from the running box, not
    # from whatever the user picked in the dashboard dropdown. Docker
    # hosts separate apt repos at .../linux/ubuntu and .../linux/debian
    # with non-overlapping codenames (ubuntu's noble/jammy/focal vs
    # debian's bookworm/bullseye). Hardcoding /linux/ubuntu silently
    # works as long as the box happens to be Ubuntu, then fails with
    # exit 100 / "Unable to locate package docker-ce" the moment a
    # customer brings a Debian VPS — which is the common Contabo
    # default and the case that stranded this server.
    . /etc/os-release
    case "${ID}" in
        ubuntu) DOCKER_DISTRO=ubuntu ;;
        debian) DOCKER_DISTRO=debian ;;
        *)
            echo "ERROR: Docker provisioning only supports Ubuntu or Debian." >&2
            echo "  /etc/os-release reports ID=${ID:-<unset>}" >&2
            exit 1
            ;;
    esac

    # VERSION_CODENAME is the canonical field across both distros.
    # Bail explicitly if it's missing so we don't write a malformed
    # apt sources line like "deb [...] noble" with an empty codename.
    if [ -z "${VERSION_CODENAME}" ]; then
        echo "ERROR: /etc/os-release has no VERSION_CODENAME; cannot determine Docker apt suite." >&2
        exit 1
    fi

    waitForAptUnlock
    sudo apt-get update -qq

    waitForAptUnlock
    sudo apt-get install -y -qq ca-certificates curl gnupg

    sudo install -m 0755 -d /etc/apt/keyrings
    if [ ! -f /etc/apt/keyrings/docker.gpg ]; then
        curl -fsSL "https://download.docker.com/linux/${DOCKER_DISTRO}/gpg" \
            | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        sudo chmod a+r /etc/apt/keyrings/docker.gpg
    fi

    ARCH="$(dpkg --print-architecture)"
    echo "deb [arch=${ARCH} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/${DOCKER_DISTRO} ${VERSION_CODENAME} stable" \
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

# AWS CLI was previously installed here so the database-backup task
# could shell out to `aws s3 cp` for S3-backed storage providers.
# Removed because:
#
#   - The `awscli` apt package exists on Ubuntu but NOT in Debian's
#     official archives, so every Debian provision died with
#     "E: Package 'awscli' has no installation candidate". The
#     install isn't strictly required mid-provision anyway.
#   - Every server paid the cost — apt download + install time — even
#     when the user never configures an S3 backup destination.
#   - We ship our own agent (launch-agent, installed in the
#     subsequent step). Storage uploads should funnel through that
#     so credentials, retries, and provider abstraction live in one
#     place instead of being spread across an unrelated CLI tool.
#
# Follow-up: rewire internal/modules/docker/tasks/database_backup.go
# to call the agent's storage subcommand instead of bare `aws s3`.
# Until then, configured S3 backups will fail with the existing
# `command -v aws` guard's "aws CLI not installed" message, which is
# a deliberate signal rather than a silent regression.
