#!/bin/bash
{{ shellDefaults }}

echo "Deploy {{ .Container }} ({{ .ImageRef }})"

{{ if .RegistryURL }}
# Login to the private registry. STDIN avoids the password landing in
# the process table or shell history.
echo "{{ .RegistryPassword }}" | sudo docker login "{{ .RegistryURL }}" --username "{{ .RegistryUsername }}" --password-stdin
{{ end }}

# Pull the image we are about to run. `--quiet` keeps the output focused.
sudo docker pull "{{ .ImageRef }}"

# Ensure the named volumes exist before we run.
{{ range .Volumes }}
sudo docker volume create "{{ .HostName }}" >/dev/null
{{ end }}

# Drop any prior container with this name. Idempotent.
sudo docker rm -f "{{ .Container }}" >/dev/null 2>&1 || true

# Launch.
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
