#!/bin/bash
{{ shellDefaults }}

echo "Removing Traefik config for: {{ .ContainerName }}"
rm -f /etc/launch/traefik/dynamic/{{ .ContainerName }}.yml
echo "Traefik config removed"
