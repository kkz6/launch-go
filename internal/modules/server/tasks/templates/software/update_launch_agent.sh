#!/bin/bash
{{ shellDefaults }}

echo "Updating Launch Agent"

# Re-run the installer. It self-detects a newer published release and
# swaps /usr/local/bin/launch-agent in place; if already current it
# exits 0 with "already updated". This does NOT touch the agent config
# or systemd unit — those were written at install time.
curl -sSL https://kkz6.github.io/launch-util/install | sh

# Restart so the running process picks up the new binary.
sudo systemctl restart launch-agent
sudo systemctl is-active launch-agent

echo "Launch Agent update complete: $(/usr/local/bin/launch-agent --version 2>/dev/null)"
