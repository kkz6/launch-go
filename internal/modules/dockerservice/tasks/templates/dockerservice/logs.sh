#!/bin/bash
{{ shellDefaults }}

# Tail the docker service container logs. The output is captured by the
# task runner and surfaced through the same task-log infrastructure as
# every other server task.
sudo docker logs --tail {{ .Tail }}{{ if .Timestamps }} --timestamps{{ end }} {{ .Container }} 2>&1
