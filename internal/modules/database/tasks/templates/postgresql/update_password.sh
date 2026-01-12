#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -c "ALTER USER {{ .Username }} WITH PASSWORD '{{ .NewPassword }}';"
