#!/bin/bash
{{ shellDefaults }}

sudo mysql --user="{{ .User }}" --password="{{ .Password }}" -e "SELECT user, host FROM mysql.user;" -s -N 2>/dev/null
