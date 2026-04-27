#!/bin/bash
{{ shellDefaults }}

echo "Uninstall {{ .Container }}"

# Stop and remove the container. Tolerate already-gone state.
sudo docker rm -f "{{ .Container }}" >/dev/null 2>&1 || true

{{ if .RemoveData }}
# Caller asked for a hard removal. Drop named volumes too.
{{ range .Volumes }}
sudo docker volume rm "{{ .HostName }}" >/dev/null 2>&1 || true
{{ end }}
echo "Removed container and {{ len .Volumes }} volume(s)."
{{ else }}
echo "Removed container. Volumes preserved."
{{ end }}
