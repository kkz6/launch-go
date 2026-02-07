#!/bin/bash
{{ shellDefaults }}

echo "Deploying docker service: {{ .ContainerName }}"

{{ if .RegistryURL }}
echo "Logging into registry: {{ .RegistryURL }}"
echo "{{ .RegistryPassword }}" | docker login {{ .RegistryURL }} -u "{{ .RegistryUsername }}" --password-stdin
{{ end }}

echo "Pulling image: {{ .Image }}"
docker pull {{ .Image }}

echo "Stopping existing container"
docker stop {{ .ContainerName }} 2>/dev/null || true
docker rm {{ .ContainerName }} 2>/dev/null || true

echo "Starting container: {{ .ContainerName }}"
docker run -d \
  --name {{ .ContainerName }} \
  --network launch-network \
  --restart {{ .RestartPolicy }} \
  {{ range .EnvVars }}-e "{{ .Key }}={{ .Value }}" \
  {{ end }}{{ range .Volumes }}-v {{ .Source }}:{{ .Target }}{{ if .ReadOnly }}:ro{{ end }} \
  {{ end }}{{ range .Ports }}-p {{ .HostPort }}:{{ .ContainerPort }}/{{ .Protocol }} \
  {{ end }}{{ if .CPULimit }}--cpus={{ .CPULimit }} \
  {{ end }}{{ if .MemoryLimit }}--memory={{ .MemoryLimit }}m \
  {{ end }}{{ if .Command }}{{ .Image }} {{ .Command }}{{ else }}{{ .Image }}{{ end }}

echo "Verifying container is running"
sleep 2
if docker ps --format '{{ "{{" }}.Names{{ "}}" }}' | grep -q "^{{ .ContainerName }}$"; then
    echo "Container {{ .ContainerName }} is running successfully"
else
    echo "ERROR: Container {{ .ContainerName }} failed to start"
    docker logs {{ .ContainerName }} 2>&1 | tail -20
    exit 1
fi

echo "Deploy completed successfully"
