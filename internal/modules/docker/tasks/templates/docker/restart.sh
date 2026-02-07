#!/bin/bash
{{ shellDefaults }}
echo "Restarting container: {{ .ContainerName }}"
docker restart {{ .ContainerName }}
echo "Container restarted"
