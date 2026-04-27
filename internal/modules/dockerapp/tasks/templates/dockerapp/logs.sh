#!/bin/bash
{{ shellDefaults }}

# Tail the application container logs. Captured by the task runner.
sudo docker logs --tail {{ .Tail }}{{ if .Timestamps }} --timestamps{{ end }} "{{ .Container }}" 2>&1
