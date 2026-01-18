#!/bin/bash
{{ shellDefaults }}

curl -sSL https://kkz6.github.io/launch-util/install | sh

sudo mkdir -p /etc/launch-agent

sudo bash -c "cat > {{ .AgentConfigPath }} <<'EOF'
pulse:
    enabled: true
    webhook:
        url: {{ .AgentURL }}
        method: POST
EOF"

sudo bash -c "cat > /etc/systemd/system/launch-agent.service <<'EOF'
[Unit]
Description=Launch Agent
After=network.target

[Service]
Type=oneshot
User={{ .RootUsername }}
ExecStart=/usr/local/bin/launch-agent start
Restart=on-failure
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target

EOF"

sudo systemctl daemon-reload
sudo systemctl enable launch-agent
sudo systemctl start launch-agent

echo "Launch Agent installed successfully"
