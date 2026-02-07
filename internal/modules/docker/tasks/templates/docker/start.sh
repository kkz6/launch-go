#!/bin/bash
{{ shellDefaults }}
echo "Starting container: {{ .ContainerName }}"
docker start {{ .ContainerName }}
echo "Container started"
