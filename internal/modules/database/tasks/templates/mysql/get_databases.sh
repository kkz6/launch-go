#!/bin/bash
{{ shellDefaults }}

sudo mysql --user="{{ .User }}" --password="{{ .Password }}" -e "SHOW DATABASES;" -s -N
