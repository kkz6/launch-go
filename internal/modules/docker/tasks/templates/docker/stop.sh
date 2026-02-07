#!/bin/bash
{{ shellDefaults }}
echo "Stopping container: {{ .ContainerName }}"
docker stop {{ .ContainerName }}
echo "Container stopped"
