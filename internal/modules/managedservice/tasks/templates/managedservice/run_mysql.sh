#!/bin/bash
{{ shellDefaults }}

echo "Run managed MySQL ({{ .Image }})"

# Idempotent: pull, drop any prior container, recreate. The named volume
# {{ .Volume }} survives across re-runs so data is preserved.
sudo docker pull {{ .Image }}
sudo docker volume create {{ .Volume }} >/dev/null
sudo docker rm -f {{ .Container }} >/dev/null 2>&1 || true

sudo docker run -d \
    --name {{ .Container }} \
    --restart unless-stopped \
    --network launch-network \
    -e MYSQL_ROOT_PASSWORD={{ .Password }} \
    -e MYSQL_DATABASE={{ .DatabaseName }} \
    -e MYSQL_USER={{ .Username }} \
    -e MYSQL_PASSWORD={{ .Password }} \
    -v {{ .Volume }}:/var/lib/mysql \
    {{ .Image }}

echo "MySQL started as {{ .Container }} on launch-network:3306"
