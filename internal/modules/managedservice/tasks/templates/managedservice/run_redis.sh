#!/bin/bash
{{ shellDefaults }}

echo "Run managed Redis ({{ .Image }})"

# Idempotent: pull, drop any prior container, recreate. The named volume
# {{ .Volume }} survives across re-runs so data is preserved.
sudo docker pull {{ .Image }}
sudo docker volume create {{ .Volume }} >/dev/null
sudo docker rm -f {{ .Container }} >/dev/null 2>&1 || true

sudo docker run -d \
    --name {{ .Container }} \
    --restart unless-stopped \
    --network launch-network \
    -v {{ .Volume }}:/data \
    {{ .Image }} \
    redis-server --requirepass {{ .Password }} --appendonly yes

echo "Redis started as {{ .Container }} on launch-network:6379"
