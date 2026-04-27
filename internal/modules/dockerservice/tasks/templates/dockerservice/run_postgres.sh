#!/bin/bash
{{ shellDefaults }}

echo "Run managed Postgres ({{ .Image }})"

# Idempotent: pull, drop any prior container, recreate. The named volume
# {{ .Volume }} survives across re-runs so data is preserved.
sudo docker pull {{ .Image }}
sudo docker volume create {{ .Volume }} >/dev/null
sudo docker rm -f {{ .Container }} >/dev/null 2>&1 || true

sudo docker run -d \
    --name {{ .Container }} \
    --restart unless-stopped \
    --network launch-network \
    -e POSTGRES_USER={{ .Username }} \
    -e POSTGRES_PASSWORD={{ .Password }} \
    -e POSTGRES_DB={{ .DatabaseName }} \
    -v {{ .Volume }}:/var/lib/postgresql/data \
    {{ .Image }}

echo "Postgres started as {{ .Container }} on launch-network:5432"
