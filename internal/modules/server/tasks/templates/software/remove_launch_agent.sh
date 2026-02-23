#!/bin/bash
{{ shellDefaults }}

echo "Remove Launch Agent"

sudo systemctl stop launch-agent 2>/dev/null || true
sudo systemctl disable launch-agent 2>/dev/null || true
sudo rm -f /etc/systemd/system/launch-agent.service
sudo rm -f /usr/local/bin/launch-agent
sudo rm -rf /etc/launch-agent
sudo rm -f ~/.launcher/launch.log
sudo rm -f ~/.launcher/launch.pid
sudo systemctl daemon-reload

echo "Launch Agent removed successfully."
