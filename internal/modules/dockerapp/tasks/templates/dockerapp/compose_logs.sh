#!/bin/bash
{{ shellDefaults }}

PROJECT_DIR="/opt/launch/apps/{{ .AppName }}"

if [ ! -d "${PROJECT_DIR}" ]; then
  echo "Project directory ${PROJECT_DIR} not found — was the app deployed?" >&2
  exit 1
fi

cd "${PROJECT_DIR}"

sudo docker compose -p "{{ .Project }}" logs --tail "{{ .Tail }}"{{ if .Timestamps }} --timestamps{{ end }}
