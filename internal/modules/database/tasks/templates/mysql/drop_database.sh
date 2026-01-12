#!/bin/bash
{{ shellDefaults }}

sudo mysql --user="{{ .User }}" --password="{{ .Password }}" -e "DROP DATABASE IF EXISTS {{ .DatabaseName }};"
