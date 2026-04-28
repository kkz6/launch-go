#!/bin/bash
{{ shellDefaults }}

echo "Uninstall compose project {{ .Project }}"

PROJECT_DIR="/opt/launch/apps/{{ .AppName }}"

if [ -d "${PROJECT_DIR}" ]; then
  cd "${PROJECT_DIR}"
{{ if .RemoveData }}
  # Hard removal — also drop named volumes managed by compose.
  sudo docker compose -p "{{ .Project }}" down -v --remove-orphans || true
{{ else }}
  sudo docker compose -p "{{ .Project }}" down --remove-orphans || true
{{ end }}
  cd /
  sudo rm -rf "${PROJECT_DIR}"
fi

{{ if .RemoveData }}
echo "Removed compose project and volumes."
{{ else }}
echo "Removed compose project. Named volumes preserved."
{{ end }}
