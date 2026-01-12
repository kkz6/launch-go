#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -c "CREATE USER {{ .Username }} WITH PASSWORD '{{ .Password }}';"
