#!/bin/bash
{{ shellDefaults }}

echo "Deploy {{ .Container }} from git ({{ .RepoURL }}@{{ .Branch }})"

PROJECT_DIR="/opt/launch/apps/{{ .AppName }}"
SOURCE_DIR="${PROJECT_DIR}/source"

sudo mkdir -p "${PROJECT_DIR}"

# Build the clone URL with optional access token. The token is only
# expanded into the URL for the duration of git fetch/clone — it never
# touches disk because we pipe it through stdin.
{{ if .GitToken }}
CLONE_URL="$(printf '%s' '{{ .RepoURL }}' | sed 's#https://#https://x-access-token:{{ .GitToken }}@#')"
{{ else }}
CLONE_URL='{{ .RepoURL }}'
{{ end }}

if [ -d "${SOURCE_DIR}/.git" ]; then
  cd "${SOURCE_DIR}"
  sudo git remote set-url origin "${CLONE_URL}"
  sudo git fetch --depth 1 origin "{{ .Branch }}"
  sudo git checkout -B "{{ .Branch }}" "origin/{{ .Branch }}"
  sudo git reset --hard "origin/{{ .Branch }}"
else
  sudo rm -rf "${SOURCE_DIR}"
  sudo git clone --depth 1 --branch "{{ .Branch }}" "${CLONE_URL}" "${SOURCE_DIR}"
  cd "${SOURCE_DIR}"
fi

COMMIT_SHA="$(sudo git rev-parse HEAD)"
echo "LAUNCH_COMMIT_SHA=${COMMIT_SHA}"

# Strip the token back out of the remote so it isn't persisted on disk.
sudo git remote set-url origin '{{ .RepoURL }}'

# Build the image. Docker BuildKit improves caching across rebuilds.
DOCKER_BUILDKIT=1 sudo docker build \
    -t "{{ .ImageRef }}" \
    -f "{{ .Dockerfile }}" \
    "{{ .BuildContext }}"

# Ensure named volumes exist.
{{ range .Volumes }}
sudo docker volume create "{{ .HostName }}" >/dev/null
{{ end }}

# Replace any prior container.
sudo docker rm -f "{{ .Container }}" >/dev/null 2>&1 || true

sudo docker run -d \
    --name "{{ .Container }}" \
    --restart "{{ .RestartPolicy }}" \
    --network launch-network \
{{- range .EnvVars }}
    -e "{{ .Key }}={{ .Value }}" \
{{- end }}
{{- range .Ports }}
    -p "{{ .HostPort }}:{{ .ContainerPort }}/{{ .Protocol }}" \
{{- end }}
{{- range .Volumes }}
    -v "{{ .HostName }}:{{ .MountPath }}" \
{{- end }}
{{- range .Labels }}
    --label "{{ . }}" \
{{- end }}
    "{{ .ImageRef }}"

echo "Deploy of {{ .Container }} complete."
