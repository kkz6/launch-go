#!/bin/bash
{{ shellDefaults }}

sudo mysql --user="{{ .User }}" --password="{{ .Password }}" -e "SHOW TABLES FROM {{ .DatabaseName }};" -s -N 2>/dev/null
