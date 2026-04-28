#!/bin/bash
{{ shellDefaults }}

# Lifecycle action against a compose project.
# Action is one of: start | stop | restart.
echo "{{ .Action }} compose project {{ .Project }}"

PROJECT_DIR="/opt/launch/apps/{{ .AppName }}"

if [ ! -d "${PROJECT_DIR}" ]; then
  echo "Project directory ${PROJECT_DIR} not found — was the app deployed?" >&2
  exit 1
fi

cd "${PROJECT_DIR}"
sudo docker compose -p "{{ .Project }}" {{ .Action }}
