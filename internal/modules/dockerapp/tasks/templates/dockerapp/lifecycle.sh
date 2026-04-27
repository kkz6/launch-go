#!/bin/bash
{{ shellDefaults }}

# Lifecycle action against an already-deployed application container.
# Action is one of: start | stop | restart.
echo "{{ .Action }} {{ .Container }}"

sudo docker {{ .Action }} "{{ .Container }}"
