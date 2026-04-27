#!/bin/bash
{{ shellDefaults }}

echo "Uninstall docker service {{ .Container }}"

# Stop and remove the container. Tolerate it being already gone.
sudo docker rm -f {{ .Container }} >/dev/null 2>&1 || true

{{ if .RemoveData }}
# Caller asked for a hard removal. Drop the volume too so all data is
# gone — only used when explicitly requested.
sudo docker volume rm {{ .Volume }} >/dev/null 2>&1 || true
echo "Removed container and volume {{ .Volume }}."
{{ else }}
echo "Removed container. Volume {{ .Volume }} kept for safety."
{{ end }}
