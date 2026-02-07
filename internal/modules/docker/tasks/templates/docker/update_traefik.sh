#!/bin/bash
{{ shellDefaults }}

echo "Updating Traefik config for: {{ .ContainerName }}"

mkdir -p /etc/launch/traefik/dynamic

cat > /etc/launch/traefik/dynamic/{{ .ContainerName }}.yml << 'TRAEFIKEOF'
{{ .YAMLContent }}
TRAEFIKEOF

echo "Traefik config updated for {{ .ContainerName }}"
